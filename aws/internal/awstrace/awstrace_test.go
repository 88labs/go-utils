package awstrace

import (
	"context"
	"net/http"
	"testing"

	ddmocktracer "github.com/DataDog/dd-trace-go/v2/ddtrace/mocktracer"
	ddtracer "github.com/DataDog/dd-trace-go/v2/ddtrace/tracer"
	"github.com/aws/smithy-go/middleware"
	smithyhttp "github.com/aws/smithy-go/transport/http"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
)

func TestAppendMiddlewares_bridgesDatadogParent(t *testing.T) {
	ddMockTracer := ddmocktracer.Start()
	t.Cleanup(ddMockTracer.Stop)

	ddSpan, ctx := ddtracer.StartSpanFromContext(
		context.Background(),
		"parent",
		ddtracer.WithSpanID(0x1234),
	)
	t.Cleanup(func() { ddSpan.Finish() })

	provider := newRecordingProvider()
	runInitialize(t, ctx, provider)
	parent := <-provider.tracer.parents

	if got, want := parent.TraceID().String(), ddSpan.Context().TraceIDBytes(); got != trace.TraceID(want).String() {
		t.Fatalf("parent trace ID = %q, want %q", got, trace.TraceID(want).String())
	}
	if got, want := parent.SpanID().String(), "0000000000001234"; got != want {
		t.Fatalf("parent span ID = %q, want %q", got, want)
	}
}

func TestAppendMiddlewares_preservesOpenTelemetryParent(t *testing.T) {
	parent := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID: trace.TraceID{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16},
		SpanID:  trace.SpanID{16, 15, 14, 13, 12, 11, 10, 9},
	})
	ctx := trace.ContextWithSpanContext(context.Background(), parent)

	provider := newRecordingProvider()
	runInitialize(t, ctx, provider)
	got := <-provider.tracer.parents

	if !got.Equal(parent) {
		t.Fatalf("parent span context = %v, want %v", got, parent)
	}
}

func TestAppendMiddlewares_usesConfiguredPropagator(t *testing.T) {
	var apiOptions []func(*middleware.Stack) error
	propagator := recordingPropagator{}
	AppendMiddlewares(&apiOptions, Config{Propagator: propagator})

	stack := middleware.NewStack("test", func() interface{} { return nil })
	for _, apiOption := range apiOptions {
		if err := apiOption(stack); err != nil {
			t.Fatalf("add API middleware: %v", err)
		}
	}

	input := &smithyhttp.Request{
		Request: &http.Request{Header: http.Header{}},
	}
	_, _, err := stack.Finalize.HandleMiddleware(
		context.Background(),
		input,
		middleware.HandlerFunc(func(context.Context, interface{}) (interface{}, middleware.Metadata, error) {
			return nil, middleware.Metadata{}, nil
		}),
	)
	if err != nil {
		t.Fatalf("run finalize middleware: %v", err)
	}
	if got, want := input.Header.Get("x-test-propagator"), "injected"; got != want {
		t.Fatalf("propagated header = %q, want %q", got, want)
	}
}

func runInitialize(t *testing.T, ctx context.Context, provider trace.TracerProvider) {
	t.Helper()

	var apiOptions []func(*middleware.Stack) error
	AppendMiddlewares(&apiOptions, Config{TracerProvider: provider})
	stack := middleware.NewStack("test", func() interface{} { return nil })
	for _, apiOption := range apiOptions {
		if err := apiOption(stack); err != nil {
			t.Fatalf("add API middleware: %v", err)
		}
	}

	_, _, err := stack.Initialize.HandleMiddleware(ctx, struct{}{}, middleware.HandlerFunc(
		func(context.Context, interface{}) (interface{}, middleware.Metadata, error) {
			return nil, middleware.Metadata{}, nil
		},
	))
	if err != nil {
		t.Fatalf("run initialize middleware: %v", err)
	}
}

type recordingProvider struct {
	noop.TracerProvider
	tracer *recordingTracer
}

func newRecordingProvider() *recordingProvider {
	return &recordingProvider{
		tracer: &recordingTracer{
			parents: make(chan trace.SpanContext, 1),
		},
	}
}

func (p *recordingProvider) Tracer(string, ...trace.TracerOption) trace.Tracer {
	return p.tracer
}

type recordingTracer struct {
	noop.Tracer
	parents chan trace.SpanContext
}

func (t *recordingTracer) Start(
	ctx context.Context, name string, options ...trace.SpanStartOption,
) (context.Context, trace.Span) {
	t.parents <- trace.SpanContextFromContext(ctx)
	return t.Tracer.Start(ctx, name, options...)
}

type recordingPropagator struct{}

func (recordingPropagator) Inject(_ context.Context, carrier propagation.TextMapCarrier) {
	carrier.Set("x-test-propagator", "injected")
}

func (recordingPropagator) Extract(context.Context, propagation.TextMapCarrier) context.Context {
	return context.Background()
}

func (recordingPropagator) Fields() []string {
	return []string{"x-test-propagator"}
}
