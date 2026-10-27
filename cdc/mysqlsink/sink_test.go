package mysqlsink

import (
	"strings"
	"testing"
	"time"

	"github.com/88labs/go-utils/cdc"
)

func TestUpsertSQLVersionGuardLast(t *testing.T) {
	s := New(nil)
	cols := []column{{name: "id"}, {name: "updated_at"}, {name: "trigger"}}
	sql := s.upsertSQL("orders", cols, []string{"id"}, "updated_at")

	if !strings.Contains(sql, "INSERT INTO `orders` (`id`, `updated_at`, `trigger`) VALUES (?, ?, ?) AS new") {
		t.Fatalf("bad insert: %s", sql)
	}
	if strings.Index(sql, "`trigger` = IF") > strings.Index(sql, "`updated_at` = IF") {
		t.Fatalf("version column must be assigned last: %s", sql)
	}
	if strings.Contains(sql, "`id` = IF") {
		t.Fatalf("pk must not be updated: %s", sql)
	}
}

func TestPickSyncedColumns(t *testing.T) {
	src := []column{{name: "id"}, {name: "updated_at"}, {name: "billing_tel"}, {name: "only_on_source"}}
	target := map[string]bool{"id": true, "updated_at": true, "billing_tel": true}

	names := func(cols []column) string {
		var n []string
		for _, c := range cols {
			n = append(n, c.name)
		}
		return strings.Join(n, ",")
	}
	tests := []struct {
		name string
		cfg  TableConfig
		want string
	}{
		{name: "columns missing on target are skipped", cfg: TableConfig{}, want: "id,updated_at,billing_tel"},
		{name: "exclude drops a column", cfg: TableConfig{Exclude: []string{"billing_tel"}}, want: "id,updated_at"},
		{name: "include limits columns", cfg: TableConfig{Include: []string{"id", "updated_at"}}, want: "id,updated_at"},
		{name: "exclude wins over include", cfg: TableConfig{Include: []string{"id", "billing_tel"}, Exclude: []string{"billing_tel"}}, want: "id"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := names(pick(src, target, tt.cfg)); got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestVersionColumn(t *testing.T) {
	cols := []column{{name: "id"}, {name: "updated_at"}, {name: "modified_at"}}
	excluded := TableConfig{Exclude: []string{"updated_at"}}

	tests := []struct {
		name    string
		cfg     TableConfig
		cols    []column
		want    string
		wantErr bool
	}{
		{name: "defaults to updated_at", cfg: TableConfig{}, cols: cols, want: "updated_at"},
		{name: "explicit column wins", cfg: TableConfig{VersionColumn: "modified_at"}, cols: cols, want: "modified_at"},
		{name: "default missing from synced columns", cfg: TableConfig{}, cols: []column{{name: "id"}}, wantErr: true},
		{name: "explicit missing from synced columns", cfg: TableConfig{VersionColumn: "modified_at"}, cols: []column{{name: "id"}, {name: "updated_at"}}, wantErr: true},
		{
			name:    "default excluded from synced columns",
			cfg:     excluded,
			cols:    pick(cols, map[string]bool{"id": true, "updated_at": true}, excluded),
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := versionColumn(tt.cfg, tt.cols)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("want error, got %q", got)
				}
				if cdc.IsPermanent(err) {
					t.Fatalf("missing version column must be retryable (DLQ), got permanent: %v", err)
				}
				return
			}
			if err != nil || got != tt.want {
				t.Fatalf("got %q, %v; want %q", got, err, tt.want)
			}
		})
	}
}

func TestTableAndAllRegisterKeys(t *testing.T) {
	r := cdc.NewRouter()
	s := New(nil)
	cfg := TableConfig{Include: []string{"id", "updated_at"}}

	s.Table(r, "orders", cfg)
	s.All(r, cfg)

	got := map[string]bool{}
	for _, k := range r.Tables() {
		got[k] = true
	}
	if len(got) != 2 || !got["orders"] || !got["*"] {
		t.Fatalf("registered keys = %v, want orders and *", r.Tables())
	}
}

func TestColumnSelectionFailsClosed(t *testing.T) {
	tests := []struct {
		name    string
		cfg     TableConfig
		wantErr bool
	}{
		{name: "empty config selects nothing", cfg: TableConfig{}, wantErr: true},
		{name: "exclude alone is not an allowlist", cfg: TableConfig{Exclude: []string{"billing_tel"}}, wantErr: true},
		{name: "version column alone is not an allowlist", cfg: TableConfig{VersionColumn: "updated_at"}, wantErr: true},
		{name: "include is an allowlist", cfg: TableConfig{Include: []string{"id"}}},
		{name: "all columns is an explicit opt-in", cfg: TableConfig{AllColumns: true}},
		{name: "all columns with exclude", cfg: TableConfig{AllColumns: true, Exclude: []string{"billing_tel"}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := checkColumnSelection("orders", tt.cfg)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestTableAndAllPanicWithoutColumnSelection(t *testing.T) {
	register := map[string]func(*Syncer, *cdc.Router){
		"Table": func(s *Syncer, r *cdc.Router) { s.Table(r, "orders", TableConfig{VersionColumn: "updated_at"}) },
		"All":   func(s *Syncer, r *cdc.Router) { s.All(r, TableConfig{VersionColumn: "updated_at"}) },
	}
	for name, fn := range register {
		t.Run(name, func(t *testing.T) {
			r := cdc.NewRouter()
			defer func() {
				if recover() == nil {
					t.Fatal("want panic for a config with no column selection")
				}
				if got := r.Tables(); len(got) != 0 {
					t.Fatalf("nothing may be registered on failure, got %v", got)
				}
			}()
			fn(New(nil), r)
		})
	}
}

func TestAllWithExplicitOptInRegisters(t *testing.T) {
	r := cdc.NewRouter()
	New(nil).All(r, TableConfig{AllColumns: true, Exclude: []string{"billing_tel"}})
	if got := r.Tables(); len(got) != 1 || got[0] != "*" {
		t.Fatalf("registered keys = %v, want [*]", got)
	}
}

func TestLegacyValues(t *testing.T) {
	sql := New(nil, WithLegacyValues()).upsertSQL("t", []column{{name: "id"}, {name: "a"}}, []string{"id"}, "")
	if !strings.Contains(sql, "`a` = VALUES(`a`)") || strings.Contains(sql, " AS new") {
		t.Fatal(sql)
	}
}

func TestConvert(t *testing.T) {
	d, _ := convert(column{typ: "int32", logical: "io.debezium.time.Date"}, []byte("20747"))
	if d != "2026-10-21" {
		t.Fatalf("date: %v", d)
	}
	ts, _ := convert(column{typ: "int64", logical: "io.debezium.time.Timestamp"}, []byte("1792592281000"))
	if ts.(time.Time).Year() != 2026 {
		t.Fatalf("ts: %v", ts)
	}
	if v, _ := convert(column{typ: "string"}, []byte("null")); v != nil {
		t.Fatal("null must stay nil")
	}
}
