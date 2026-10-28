package mysqlsink

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/88labs/go-utils/cdc"
)

func upsertEvent(after string) *cdc.RawEvent {
	raw := json.RawMessage(after)
	return &cdc.RawEvent{
		Schema: json.RawMessage(strings.Replace(orderSchema, `"before"`, `"after"`, 1)),
		Envelope: cdc.Envelope[json.RawMessage]{
			Op:     cdc.OpUpdate,
			After:  &raw,
			Source: cdc.Source{DB: "d", Table: "orders"},
		},
	}
}

func TestUpsertGuardRejectsNullIncomingVersionOverNewerRow(t *testing.T) {
	tests := []struct {
		name string
		s    *Syncer
		want string
	}{
		{
			name: "row alias",
			s:    New(nil),
			want: "`name` = IF(`orders`.`updated_at` IS NULL OR (new.`updated_at` IS NOT NULL AND new.`updated_at` >= `orders`.`updated_at`), new.`name`, `orders`.`name`)",
		},
		{
			name: "legacy values",
			s:    New(nil, WithLegacyValues()),
			want: "`name` = IF(`orders`.`updated_at` IS NULL OR (VALUES(`updated_at`) IS NOT NULL AND VALUES(`updated_at`) >= `orders`.`updated_at`), VALUES(`name`), `orders`.`name`)",
		},
	}
	cols := []column{{name: "id"}, {name: "name"}, {name: "updated_at"}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sql := tt.s.upsertSQL("orders", cols, []string{"id"}, "updated_at")
			if !strings.Contains(sql, tt.want) {
				t.Fatalf("missing guard\n%s\nin\n%s", tt.want, sql)
			}
			// The old NULL branch must not apply to the incoming value alone.
			if strings.Contains(sql, "updated_at` IS NULL OR `orders`") || strings.Contains(sql, "IF(new.`updated_at` IS NULL") {
				t.Fatalf("incoming NULL version must not pass the guard: %s", sql)
			}
		})
	}
}

// TestIntegrationNullVersionEventDoesNotClearWatermark replays the reviewed
// sequence against a real MySQL: a newer row, then a delayed event with a NULL
// version, then a stale event. Neither may change the row. It needs
// orders(id PRIMARY KEY, name, updated_at DATETIME(3) NULL) and skips unless
// CDC_IT_TARGET_DSN is set (parseTime=true&loc=UTC, see the package doc).
func TestIntegrationNullVersionEventDoesNotClearWatermark(t *testing.T) {
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
	const id = 9_000_002
	cleanup := func() { _, _ = db.ExecContext(ctx, "DELETE FROM orders WHERE id = ?", id) }
	cleanup()
	t.Cleanup(cleanup)

	s := New(db)
	cfg := TableConfig{AllColumns: true}
	apply := func(after string) {
		t.Helper()
		if err := s.apply(ctx, cfg, upsertEvent(after)); err != nil {
			t.Fatalf("apply %s: %v", after, err)
		}
	}
	state := func() (name string, version sql.NullTime) {
		t.Helper()
		if err := db.QueryRowContext(ctx, "SELECT name, updated_at FROM orders WHERE id = ?", id).Scan(&name, &version); err != nil {
			t.Fatalf("read row: %v", err)
		}
		return name, version
	}

	apply(`{"id":9000002,"name":"newer","updated_at":2000}`)
	apply(`{"id":9000002,"name":"null-version","updated_at":null}`)
	apply(`{"id":9000002,"name":"stale","updated_at":1000}`)

	name, version := state()
	if name != "newer" || !version.Valid {
		t.Fatalf("row = (%q, %v), want (\"newer\", non-null version)", name, version)
	}
}

// TestUpsertIssuesOneVersionGuardedStatement pins that an update becomes a
// single INSERT ... ON DUPLICATE KEY UPDATE whose non-key assignments are all
// guarded by the version column and bound once per column.
func TestUpsertIssuesOneVersionGuardedStatement(t *testing.T) {
	db, rec := newRecordingDB(t, "id", "name", "updated_at")
	err := New(db).apply(context.Background(), TableConfig{AllColumns: true}, upsertEvent(`{"id":7,"name":"a","updated_at":1000}`))
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if len(rec.exec) != 1 {
		t.Fatalf("exec calls = %d, want 1", len(rec.exec))
	}
	got := rec.exec[0]
	if !strings.HasPrefix(got.query, "INSERT INTO `orders` (`id`, `name`, `updated_at`) VALUES (?, ?, ?) AS new ON DUPLICATE KEY UPDATE ") {
		t.Fatalf("unexpected statement: %s", got.query)
	}
	for _, c := range []string{"`name` = IF(", "`updated_at` = IF("} {
		if !strings.Contains(got.query, c) {
			t.Fatalf("missing guarded assignment %q: %s", c, got.query)
		}
	}
	if len(got.args) != 3 || got.args[0].Value != int64(7) {
		t.Fatalf("args = %v, want one per column starting with the pk", got.args)
	}
}
