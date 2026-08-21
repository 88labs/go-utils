package awssqs

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

// WithTrace enables OpenTelemetry tracing for AWS SDK requests and SQS message
// propagation created by the client. A nil provider uses the globally
// configured OpenTelemetry provider. Datadog v2 spans in request contexts are
// also accepted as trace parents.
//
// SendMessage and SendMessageBatch reserve the traceparent and tracestate SQS
// message attributes, leaving at most eight application attributes. They
// return an error before sending when the W3C propagator or trace provider is
// not usable, when a reserved attribute is supplied, or when the attribute
// limit would be exceeded. The trace attributes are encoded as SQS String
// values and baggage is not propagated. SendMessageBatch can still return a
// nil Go error with per-entry failures in its Failed field; callers must
// inspect it.
// ProcessMessage uses the worker context as the process-span parent and the
// incoming sender context as a link. Malformed incoming trace attributes are
// recorded on that span but do not stop the handler or acknowledgement.
func WithTrace(provider oteltrace.TracerProvider) ClientOption {
	return clientOptionFunc(func(cfg *clientConfig) {
		cfg.traceProvider = provider
		cfg.traceEnabled = true
	})
}
