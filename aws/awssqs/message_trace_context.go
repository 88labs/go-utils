package awssqs

import (
	"context"

	"github.com/88labs/go-utils/tracers"
	"go.opentelemetry.io/otel/propagation"
	oteltrace "go.opentelemetry.io/otel/trace"
)

type messageTraceContextKey struct{}

type messageTraceContextValue struct {
	info tracers.TraceInfo
	ok   bool
}

// ExtractMessageTraceContext returns the trace context propagated by the
// sender of the current message.
//
// The value is available from the context passed to a ProcessMessage handler
// when the message contains a valid W3C traceparent. It is separate from the
// active process span: ProcessMessage keeps the worker context as the process
// span's parent and represents the sender context as a span link. Callers can
// use the returned TraceInfo for log correlation without making it the parent
// of a new span.
//
// It returns false when called outside a traced ProcessMessage handler or when
// the message did not contain a valid trace context.
func ExtractMessageTraceContext(ctx context.Context) (tracers.TraceInfo, bool) {
	if ctx == nil {
		return tracers.TraceInfo{}, false
	}

	value, ok := ctx.Value(messageTraceContextKey{}).(messageTraceContextValue)
	if !ok || !value.ok || !value.info.IsValid() {
		return tracers.TraceInfo{}, false
	}
	return value.info, true
}

func withMessageTraceContext(ctx context.Context, spanContext oteltrace.SpanContext) context.Context {
	value := messageTraceContextValue{}
	carrier := propagation.MapCarrier{}
	propagation.TraceContext{}.Inject(
		oteltrace.ContextWithSpanContext(context.Background(), spanContext),
		carrier,
	)
	info := tracers.NewTraceInfoFromTraceParent(
		carrier.Get(traceParentMessageAttribute),
		carrier.Get(traceStateMessageAttribute),
	)
	if info.IsValid() {
		value.info = info
		value.ok = true
	}
	return context.WithValue(ctx, messageTraceContextKey{}, value)
}

func withoutMessageTraceContext(ctx context.Context) context.Context {
	return context.WithValue(ctx, messageTraceContextKey{}, messageTraceContextValue{})
}
