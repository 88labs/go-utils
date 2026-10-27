package cdc

import (
	"context"
	"encoding/json"
	"testing"
)

type order struct {
	ID          int32      `json:"id"`
	PendingCD   *string    `json:"pending_cd"`
	ClientID    *int32     `json:"client_id"`
	SiteName    *string    `json:"site_name"`
	ContactedAt *Date      `json:"contacted_date"`
	CreatedAt   Timestamp  `json:"created_at"`
	DeletedAt   *Timestamp `json:"deleted_at"`
}

func snsBody(t *testing.T, debezium string) string {
	b, _ := json.Marshal(map[string]any{
		"Type": "Notification", "MessageId": "sns-1",
		"TopicArn": "arn:aws:sns:ap-northeast-1:000000000000:cdc.andpad_development.orders",
		"Message":  debezium,
	})
	return string(b)
}

func TestDispatchOrderCreate(t *testing.T) {
	dbz := `{"schema":{"type":"struct"},"payload":{"before":null,"after":{"id":208,"pending_cd":"2026102101","client_id":123456,"site_name":"hurl","contacted_date":20747,"created_at":1792592281000,"deleted_at":null},"source":{"db":"andpad_development","table":"orders","file":"binlog.000011","pos":2718980},"op":"c","ts_ms":1792661153688}}`

	r := NewRouter()
	var got Event[order]
	handle(r, "orders", func(_ context.Context, e Event[order]) error { got = e; return nil })

	if err := r.dispatch(context.Background(), snsBody(t, dbz), Meta{ReceiveCount: 3}); err != nil {
		t.Fatal(err)
	}
	if got.Op != OpCreate || got.Before != nil || got.After == nil || got.After.ID != 208 {
		t.Fatalf("unexpected event: %+v", got)
	}
	if got.After.ContactedAt.Format("2006-01-02") != "2026-10-22" && got.After.ContactedAt.Year() != 2026 {
		t.Fatalf("date decode: %v", got.After.ContactedAt)
	}
	if got.After.DeletedAt != nil || got.Meta.ReceiveCount != 3 || got.ID != "sns-1" {
		t.Fatalf("unexpected: %+v", got)
	}
}

func TestRawDeliveryAndUnknownTable(t *testing.T) {
	r := NewRouter()
	called := false
	handle(r, "orders", func(context.Context, Event[order]) error { called = true; return nil })
	body := `{"before":null,"after":{"id":1},"source":{"db":"d","table":"clients"},"op":"c"}`
	if err := r.dispatch(context.Background(), body, Meta{}); err != nil || called {
		t.Fatalf("unknown table should be ignored: err=%v called=%v", err, called)
	}
}

func TestBadPayloadIsPermanent(t *testing.T) {
	r := NewRouter()
	handle(r, "orders", func(context.Context, Event[order]) error { return nil })
	body := `{"before":null,"after":{"id":"not-int"},"source":{"db":"d","table":"orders"},"op":"c"}`
	if err := r.dispatch(context.Background(), body, Meta{}); !IsPermanent(err) {
		t.Fatalf("want permanent, got %v", err)
	}
}
