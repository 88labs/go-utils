package awstrace

import (
	"context"

	"github.com/aws/smithy-go/middleware"
	"go.opentelemetry.io/contrib/instrumentation/github.com/aws/aws-sdk-go-v2/otelaws"
	"go.opentelemetry.io/otel/propagation"
	oteltrace "go.opentelemetry.io/otel/trace"

	"github.com/88labs/go-utils/tracers"
)

// AppendMiddlewares enables OpenTelemetry instrumentation for an AWS SDK v2
// client. Datadog v2 spans in the request context are bridged to an
// OpenTelemetry SpanContext before the instrumentation starts its child span.
func AppendMiddlewares(
	apiOptions *[]func(*middleware.Stack) error,
	provider oteltrace.TracerProvider,
) {
	// This middleware must be registered before otelaws' initialize middleware
	// so a Datadog span can be used as the parent of the AWS span.
	*apiOptions = append(*apiOptions, addDatadogTraceBridge)

	if provider == nil {
		otelaws.AppendMiddlewares(apiOptions)
		return
	}
	otelaws.AppendMiddlewares(apiOptions, otelaws.WithTracerProvider(provider))
}

func addDatadogTraceBridge(stack *middleware.Stack) error {
	return stack.Initialize.Add(datadogTraceBridge{}, middleware.Before)
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
	if oteltrace.SpanContextFromContext(ctx).IsValid() {
		return next.HandleInitialize(ctx, in)
	}

	spanContext, ok := datadogSpanContext(ctx)
	if ok {
		ctx = oteltrace.ContextWithSpanContext(ctx, spanContext)
	}
	return next.HandleInitialize(ctx, in)
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
