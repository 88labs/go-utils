// Package tracers extracts trace information from a context.Context.
//
// Trace context is the portable metadata that identifies a position in a
// distributed trace as work crosses process boundaries. This package reads
// OpenTelemetry's vendor-neutral SpanContext first and uses Datadog APM v2 as
// a fallback when a Datadog span is present in the context.
//
// The package emits and parses the W3C Trace Context fields traceparent and
// tracestate. traceparent carries the trace ID, parent span ID, and trace
// flags in a fixed format. tracestate is optional and carries additional
// vendor-specific trace data.
//
// References:
//
//   - W3C Trace Context: https://www.w3.org/TR/trace-context/
//   - OpenTelemetry Tracing API: https://opentelemetry.io/docs/specs/otel/trace/api/
//   - Datadog Go tracer v2: https://pkg.go.dev/github.com/DataDog/dd-trace-go/v2/ddtrace/tracer
package tracers

import (
	"context"
	"encoding/binary"
	"strconv"

	ddtracer "github.com/DataDog/dd-trace-go/v2/ddtrace/tracer"
	"go.opentelemetry.io/otel/propagation"
	oteltrace "go.opentelemetry.io/otel/trace"
)

// TraceInfo contains W3C Trace Context fields.
//
// Use GetTraceParent and GetTraceState to retrieve the W3C propagation values.
// GetTraceID returns the retained 128-bit trace ID in the requested format.
// GetSpanID derives the span ID from the traceparent value.
type TraceInfo struct {
	traceParent string
	traceState  string
	traceID     string
}

// GetTraceParent returns the W3C Trace Context traceparent value.
func (info TraceInfo) GetTraceParent() string {
	return info.traceParent
}

// GetTraceState returns the optional W3C Trace Context tracestate value.
func (info TraceInfo) GetTraceState() string {
	return info.traceState
}

// IsValid reports whether the current traceparent value can be parsed as a
// valid W3C span context with non-zero trace and span IDs. It reflects the
// current fields, regardless of how the TraceInfo was created.
func (info TraceInfo) IsValid() bool {
	_, ok := info.spanContext()
	return ok
}

// GetTraceID returns the retained trace ID in the requested format.
func (info TraceInfo) GetTraceID(format TraceIDFormat) string {
	return formatTraceID(info.traceID, format)
}

// GetSpanID returns the span ID derived from the traceparent value as lowercase
// hexadecimal text.
func (info TraceInfo) GetSpanID() string {
	spanContext, ok := info.spanContext()
	if !ok {
		return ""
	}
	return spanContext.SpanID().String()
}

// GetSpanIDUInt64 returns the SpanID as a uint64 for Datadog log fields.
// It returns zero when the TraceInfo does not contain a valid span context.
func (info TraceInfo) GetSpanIDUInt64() uint64 {
	spanContext, ok := info.spanContext()
	if !ok {
		return 0
	}
	spanID := spanContext.SpanID()
	return binary.BigEndian.Uint64(spanID[:])
}

// ExtractTraceContext extracts trace information from ctx.
//
// OpenTelemetry is checked first because it is vendor-neutral. Datadog APM
// v2 is used as the fallback. The complete 128-bit trace ID is retained, and
// GetTraceID selects the output format. The returned bool reports whether a
// valid trace context was found. A nil context or a context without a valid
// span returns the zero value and false.
//
// OpenTelemetry's SpanContext validity requires both a non-zero trace ID and
// a non-zero span ID. The bool result exposes that presence check without
// requiring callers to infer it from TraceInfo fields.
func ExtractTraceContext(ctx context.Context) (TraceInfo, bool) {
	if ctx == nil {
		return TraceInfo{}, false
	}

	if info, ok := extractOpenTelemetry(ctx); ok {
		return info, true
	}
	if info, ok := extractDatadog(ctx); ok {
		return info, true
	}
	return TraceInfo{}, false
}

// NewTraceInfoFromTraceParent creates TraceInfo from W3C Trace Context
// traceparent and tracestate values.
//
// Invalid traceparent values return the zero value. Invalid tracestate values
// are ignored according to the W3C Trace Context processing model.
// The full 128-bit trace ID is retained and can be formatted with GetTraceID.
//
// This function is intended for values received through a propagation
// boundary. It validates the required non-zero trace ID and parent ID before
// copying the fields into TraceInfo. See the W3C Trace Context specification
// for the propagation model and field definitions:
// https://www.w3.org/TR/trace-context/.
func NewTraceInfoFromTraceParent(traceParent, traceState string) TraceInfo {
	info, ok := traceInfoFromPropagation(traceParent, traceState)
	if !ok {
		return TraceInfo{}
	}
	return info
}

