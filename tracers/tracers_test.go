package tracers_test

import (
	"context"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"strconv"
	"testing"

	"github.com/DataDog/dd-trace-go/v2/ddtrace/ext"
	ddmocktracer "github.com/DataDog/dd-trace-go/v2/ddtrace/mocktracer"
	ddtracer "github.com/DataDog/dd-trace-go/v2/ddtrace/tracer"
	oteltrace "go.opentelemetry.io/otel/trace"

	"github.com/88labs/go-utils/tracers"
)

func TestExtractTraceContext(t *testing.T) {
	testCases := []struct {
		name  string
		setup func(*testing.T) (context.Context, string, string, string, string, string)
		found bool
	}{
		{
			name:  "OpenTelemetry",
			found: true,
			setup: func(t *testing.T) (context.Context, string, string, string, string, string) {
				ctx, spanContext := openTelemetryContext(t)
				traceParent, traceState := openTelemetryTraceInfo(spanContext)
				return ctx, traceParent, traceState,
					openTelemetryTraceID(spanContext, tracers.FormatString),
					openTelemetryTraceID(spanContext, tracers.FormatInt64),
					spanContext.SpanID().String()
			},
		},
		{
			name:  "DatadogV2",
			found: true,
			setup: func(t *testing.T) (context.Context, string, string, string, string, string) {
				ctx, span := datadogContext(t)
				traceParent, traceState := datadogTraceInfo(span)
				return ctx, traceParent, traceState,
					datadogTraceID(span, tracers.FormatString),
					datadogTraceID(span, tracers.FormatInt64),
					fmt.Sprintf("%016x", span.Context().SpanID())
			},
		},
		{
			name:  "OpenTelemetryTakesPrecedence",
			found: true,
			setup: func(t *testing.T) (context.Context, string, string, string, string, string) {
				ddContext, _ := datadogContext(t)
				ctx, spanContext := openTelemetryContextFrom(t, ddContext)
				traceParent, traceState := openTelemetryTraceInfo(spanContext)
				return ctx, traceParent, traceState,
					openTelemetryTraceID(spanContext, tracers.FormatString),
					openTelemetryTraceID(spanContext, tracers.FormatInt64),
					spanContext.SpanID().String()
			},
		},
		{
			name:  "Empty",
			found: false,
			setup: func(*testing.T) (context.Context, string, string, string, string, string) {
				return context.Background(), "", "", "", "", ""
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			ctx, wantTraceParent, wantTraceState, wantTraceIDString, wantTraceIDInt64, wantSpanID := testCase.setup(t)
			got, found := tracers.ExtractTraceContext(ctx)
			assertTraceInfo(t, got, wantTraceParent, wantTraceState, wantTraceIDString, wantTraceIDInt64, wantSpanID)
			if found != testCase.found {
				t.Fatalf("expected found=%t, got %t", testCase.found, found)
			}
		})
	}
}

func TestExtractTraceContext_nilContext(t *testing.T) {
	got, found := tracers.ExtractTraceContext(nil)
	if got != (tracers.TraceInfo{}) {
		t.Fatalf("expected the zero value, got %#v", got)
	}
	if found {
		t.Fatal("expected found=false")
	}
}

func TestTraceInfo_GetTraceID_unknownFormatUsesString(t *testing.T) {
	ctx, _ := datadogContext(t)
	got, found := tracers.ExtractTraceContext(ctx)
	if !found {
		t.Fatal("expected found=true")
	}
	if got.GetTraceID(tracers.TraceIDFormat(255)) != got.GetTraceID(tracers.FormatString) {
		t.Fatalf("unknown format should return the string trace ID")
	}
}

func TestExtractTraceContext_datadogPreservesW3CPropagation(t *testing.T) {
	ctx, span := datadogContext(t)

	wantTraceParent, wantTraceState := datadogW3CPropagation(t, span)
	got, found := tracers.ExtractTraceContext(ctx)

	if !found {
		t.Fatal("expected found=true")
	}
	if got.GetTraceParent() != wantTraceParent {
		t.Fatalf("GetTraceParent() = %q, want %q", got.GetTraceParent(), wantTraceParent)
	}
	if got.GetTraceState() != wantTraceState {
		t.Fatalf("GetTraceState() = %q, want %q", got.GetTraceState(), wantTraceState)
	}
}

