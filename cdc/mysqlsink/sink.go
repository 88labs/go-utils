// Package mysqlsink replays Debezium change events into a target MySQL
// database. It is schema-driven: column names and types come from the
// Debezium "schema" block, so no Go struct per table is needed.
//
// Requirements:
//   - Debezium value.converter.schemas.enable=true (your sample has it).
//   - Target tables already exist (DDL is not synced). Columns missing on the
//     target are skipped; columns missing on the source are left alone.
//   - DSN should use parseTime=true&loc=UTC so DATETIME values round-trip as
//     Debezium emitted them (wall clock read as UTC).
//   - decimal.handling.mode=string (or double) if you have DECIMAL columns.
//   - MySQL >= 8.0.19 (row alias syntax). Use WithLegacyValues() for MariaDB.
package mysqlsink

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/88labs/go-utils/cdc"
)

type TableConfig struct {
	Target        string   // target table; default = source table name
	PK            []string // default: PRIMARY KEY columns read from the target table
	Include       []string // only these columns (default: all)
	Exclude       []string // never sync these columns
	VersionColumn string   // e.g. "updated_at": ignore stale/out-of-order updates
	SkipDelete    bool     // ignore op=d instead of DELETE
}

type Option func(*Syncer)

// WithLegacyValues uses VALUES(col) instead of the "AS new" row alias.
func WithLegacyValues() Option { return func(s *Syncer) { s.legacy = true } }

// WithTargetInfoTTL controls how long target columns/PK are cached (default 5m).
func WithTargetInfoTTL(d time.Duration) Option { return func(s *Syncer) { s.ttl = d } }

type column struct{ name, typ, logical string }

type targetInfo struct {
	cols map[string]bool
	pk   []string
	at   time.Time
}

type Syncer struct {
	db     *sql.DB
	legacy bool
	ttl    time.Duration

	mu      sync.Mutex
	srcCols map[uint64][]column
	targets map[string]*targetInfo
}

func New(db *sql.DB, opts ...Option) *Syncer {
	s := &Syncer{db: db, ttl: 5 * time.Minute, srcCols: map[uint64][]column{}, targets: map[string]*targetInfo{}}
	for _, o := range opts {
		o(s)
	}
	return s
}

// Table registers sync for one source table (key: "orders" or "db.orders").
func (s *Syncer) Table(r *cdc.Router, key string, cfg TableConfig) {
	cdc.HandleRaw(r, key, func(ctx context.Context, ev *cdc.RawEvent, _ cdc.Meta) error {
		return s.apply(ctx, cfg, ev)
	})
}

// All registers a catch-all: every table without its own handler is synced
// into a same-named target table. Tables you register with Table() win.
func (s *Syncer) All(r *cdc.Router, cfg TableConfig) { s.Table(r, "*", cfg) }

func (s *Syncer) apply(ctx context.Context, cfg TableConfig, ev *cdc.RawEvent) error {
	p := ev.Envelope
	target := cfg.Target
	if target == "" {
		target = p.Source.Table
	}
	info, err := s.target(ctx, target)
	if err != nil {
		return err // transient: retry
	}
	pk := cfg.PK
	if len(pk) == 0 {
		pk = info.pk
	}
	if len(pk) == 0 {
		return cdc.Permanent(fmt.Errorf("mysqlsink: no primary key for %s", target))
	}
	src, err := s.sourceColumns(ev.Schema)
	if err != nil {
		return cdc.Permanent(err)
	}
	cols := pick(src, info.cols, cfg)

	switch p.Op {
	case cdc.OpCreate, cdc.OpUpdate, cdc.OpRead:
		if p.After == nil {
			return cdc.Permanent(fmt.Errorf("mysqlsink: op=%s without after image", p.Op))
		}
		if !hasAll(cols, pk) {
			return cdc.Permanent(fmt.Errorf("mysqlsink: pk %v not in synced columns of %s", pk, target))
		}
		args, err := decodeRow(cols, *p.After)
		if err != nil {
			return cdc.Permanent(err)
		}
		_, err = s.db.ExecContext(ctx, s.upsertSQL(target, cols, pk, cfg.VersionColumn), args...)
		return err

	case cdc.OpDelete:
		if cfg.SkipDelete {
			return nil
		}
		if p.Before == nil {
			return cdc.Permanent(errors.New("mysqlsink: op=d without before image"))
		}
		vals, err := decodeRow(cols, *p.Before)
		if err != nil {
			return cdc.Permanent(err)
		}
		byName := map[string]any{}
		for i, c := range cols {
			byName[c.name] = vals[i]
		}
		where, args := make([]string, 0, len(pk)), make([]any, 0, len(pk))
		for _, k := range pk {
			v, ok := byName[k]
			if !ok {
				return cdc.Permanent(fmt.Errorf("mysqlsink: pk %q missing in before image", k))
			}
			where = append(where, q(k)+" = ?")
			args = append(args, v)
		}
		_, err = s.db.ExecContext(ctx, "DELETE FROM "+q(target)+" WHERE "+strings.Join(where, " AND "), args...)
		return err
	}
	return nil // truncate etc.: ignored
}

