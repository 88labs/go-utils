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

// TraceConfig configures OpenTelemetry tracing for an SQS Client.
//
// A zero TraceConfig uses the globally configured TracerProvider and the
// standard W3C Trace Context propagator. The W3C Trace Context propagator is
// always included when a custom Propagator is supplied.
// TraceConfig does not initialize a TracerProvider. If TracerProvider is nil,
// configure the global TracerProvider in the application's entry point, such
// as main, before creating the Client or initializing the package singleton.
type TraceConfig struct {
	// TracerProvider creates spans for SQS operations. A nil provider uses the
	// globally configured OpenTelemetry TracerProvider. With a nil provider,
	// the global provider must be configured before NewClient or the first
	// GetClient call.
	TracerProvider oteltrace.TracerProvider

	// Propagator propagates additional context in SQS message attributes. A
	// nil propagator uses only the standard W3C Trace Context propagator. To
	// propagate W3C Baggage, provide a custom propagator such as
	// propagation.Baggage{}; W3C Trace Context is always included automatically.
	Propagator propagation.TextMapPropagator
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

var defaultTracePropagator propagation.TextMapPropagator = propagation.TraceContext{}

// WithTrace enables OpenTelemetry tracing using config. A zero config uses the
// globally configured TracerProvider and the standard W3C Trace Context
// propagator. A nil TracerProvider uses the globally configured
// provider. A non-nil Propagator is composed after the mandatory W3C Trace
// Context propagator. WithTrace does not initialize or register a global
// TracerProvider; applications must configure it in an entry point such as
// main before creating the Client or initializing the package singleton. If
// the configured provider cannot create a valid span context, traced
// operations return ErrTraceProviderNotConfigured. Datadog v2 spans in
// request contexts are also accepted as trace parents.
//
// The propagator's fields are reserved SQS message attributes. The standard
// Trace Context propagator reserves two attributes, leaving at most eight
// attributes for application data. A custom propagator may reserve additional
// fields, including baggage. The propagator is injected and extracted directly
// by this client; the global TextMapPropagator is not read or changed.
func WithTrace(config TraceConfig) ClientOption {
	return clientOptionFunc(func(cfg *clientConfig) {
		resolvedProvider := config.TracerProvider
		if resolvedProvider == nil {
			resolvedProvider = otel.GetTracerProvider()
		}

		cfg.traceProvider = resolvedProvider
		if config.Propagator == nil {
			cfg.tracePropagator = defaultTracePropagator
		} else {
			cfg.tracePropagator = withRequiredTraceContext(config.Propagator)
		}
		cfg.traceEnabled = true
	})
}

func withRequiredTraceContext(additional propagation.TextMapPropagator) propagation.TextMapPropagator {
	if additional == nil {
		return defaultTracePropagator
	}
	return propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		additional,
	)
}