func TestExtractTraceContext_datadogPreservesSamplingDecision(t *testing.T) {
	ctx, span := datadogContext(t)
	span.SetTag(ext.ManualDrop, true)

	wantTraceParent, wantTraceState := datadogW3CPropagation(t, span)
	got, found := tracers.ExtractTraceContext(ctx)

	if !found {
		t.Fatal("expected found=true")
	}
	if got.GetTraceParent() != wantTraceParent {
		t.Fatalf("GetTraceParent() = %q, want %q", got.GetTraceParent(), wantTraceParent)
	}
	if got.GetTraceState() != wantTraceState {
		t.Fatalf("GetTraceState() = %q, want %q", got.GetTraceState(), wantTraceState)
	}
}

func TestExtractTraceContext_datadogPreservesUpstreamTraceState(t *testing.T) {
	ctx, span := datadogContextFromTraceParent(t,
		"00-0102030405060708090a0b0c0d0e0f10-100f0e0d0c0b0a09-01",
		"vendor=value",
	)

	wantTraceParent, wantTraceState := datadogW3CPropagation(t, span)
	got, found := tracers.ExtractTraceContext(ctx)

	if !found {
		t.Fatal("expected found=true")
	}
	if got.GetTraceParent() != wantTraceParent {
		t.Fatalf("GetTraceParent() = %q, want %q", got.GetTraceParent(), wantTraceParent)
	}
	if got.GetTraceState() != wantTraceState {
		t.Fatalf("GetTraceState() = %q, want %q", got.GetTraceState(), wantTraceState)
	}
}

func TestExtractTraceContext_unsampledOpenTelemetry(t *testing.T) {
	spanContext := oteltrace.NewSpanContext(oteltrace.SpanContextConfig{
		TraceID: oteltrace.TraceID{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16},
		SpanID:  oteltrace.SpanID{16, 15, 14, 13, 12, 11, 10, 9},
	})
	ctx := oteltrace.ContextWithSpanContext(context.Background(), spanContext)

	got, found := tracers.ExtractTraceContext(ctx)
	wantTraceParent, wantTraceState := openTelemetryTraceInfo(spanContext)
	assertTraceInfo(
		t,
		got,
		wantTraceParent,
		wantTraceState,
		openTelemetryTraceID(spanContext, tracers.FormatString),
		openTelemetryTraceID(spanContext, tracers.FormatInt64),
		spanContext.SpanID().String(),
	)
	if !found {
		t.Fatal("expected found=true")
	}
}

func TestNewTraceInfoFromTraceParent(t *testing.T) {
	traceParent := "00-0102030405060708090a0b0c0d0e0f10-100f0e0d0c0b0a09-01"

	got := tracers.NewTraceInfoFromTraceParent(traceParent, "vendor=value")
	assertTraceInfo(
		t,
		got,
		traceParent,
		"vendor=value",
		"0102030405060708090a0b0c0d0e0f10",
		"651345242494996240",
		"100f0e0d0c0b0a09",
	)
}

func TestNewTraceInfoFromTraceParent_invalid(t *testing.T) {
	for _, testCase := range []struct {
		name        string
		traceParent string
	}{
		{name: "empty traceparent"},
		{name: "unsupported version", traceParent: "ff-0102030405060708090a0b0c0d0e0f10-100f0e0d0c0b0a09-01"},
		{name: "uppercase trace id", traceParent: "00-0102030405060708090A0b0c0d0e0f10-100f0e0d0c0b0a09-01"},
		{name: "zero trace id", traceParent: "00-00000000000000000000000000000000-100f0e0d0c0b0a09-01"},
		{name: "zero span id", traceParent: "00-0102030405060708090a0b0c0d0e0f10-0000000000000000-01"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			if got := tracers.NewTraceInfoFromTraceParent(testCase.traceParent, ""); got != (tracers.TraceInfo{}) {
				t.Fatalf("expected the zero value, got %#v", got)
			}
		})
	}
}

func TestNewTraceInfoFromTraceParent_usesOfficialPropagation(t *testing.T) {
	traceParent := "02-0102030405060708090a0b0c0d0e0f10-100f0e0d0c0b0a09-01-extra"

	got := tracers.NewTraceInfoFromTraceParent(traceParent, "invalid")
	assertTraceInfo(t, got, traceParent, "", "0102030405060708090a0b0c0d0e0f10", "651345242494996240", "100f0e0d0c0b0a09")
}

func openTelemetryContext(t *testing.T) (context.Context, oteltrace.SpanContext) {
	return openTelemetryContextFrom(t, context.Background())
}