// upsertSQL builds INSERT ... ON DUPLICATE KEY UPDATE. With a version column,
// every assignment is guarded so older events never overwrite newer rows; the
// version column is assigned LAST because MySQL evaluates assignments left to
// right and later ones see the already-updated values.
func (s *Syncer) upsertSQL(target string, cols []column, pk []string, version string) string {
	isPK := map[string]bool{}
	for _, k := range pk {
		isPK[k] = true
	}
	ref := func(c string) string {
		if s.legacy {
			return "VALUES(" + q(c) + ")"
		}
		return "new." + q(c)
	}
	old := func(c string) string { return q(target) + "." + q(c) }

	var names, marks, sets []string
	for _, c := range cols {
		names = append(names, q(c.name))
		marks = append(marks, "?")
	}
	assign := func(c string) string {
		if version == "" {
			return q(c) + " = " + ref(c)
		}
		return fmt.Sprintf("%s = IF(%s IS NULL OR %s IS NULL OR %s >= %s, %s, %s)",
			q(c), ref(version), old(version), ref(version), old(version), ref(c), old(c))
	}
	hasVersion := false
	for _, c := range cols {
		switch {
		case isPK[c.name]:
		case c.name == version:
			hasVersion = true
		default:
			sets = append(sets, assign(c.name))
		}
	}
	if hasVersion {
		sets = append(sets, assign(version))
	}
	if len(sets) == 0 { // pk-only table
		sets = append(sets, q(pk[0])+" = "+q(pk[0]))
	}

	var b strings.Builder
	b.WriteString("INSERT INTO " + q(target) + " (" + strings.Join(names, ", ") + ") VALUES (" + strings.Join(marks, ", ") + ")")
	if !s.legacy {
		b.WriteString(" AS new")
	}
	b.WriteString(" ON DUPLICATE KEY UPDATE " + strings.Join(sets, ", "))
	return b.String()
}

// ---- schema handling ------------------------------------------------------

type schemaField struct {
	Type   string        `json:"type"`
	Name   string        `json:"name"`
	Field  string        `json:"field"`
	Fields []schemaField `json:"fields"`
}

func (s *Syncer) sourceColumns(schema json.RawMessage) ([]column, error) {
	if len(schema) == 0 {
		return nil, errors.New("mysqlsink: message has no schema (set value.converter.schemas.enable=true)")
	}
	h := fnv.New64a()
	h.Write(schema)
	key := h.Sum64()

	s.mu.Lock()
	cols, ok := s.srcCols[key]
	s.mu.Unlock()
	if ok {
		return cols, nil
	}

	var root schemaField
	if err := json.Unmarshal(schema, &root); err != nil {
		return nil, fmt.Errorf("mysqlsink: decode schema: %w", err)
	}
	for _, want := range []string{"after", "before"} {
		for _, f := range root.Fields {
			if f.Field != want || len(f.Fields) == 0 {
				continue
			}
			for _, c := range f.Fields {
				cols = append(cols, column{name: c.Field, typ: c.Type, logical: c.Name})
			}
			s.mu.Lock()
			s.srcCols[key] = cols
			s.mu.Unlock()
			return cols, nil
		}
	}
	return nil, errors.New("mysqlsink: schema has no before/after fields")
}

