package awstrace

import (
	"context"

	"github.com/aws/smithy-go/middleware"
	"go.opentelemetry.io/contrib/instrumentation/github.com/aws/aws-sdk-go-v2/otelaws"
	"go.opentelemetry.io/otel/propagation"
	oteltrace "go.opentelemetry.io/otel/trace"

	"github.com/88labs/go-utils/tracers"
)

// Config configures the tracing middleware shared by the AWS SDK wrappers.
//
// A nil TracerProvider or Propagator is resolved by otelaws from the
// corresponding OpenTelemetry global setting. SQS supplies its message
// propagator here as well, so SDK request headers and SQS message attributes
// use the same propagation configuration.
type Config struct {
	TracerProvider oteltrace.TracerProvider
	Propagator     propagation.TextMapPropagator
}

// AppendMiddlewares enables OpenTelemetry instrumentation for an AWS SDK v2
// client. Datadog v2 spans in the request context are bridged to an
// OpenTelemetry SpanContext before the instrumentation starts its child span.
// The same helper is used by every AWS service wrapper; service-specific
// propagation (for example, SQS message attributes) is configured through
// Config without changing this SDK middleware boundary.
func AppendMiddlewares(
	apiOptions *[]func(*middleware.Stack) error,
	cfg Config,
) {
	// This middleware must be registered before otelaws' initialize middleware
	// so a Datadog span can be used as the parent of the AWS span.
	*apiOptions = append(*apiOptions, addDatadogTraceBridge)

	options := make([]otelaws.Option, 0, 2)
	if cfg.TracerProvider != nil {
		options = append(options, otelaws.WithTracerProvider(cfg.TracerProvider))
	}
	if cfg.Propagator != nil {
		options = append(options, otelaws.WithTextMapPropagator(cfg.Propagator))
	}
	otelaws.AppendMiddlewares(apiOptions, options...)
}

func addDatadogTraceBridge(stack *middleware.Stack) error {
	return stack.Initialize.Add(datadogTraceBridge{}, middleware.Before)
}

// ContextWithDatadogParent adds a Datadog span as an OpenTelemetry parent when
// the context does not already contain a valid OpenTelemetry SpanContext.
func ContextWithDatadogParent(ctx context.Context) context.Context {
	if oteltrace.SpanContextFromContext(ctx).IsValid() {
		return ctx
	}

	spanContext, ok := datadogSpanContext(ctx)
	if !ok {
		return ctx
	}
	return oteltrace.ContextWithSpanContext(ctx, spanContext)
}

type datadogTraceBridge struct{}

func (datadogTraceBridge) ID() string {
	return "go-utils/aws/DatadogTraceBridge"
}

func (datadogTraceBridge) HandleInitialize(
	ctx context.Context,
	in middleware.InitializeInput,
	next middleware.InitializeHandler,
) (middleware.InitializeOutput, middleware.Metadata, error) {
	return next.HandleInitialize(ContextWithDatadogParent(ctx), in)
}

func datadogSpanContext(ctx context.Context) (oteltrace.SpanContext, bool) {
	traceInfo, ok := tracers.ExtractTraceContext(ctx)
	if !ok {
		return oteltrace.SpanContext{}, false
	}

	// tracers normalizes OpenTelemetry and Datadog context into W3C fields.
	carrier := propagation.MapCarrier{
		"traceparent": traceInfo.GetTraceParent(),
	}
	if traceInfo.GetTraceState() != "" {
		carrier["tracestate"] = traceInfo.GetTraceState()
	}
	spanContext := oteltrace.SpanContextFromContext(
		propagation.TraceContext{}.Extract(context.Background(), carrier),
	)
	if !spanContext.IsValid() {
		return oteltrace.SpanContext{}, false
	}
	return spanContext, true
}
