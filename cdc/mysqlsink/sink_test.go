package mysqlsink

import (
	"strings"
	"testing"
	"time"
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