func (s *Syncer) target(ctx context.Context, table string) (*targetInfo, error) {
	s.mu.Lock()
	t := s.targets[table]
	s.mu.Unlock()
	if t != nil && time.Since(t.at) < s.ttl {
		return t, nil
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT COLUMN_NAME, COLUMN_KEY FROM information_schema.COLUMNS
		 WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ? ORDER BY ORDINAL_POSITION`, table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	t = &targetInfo{cols: map[string]bool{}, at: time.Now()}
	for rows.Next() {
		var name, key string
		if err := rows.Scan(&name, &key); err != nil {
			return nil, err
		}
		t.cols[name] = true
		if key == "PRI" {
			t.pk = append(t.pk, name)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(t.cols) == 0 {
		return nil, fmt.Errorf("mysqlsink: target table %q not found", table) // transient: may be created later
	}
	s.mu.Lock()
	s.targets[table] = t
	s.mu.Unlock()
	return t, nil
}

func pick(src []column, target map[string]bool, cfg TableConfig) []column {
	inc, exc := toSet(cfg.Include), toSet(cfg.Exclude)
	out := make([]column, 0, len(src))
	for _, c := range src {
		if !target[c.name] || exc[c.name] || (len(inc) > 0 && !inc[c.name]) {
			continue
		}
		out = append(out, c)
	}
	return out
}

func toSet(a []string) map[string]bool {
	m := make(map[string]bool, len(a))
	for _, s := range a {
		m[s] = true
	}
	return m
}

func hasAll(cols []column, names []string) bool {
	have := map[string]bool{}
	for _, c := range cols {
		have[c.name] = true
	}
	for _, n := range names {
		if !have[n] {
			return false
		}
	}
	return true
}

// ---- value conversion -----------------------------------------------------

func decodeRow(cols []column, raw json.RawMessage) ([]any, error) {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, fmt.Errorf("mysqlsink: decode row: %w", err)
	}
	out := make([]any, len(cols))
	for i, c := range cols {
		v, err := convert(c, m[c.name])
		if err != nil {
			return nil, fmt.Errorf("mysqlsink: column %s: %w", c.name, err)
		}
		out[i] = v
	}
	return out, nil
}

func convert(c column, raw json.RawMessage) (any, error) {
	if isNull(raw) {
		return nil, nil
	}
	switch c.logical {
	case "io.debezium.time.Date":
		n, err := strconv.ParseInt(string(raw), 10, 64)
		if err != nil {
			return nil, err
		}
		return time.Unix(n*86400, 0).UTC().Format("2006-01-02"), nil
	case "io.debezium.time.Timestamp":
		n, err := strconv.ParseInt(string(raw), 10, 64)
		return time.UnixMilli(n).UTC(), err
	case "io.debezium.time.MicroTimestamp":
		n, err := strconv.ParseInt(string(raw), 10, 64)
		return time.UnixMicro(n).UTC(), err
	case "io.debezium.time.NanoTimestamp":
		n, err := strconv.ParseInt(string(raw), 10, 64)
		return time.Unix(0, n).UTC(), err
	case "io.debezium.time.ZonedTimestamp": // TIMESTAMP columns, ISO-8601 in UTC
		var str string
		if err := json.Unmarshal(raw, &str); err != nil {
			return nil, err
		}
		t, err := time.Parse(time.RFC3339Nano, str)
		return t.UTC(), err
	case "org.apache.kafka.connect.data.Decimal", "io.debezium.data.VariableScaleDecimal":
		return nil, errors.New("binary DECIMAL unsupported; set decimal.handling.mode=string")
	}
	switch c.typ {
	case "int8", "int16", "int32", "int64":
		return strconv.ParseInt(string(raw), 10, 64)
	case "float", "double":
		return strconv.ParseFloat(string(raw), 64)
	case "boolean":
		return strconv.ParseBool(string(raw))
	case "string":
		var str string
		err := json.Unmarshal(raw, &str)
		return str, err
	case "bytes":
		var str string
		if err := json.Unmarshal(raw, &str); err != nil {
			return nil, err
		}
		return base64.StdEncoding.DecodeString(str)
	}
	return nil, fmt.Errorf("unsupported type %q (%s)", c.typ, c.logical)
}

func isNull(b json.RawMessage) bool { return len(b) == 0 || string(b) == "null" }

func q(id string) string { return "`" + strings.ReplaceAll(id, "`", "``") + "`" }
