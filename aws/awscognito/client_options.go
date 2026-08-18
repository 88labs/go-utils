package awscognito

import oteltrace "go.opentelemetry.io/otel/trace"

// ClientOption configures a Client created with NewClient.
type ClientOption interface {
	apply(*clientConfig)
}

type clientConfig struct {
	traceProvider oteltrace.TracerProvider
	traceEnabled  bool
}

type clientOptionFunc func(*clientConfig)

func (f clientOptionFunc) apply(cfg *clientConfig) {
	f(cfg)
}

// WithTrace enables OpenTelemetry tracing for AWS SDK requests created by the
// client. A nil provider uses the globally configured OpenTelemetry provider.
// Datadog v2 spans in request contexts are also accepted as trace parents.
func WithTrace(provider oteltrace.TracerProvider) ClientOption {
	return clientOptionFunc(func(cfg *clientConfig) {
		cfg.traceProvider = provider
		cfg.traceEnabled = true
	})
}
