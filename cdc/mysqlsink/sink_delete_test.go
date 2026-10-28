package mysqlsink

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"sync/atomic"
	"testing"

	_ "github.com/go-sql-driver/mysql"

	"github.com/88labs/go-utils/cdc"
)

// recordingDriver is a minimal database/sql driver standing in for MySQL, the
// external I/O seam. It answers the information_schema lookup with a fixed
// target table and records every other statement.
type recordingDriver struct {
	cols []string // target columns; the first is the primary key
	exec []execCall
}

type execCall struct {
	query string
	args  []driver.NamedValue
}

var driverSeq atomic.Int64

func newRecordingDB(t *testing.T, cols ...string) (*sql.DB, *recordingDriver) {
	t.Helper()
	d := &recordingDriver{cols: cols}
	name := "recording-" + string(rune('a'+driverSeq.Add(1)))
	sql.Register(name, d)
	db, err := sql.Open(name, "")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db, d
}

func (d *recordingDriver) Open(string) (driver.Conn, error) { return &recordingConn{d}, nil }

type recordingConn struct{ d *recordingDriver }

func (c *recordingConn) Prepare(string) (driver.Stmt, error) { return nil, driver.ErrSkip }
func (c *recordingConn) Close() error                        { return nil }
func (c *recordingConn) Begin() (driver.Tx, error)           { return nil, driver.ErrSkip }

func (c *recordingConn) ExecContext(_ context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	c.d.exec = append(c.d.exec, execCall{query, args})
	return driver.RowsAffected(1), nil
}

func (c *recordingConn) QueryContext(_ context.Context, _ string, _ []driver.NamedValue) (driver.Rows, error) {
	return &columnRows{cols: c.d.cols}, nil
}

type columnRows struct {
	cols []string
	i    int
}

func (r *columnRows) Columns() []string { return []string{"COLUMN_NAME", "COLUMN_KEY"} }
func (r *columnRows) Close() error      { return nil }
func (r *columnRows) Next(dest []driver.Value) error {
	if r.i >= len(r.cols) {
		return io.EOF
	}
	dest[0] = r.cols[r.i]
	dest[1] = ""
	if r.i == 0 {
		dest[1] = "PRI"
	}
	r.i++
	return nil
}

const orderSchema = `{"fields":[{"field":"before","fields":[
	{"field":"id","type":"int64"},
	{"field":"name","type":"string"},
	{"field":"updated_at","type":"int64","name":"io.debezium.time.Timestamp"}]}]}`

func deleteEvent(before string) *cdc.RawEvent {
	raw := json.RawMessage(before)
	return &cdc.RawEvent{
		Schema: json.RawMessage(orderSchema),
		Envelope: cdc.Envelope[json.RawMessage]{
			Op:     cdc.OpDelete,
			Before: &raw,
			Source: cdc.Source{DB: "d", Table: "orders"},
		},
	}
}

func TestSkipDeleteNeedsNoVersionColumn(t *testing.T) {
	db, rec := newRecordingDB(t, "id", "name")
	cfg := TableConfig{AllColumns: true, SkipDelete: true}
	if err := New(db).apply(context.Background(), cfg, deleteEvent(`{"id":7,"name":"a"}`)); err != nil {
		t.Fatalf("SkipDelete must ack: %v", err)
	}
	if len(rec.exec) != 0 {
		t.Fatalf("SkipDelete must not delete, got %v", rec.exec)
	}
}

func TestDeleteIsGuardedByBeforeImageVersion(t *testing.T) {
	const guarded = "DELETE FROM `orders` WHERE `id` = ? AND (`%s` IS NULL OR ? IS NULL OR `%[1]s` <= ?)"
	tests := []struct {
		name    string
		cfg     TableConfig
		before  string
		wantSQL string
	}{
		{
			name:    "default version column",
			cfg:     TableConfig{AllColumns: true},
			before:  `{"id":7,"name":"a","updated_at":1000}`,
			wantSQL: fmt.Sprintf(guarded, "updated_at"),
		},
		{
			name:    "custom version column",
			cfg:     TableConfig{AllColumns: true, VersionColumn: "name"},
			before:  `{"id":7,"name":"a","updated_at":1000}`,
			wantSQL: fmt.Sprintf(guarded, "name"),
		},
		{
			name:    "null before version still issues the guarded delete",
			cfg:     TableConfig{AllColumns: true},
			before:  `{"id":7,"name":"a","updated_at":null}`,
			wantSQL: fmt.Sprintf(guarded, "updated_at"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, rec := newRecordingDB(t, "id", "name", "updated_at")
			if err := New(db).apply(context.Background(), tt.cfg, deleteEvent(tt.before)); err != nil {
				t.Fatalf("apply: %v", err)
			}
			if len(rec.exec) != 1 {
				t.Fatalf("exec calls = %d, want 1", len(rec.exec))
			}
			got := rec.exec[0]
			if got.query != tt.wantSQL {
				t.Fatalf("query =\n%s\nwant\n%s", got.query, tt.wantSQL)
			}
			// Arguments: the primary key, then the before-image version twice.
			if len(got.args) != 3 {
				t.Fatalf("args = %v, want 3", got.args)
			}
			if got.args[0].Value != int64(7) {
				t.Fatalf("pk arg = %v, want 7", got.args[0].Value)
			}
			if got.args[1].Value != got.args[2].Value {
				t.Fatalf("version args differ: %v vs %v", got.args[1].Value, got.args[2].Value)
			}
		})
	}
}