func (info TraceInfo) spanContext() (oteltrace.SpanContext, bool) {
	return spanContextFromPropagation(info.traceParent, info.traceState)
}

func extractOpenTelemetry(ctx context.Context) (TraceInfo, bool) {
	spanContext := oteltrace.SpanContextFromContext(ctx)
	if !spanContext.IsValid() {
		return TraceInfo{}, false
	}

	return traceInfoFromSpanContext(spanContext), true
}

func extractDatadog(ctx context.Context) (TraceInfo, bool) {
	span, ok := ddtracer.SpanFromContext(ctx)
	if !ok || span == nil || span.Context() == nil {
		return TraceInfo{}, false
	}

	datadogContext := span.Context()
	traceIDBytes := datadogContext.TraceIDBytes()
	spanID := datadogContext.SpanID()
	if traceIDBytes == [16]byte{} || spanID == 0 {
		return TraceInfo{}, false
	}
	if info, ok := extractDatadogPropagation(datadogContext); ok {
		return info, true
	}

	spanIDBytes := [8]byte{}
	binary.BigEndian.PutUint64(spanIDBytes[:], spanID)
	traceFlags := oteltrace.FlagsSampled
	if priority, ok := datadogContext.SamplingPriority(); ok && priority < 0 {
		traceFlags = 0
	}
	otelSpanContext := oteltrace.NewSpanContext(oteltrace.SpanContextConfig{
		TraceID:    oteltrace.TraceID(traceIDBytes),
		SpanID:     oteltrace.SpanID(spanIDBytes),
		TraceFlags: traceFlags,
	})
	return traceInfoFromSpanContext(otelSpanContext), true
}

func extractDatadogPropagation(datadogContext *ddtracer.SpanContext) (TraceInfo, bool) {
	carrier := make(ddtracer.TextMapCarrier)
	if err := ddtracer.Inject(datadogContext, carrier); err != nil {
		return TraceInfo{}, false
	}
	return traceInfoFromPropagation(carrier["traceparent"], carrier["tracestate"])
}

func traceInfoFromPropagation(traceParent, traceState string) (TraceInfo, bool) {
	spanContext, ok := spanContextFromPropagation(traceParent, traceState)
	if !ok {
		return TraceInfo{}, false
	}
	return TraceInfo{
		traceParent: traceParent,
		traceState:  spanContext.TraceState().String(),
		traceID:     spanContext.TraceID().String(),
	}, true
}

func spanContextFromPropagation(traceParent, traceState string) (oteltrace.SpanContext, bool) {
	if traceParent == "" {
		return oteltrace.SpanContext{}, false
	}
	carrier := propagation.MapCarrier{"traceparent": traceParent}
	if traceState != "" {
		carrier["tracestate"] = traceState
	}
	ctx := propagation.TraceContext{}.Extract(context.Background(), carrier)
	spanContext := oteltrace.SpanContextFromContext(ctx)
	return spanContext, spanContext.IsValid()
}

func traceInfoFromSpanContext(spanContext oteltrace.SpanContext) TraceInfo {
	return TraceInfo{
		traceParent: traceParentFromSpanContext(spanContext),
		traceState:  spanContext.TraceState().String(),
		traceID:     spanContext.TraceID().String(),
	}
}

func formatTraceID(traceID string, format TraceIDFormat) string {
	if format != FormatInt64 {
		return traceID
	}
	if len(traceID) != 32 {
		return ""
	}
	traceIDLower, err := strconv.ParseUint(traceID[16:], 16, 64)
	if err != nil {
		return ""
	}
	return strconv.FormatUint(traceIDLower, 10)
}

func traceParentFromSpanContext(spanContext oteltrace.SpanContext) string {
	carrier := propagation.MapCarrier{}
	propagation.TraceContext{}.Inject(
		oteltrace.ContextWithSpanContext(context.Background(), spanContext),
		carrier,
	)
	return carrier.Get("traceparent")
}
