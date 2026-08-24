package awssqs

import (
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	oteltrace "go.opentelemetry.io/otel/trace"
)

// ClientOption configures a Client created with NewClient.
type ClientOption interface {
	apply(*clientConfig)
}

type clientConfig struct {
	traceProvider   oteltrace.TracerProvider
	tracePropagator propagation.TextMapPropagator
	traceEnabled    bool
}

type clientOptionFunc func(*clientConfig)

func (f clientOptionFunc) apply(cfg *clientConfig) {
	f(cfg)
}

// WithTraceDefault enables OpenTelemetry tracing using the globally configured
// TracerProvider and the default SQS propagator. The default propagator always
// includes W3C Trace Context and also propagates W3C Baggage. It does not read
// or change the global TextMapPropagator.
//
// WithTraceDefault reserves up to three SQS message attributes
// (traceparent, tracestate, and baggage), leaving at most seven attributes for
// application data. If no SDK TracerProvider is configured, requests fail with
// ErrTraceProviderNotConfigured when a trace span is required.
func WithTraceDefault() ClientOption {
	return clientOptionFunc(func(cfg *clientConfig) {
		cfg.traceProvider = otel.GetTracerProvider()
		cfg.tracePropagator = propagation.NewCompositeTextMapPropagator(
			propagation.TraceContext{},
			propagation.Baggage{},
		)
		cfg.traceEnabled = true
	})
}

// WithTrace enables OpenTelemetry tracing using provider and an optional
// additional propagator. The client always includes W3C Trace Context; the
// additional propagator is composed after it. Pass propagation.Baggage{} to
// propagate W3C Baggage. A nil provider uses the globally configured
// TracerProvider. Datadog v2 spans in request contexts are also accepted as
// trace parents.
//
// The propagator's fields are reserved SQS message attributes. With the
// standard Trace Context and Baggage propagators, up to three attributes are
// reserved, leaving at most seven attributes for application data. The
// propagator is injected and extracted directly by this client; the global
// TextMapPropagator is not read or changed.
func WithTrace(
	provider oteltrace.TracerProvider,
	propagator propagation.TextMapPropagator,
) ClientOption {
	return clientOptionFunc(func(cfg *clientConfig) {
		resolvedProvider := provider
		if resolvedProvider == nil {
			resolvedProvider = otel.GetTracerProvider()
		}

		cfg.traceProvider = resolvedProvider
		cfg.tracePropagator = withRequiredTraceContext(propagator)
		cfg.traceEnabled = true
	})
}

func withRequiredTraceContext(additional propagation.TextMapPropagator) propagation.TextMapPropagator {
	if additional == nil {
		return propagation.TraceContext{}
	}
	return propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		additional,
	)
}
