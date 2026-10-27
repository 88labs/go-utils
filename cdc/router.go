package cdc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync/atomic"
)

// Meta carries SQS delivery info.
type Meta struct {
	SQSMessageID string
	ReceiveCount int // ApproximateReceiveCount; >1 means a redelivery
}

// Event is a typed change event for one table row.
type Event[T any] struct {
	Envelope[T]        // Before, After, Source, Op, TsMs
	ID          string // SNS message id
	Meta        Meta
}

// Row returns After, or Before for deletes.
func (e Event[T]) Row() *T {
	if e.After != nil {
		return e.After
	}
	return e.Before
}

type HandlerFunc[T any] func(ctx context.Context, e Event[T]) error

// permanentError marks errors that retrying cannot fix. The consumer deletes
// such messages (after reporting) instead of letting them loop to the DLQ.
type permanentError struct{ err error }

func (p permanentError) Error() string { return p.err.Error() }
func (p permanentError) Unwrap() error { return p.err }

func Permanent(err error) error {
	if err == nil {
		return nil
	}
	return permanentError{err}
}
func IsPermanent(err error) bool {
	var p permanentError
	return errors.As(err, &p)
}

type rawHandler func(ctx context.Context, ev *RawEvent, meta Meta) error

// Router maps "table" or "db.table" to handlers.
//
// Register every handler from one goroutine before calling NewConsumer. The
// consumer works on a private copy, so the router needs no lock, and
// registering after NewConsumer panics instead of racing with message handling.
type Router struct {
	handlers  map[string]rawHandler
	onUnknown func(ctx context.Context, ev *RawEvent) error
	attached  atomic.Bool // set by NewConsumer; the router is then read-only
}

func NewRouter() *Router { return &Router{handlers: map[string]rawHandler{}} }

// mustBeOpen panics once a consumer has taken its copy of the router: a late
// registration would silently never be used.
func (r *Router) mustBeOpen() {
	if r.attached.Load() {
		panic("cdc: handlers must be registered before NewConsumer")
	}
}

// freeze marks the router read-only and returns the copy a consumer runs on.
func (r *Router) freeze() *Router {
	r.attached.Store(true)
	c := &Router{handlers: make(map[string]rawHandler, len(r.handlers)), onUnknown: r.onUnknown}
	for k, h := range r.handlers {
		c.handlers[k] = h
	}
	c.attached.Store(true)
	return c
}

// OnUnknown sets the behaviour for tables with no handler (default: ignore + ack).
func (r *Router) OnUnknown(fn func(ctx context.Context, ev *RawEvent) error) {
	r.mustBeOpen()
	r.onUnknown = fn
}

// handle registers a typed handler. key is "orders" or "andpad_development.orders".
// (A generic function because Go methods cannot have type parameters.)
func handle[T any](r *Router, key string, h HandlerFunc[T]) {
	r.mustBeOpen()
	if _, dup := r.handlers[key]; dup {
		panic("cdc: duplicate handler for " + key)
	}
	r.handlers[key] = func(ctx context.Context, raw *RawEvent, meta Meta) error {
		var env Envelope[T]
		if err := json.Unmarshal(raw.payload, &env); err != nil {
			return Permanent(fmt.Errorf("cdc: decode %s: %w", key, err))
		}
		return h(ctx, Event[T]{
			Envelope: env,
			ID:       raw.ID,
			Meta:     meta,
		})
	}
}

// HandleRaw registers an untyped handler. Use key "*" as a catch-all for every
// table without a more specific handler (lookup order: "db.table", "table", "*").
func HandleRaw(r *Router, key string, h func(ctx context.Context, ev *RawEvent, meta Meta) error) {
	r.mustBeOpen()
	if _, dup := r.handlers[key]; dup {
		panic("cdc: duplicate handler for " + key)
	}
	r.handlers[key] = h
}

func (r *Router) lookup(s Source) rawHandler {
	if h, ok := r.handlers[s.DB+"."+s.Table]; ok {
		return h
	}
	if h, ok := r.handlers[s.Table]; ok {
		return h
	}
	return r.handlers["*"]
}

// dispatch parses an SQS body and invokes the matching handler.
func (r *Router) dispatch(ctx context.Context, body string, meta Meta) error {
	raw, err := ParseMessage(body)
	if err != nil {
		return Permanent(err)
	}
	h := r.lookup(raw.Source)
	if h == nil {
		if r.onUnknown != nil {
			return r.onUnknown(ctx, raw)
		}
		return nil
	}
	return h(ctx, raw, meta)
}

// Tables lists registered keys (handy for logging at startup).
func (r *Router) Tables() []string {
	out := make([]string, 0, len(r.handlers))
	for k := range r.handlers {
		out = append(out, k)
	}
	return out
}