func TestDeleteWithoutVersionColumnFailsRetryably(t *testing.T) {
	db, rec := newRecordingDB(t, "id", "name") // target has no updated_at
	err := New(db).apply(context.Background(), TableConfig{AllColumns: true}, deleteEvent(`{"id":7,"name":"a"}`))
	if err == nil {
		t.Fatal("delete without a version column must not be applied unguarded")
	}
	if cdc.IsPermanent(err) {
		t.Fatalf("must be retryable (retry, then DLQ) like the upsert path, got permanent: %v", err)
	}
	if !strings.Contains(err.Error(), "updated_at") {
		t.Fatalf("error should name the version column: %v", err)
	}
	if len(rec.exec) != 0 {
		t.Fatalf("no statement may run, got %v", rec.exec)
	}
}

// TestIntegrationDelayedDeleteDoesNotRemoveRecreatedRow replays the reviewed
// race against a real MySQL: the delete of version 1 arrives after the row was
// recreated at version 2. It needs a target database with
// orders(id PRIMARY KEY, name, updated_at DATETIME(3)); it skips unless
// CDC_IT_TARGET_DSN is set (parseTime=true&loc=UTC, see the package doc).
func TestIntegrationDelayedDeleteDoesNotRemoveRecreatedRow(t *testing.T) {
	dsn := os.Getenv("CDC_IT_TARGET_DSN")
	if dsn == "" {
		t.Skip("set CDC_IT_TARGET_DSN to run this test")
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	ctx := context.Background()
	const id = 9_000_001
	cleanup := func() { _, _ = db.ExecContext(ctx, "DELETE FROM orders WHERE id = ?", id) }
	cleanup()
	t.Cleanup(cleanup)

	const schema = `{"fields":[{"field":"after","fields":[
		{"field":"id","type":"int64"},{"field":"name","type":"string"},
		{"field":"updated_at","type":"int64","name":"io.debezium.time.Timestamp"}]},
		{"field":"before","fields":[
		{"field":"id","type":"int64"},{"field":"name","type":"string"},
		{"field":"updated_at","type":"int64","name":"io.debezium.time.Timestamp"}]}]}`
	event := func(op cdc.Op, before, after string) *cdc.RawEvent {
		ev := &cdc.RawEvent{Schema: json.RawMessage(schema)}
		ev.Op, ev.Source = op, cdc.Source{DB: "d", Table: "orders"}
		if before != "" {
			b := json.RawMessage(before)
			ev.Before = &b
		}
		if after != "" {
			a := json.RawMessage(after)
			ev.After = &a
		}
		return ev
	}
	s := New(db)
	cfg := TableConfig{AllColumns: true}
	count := func() (n int) {
		if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM orders WHERE id = ?", id).Scan(&n); err != nil {
			t.Fatalf("count: %v", err)
		}
		return n
	}
	apply := func(ev *cdc.RawEvent) {
		t.Helper()
		if err := s.apply(ctx, cfg, ev); err != nil {
			t.Fatalf("apply %s: %v", ev.Op, err)
		}
	}

	apply(event(cdc.OpCreate, "", `{"id":9000001,"name":"v1","updated_at":1000}`))
	apply(event(cdc.OpDelete, `{"id":9000001,"name":"v1","updated_at":1000}`, ""))
	apply(event(cdc.OpCreate, "", `{"id":9000001,"name":"v2","updated_at":2000}`))

	// The first delete is redelivered late, after the recreate.
	apply(event(cdc.OpDelete, `{"id":9000001,"name":"v1","updated_at":1000}`, ""))
	if count() != 1 {
		t.Fatal("a delayed delete of version 1 removed the version 2 row")
	}

	apply(event(cdc.OpDelete, `{"id":9000001,"name":"v2","updated_at":2000}`, ""))
	if count() != 0 {
		t.Fatal("the delete of the current version must remove the row")
	}
}
