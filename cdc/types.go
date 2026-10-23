package cdc

import (
	"encoding/json"
	"time"
)

// Date decodes io.debezium.time.Date (int32 days since 1970-01-01).
// Use *Date for nullable columns.
type Date struct{ time.Time }

func (d *Date) UnmarshalJSON(b []byte) error {
	var n int64
	if err := json.Unmarshal(b, &n); err != nil {
		return err
	}
	d.Time = time.Unix(n*86400, 0).UTC()
	return nil
}

// Timestamp decodes io.debezium.time.Timestamp (int64 millis since epoch).
// MySQL DATETIME has no zone: the value is the wall clock read as UTC, so
// reinterpret it with time.Date(..., loc) if your DB stores local time.
// Use *Timestamp for nullable columns.
type Timestamp struct{ time.Time }

func (t *Timestamp) UnmarshalJSON(b []byte) error {
	var n int64
	if err := json.Unmarshal(b, &n); err != nil {
		return err
	}
	t.Time = time.UnixMilli(n).UTC()
	return nil
}
