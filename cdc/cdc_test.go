package cdc

import (
	"context"
	"encoding/json"
	"sync"
	"sync/atomic"
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
	Handle(r, "orders", func(_ context.Context, e Event[order]) error { got = e; return nil })

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
	Handle(r, "orders", func(context.Context, Event[order]) error { called = true; return nil })
	body := `{"before":null,"after":{"id":1},"source":{"db":"d","table":"clients"},"op":"c"}`
	if err := r.dispatch(context.Background(), body, Meta{}); err != nil || called {
		t.Fatalf("unknown table should be ignored: err=%v called=%v", err, called)
	}
}

func TestLookupOrder(t *testing.T) {
	r := NewRouter()
	var got string
	record := func(name string) func(context.Context, *RawEvent, Meta) error {
		return func(context.Context, *RawEvent, Meta) error { got = name; return nil }
	}
	HandleRaw(r, "d.orders", record("db.table"))
	HandleRaw(r, "orders", record("table"))
	HandleRaw(r, "*", record("catch-all"))

	tests := []struct{ db, table, want string }{
		{db: "d", table: "orders", want: "db.table"},
		{db: "other", table: "orders", want: "table"},
		{db: "d", table: "clients", want: "catch-all"},
	}
	for _, tt := range tests {
		got = ""
		body := `{"before":null,"after":{"id":1},"source":{"db":"` + tt.db + `","table":"` + tt.table + `"},"op":"c"}`
		if err := r.dispatch(context.Background(), body, Meta{}); err != nil {
			t.Fatal(err)
		}
		if got != tt.want {
			t.Fatalf("%s.%s routed to %q, want %q", tt.db, tt.table, got, tt.want)
		}
	}
}

func noopHandler(context.Context, *RawEvent, Meta) error { return nil }

func TestRegisterAfterNewConsumerPanics(t *testing.T) {
	register := map[string]func(*Router){
		"HandleRaw": func(r *Router) { HandleRaw(r, "orders", noopHandler) },
		"handle":    func(r *Router) { Handle(r, "orders", func(context.Context, Event[order]) error { return nil }) },
		"OnUnknown": func(r *Router) { r.OnUnknown(func(context.Context, *RawEvent) error { return nil }) },
	}
	for name, fn := range register {
		t.Run(name, func(t *testing.T) {
			r := NewRouter()
			NewConsumer(nil, r, Config{QueueURL: "q"})
			defer func() {
				if recover() == nil {
					t.Fatal("registering after NewConsumer must panic")
				}
			}()
			fn(r)
		})
	}
}

func TestConsumerRunsOnACopyOfTheRouter(t *testing.T) {
	r := NewRouter()
	var got []string
	HandleRaw(r, "orders", func(context.Context, *RawEvent, Meta) error { got = append(got, "orders"); return nil })
	c := NewConsumer(nil, r, Config{QueueURL: "q"})

	body := `{"before":null,"after":{"id":1},"source":{"db":"d","table":"orders"},"op":"c"}`
	if err := c.router.dispatch(context.Background(), body, Meta{}); err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("handler registered before NewConsumer must receive events, got %v", got)
	}
	if c.router == r {
		t.Fatal("consumer must not share the live router")
	}
	if tables := c.router.Tables(); len(tables) != 1 || tables[0] != "orders" {
		t.Fatalf("copy tables = %v", tables)
	}
}

func TestDispatchIsSafeForConcurrentUse(t *testing.T) {
	r := NewRouter()
	var calls atomic.Int64
	HandleRaw(r, "orders", func(context.Context, *RawEvent, Meta) error { calls.Add(1); return nil })
	HandleRaw(r, "*", noopHandler)
	c := NewConsumer(nil, r, Config{QueueURL: "q"})

	const workers, perWorker = 8, 50
	body := `{"before":null,"after":{"id":1},"source":{"db":"d","table":"orders"},"op":"c"}`
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < perWorker; j++ {
				if err := c.router.dispatch(context.Background(), body, Meta{}); err != nil {
					t.Error(err)
					return
				}
			}
		}()
	}
	wg.Wait()
	if got := calls.Load(); got != workers*perWorker {
		t.Fatalf("handled %d messages, want %d", got, workers*perWorker)
	}
}

func TestBadPayloadIsPermanent(t *testing.T) {
	r := NewRouter()
	Handle(r, "orders", func(context.Context, Event[order]) error { return nil })
	body := `{"before":null,"after":{"id":"not-int"},"source":{"db":"d","table":"orders"},"op":"c"}`
	if err := r.dispatch(context.Background(), body, Meta{}); !IsPermanent(err) {
		t.Fatalf("want permanent, got %v", err)
	}
}