func openTelemetryContextFrom(t *testing.T, parent context.Context) (context.Context, oteltrace.SpanContext) {
	t.Helper()

	traceState, err := oteltrace.ParseTraceState("vendor=value")
	if err != nil {
		t.Fatalf("parse tracestate: %v", err)
	}
	spanContext := oteltrace.NewSpanContext(oteltrace.SpanContextConfig{
		TraceID:    oteltrace.TraceID{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16},
		SpanID:     oteltrace.SpanID{16, 15, 14, 13, 12, 11, 10, 9},
		TraceFlags: oteltrace.FlagsSampled,
		TraceState: traceState,
	})
	return oteltrace.ContextWithSpanContext(parent, spanContext), spanContext
}

func datadogContext(t *testing.T) (context.Context, *ddtracer.Span) {
	t.Helper()

	mockTracer := ddmocktracer.Start()
	t.Cleanup(mockTracer.Stop)
	span, ctx := ddtracer.StartSpanFromContext(
		context.Background(),
		"operation",
		ddtracer.WithSpanID(0x1234),
	)
	return ctx, span
}

func datadogContextFromTraceParent(t *testing.T, traceParent, traceState string) (context.Context, *ddtracer.Span) {
	t.Helper()

	mockTracer := ddmocktracer.Start()
	t.Cleanup(mockTracer.Stop)
	span, ctx := ddtracer.StartSpanFromPropagatedContext(
		context.Background(),
		"operation",
		ddtracer.TextMapCarrier{
			"traceparent": traceParent,
			"tracestate":  traceState,
		},
	)
	return ctx, span
}

func assertTraceInfo(t *testing.T, got tracers.TraceInfo, wantTraceParent, wantTraceState, wantTraceIDString, wantTraceIDInt64, wantSpanID string) {
	t.Helper()
	if got.GetTraceParent() != wantTraceParent {
		t.Fatalf("GetTraceParent() = %q, want %q", got.GetTraceParent(), wantTraceParent)
	}
	if got.GetTraceState() != wantTraceState {
		t.Fatalf("GetTraceState() = %q, want %q", got.GetTraceState(), wantTraceState)
	}
	if got.GetTraceID(tracers.FormatString) != wantTraceIDString {
		t.Fatalf("GetTraceID(FormatString) = %q, want %q", got.GetTraceID(tracers.FormatString), wantTraceIDString)
	}
	if got.GetTraceID(tracers.FormatInt64) != wantTraceIDInt64 {
		t.Fatalf("GetTraceID(FormatInt64) = %q, want %q", got.GetTraceID(tracers.FormatInt64), wantTraceIDInt64)
	}
	if got.GetSpanID() != wantSpanID {
		t.Fatalf("GetSpanID() = %q, want %q", got.GetSpanID(), wantSpanID)
	}
}

func openTelemetryTraceInfo(spanContext oteltrace.SpanContext) (string, string) {
	traceID := spanContext.TraceID()
	spanID := spanContext.SpanID().String()
	return fmt.Sprintf("00-%s-%s-%02x", traceID, spanID, byte(spanContext.TraceFlags())), spanContext.TraceState().String()
}

func openTelemetryTraceID(spanContext oteltrace.SpanContext, format tracers.TraceIDFormat) string {
	traceID := spanContext.TraceID()
	if format == tracers.FormatInt64 {
		return strconv.FormatUint(binary.BigEndian.Uint64(traceID[8:]), 10)
	}
	return traceID.String()
}

func datadogTraceInfo(span *ddtracer.Span) (string, string) {
	return datadogW3CPropagation(nil, span)
}

func datadogW3CPropagation(t *testing.T, span *ddtracer.Span) (string, string) {
	if t != nil {
		t.Helper()
	}
	carrier := make(ddtracer.TextMapCarrier)
	if err := ddtracer.Inject(span.Context(), carrier); err != nil {
		if t != nil {
			t.Fatalf("inject Datadog propagation: %v", err)
		}
		return "", ""
	}
	return carrier["traceparent"], carrier["tracestate"]
}

func datadogTraceID(span *ddtracer.Span, format tracers.TraceIDFormat) string {
	if format == tracers.FormatInt64 {
		return strconv.FormatUint(span.Context().TraceIDLower(), 10)
	}
	traceIDBytes := span.Context().TraceIDBytes()
	return hex.EncodeToString(traceIDBytes[:])
}
