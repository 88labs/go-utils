package awssqs

import oteltrace "go.opentelemetry.io/otel/trace"

// ClientOption configures a Client created with NewClient.
type ClientOption interface {
	apply(*clientConfig)
}

type clientConfig struct {
	traceProvider   oteltrace.TracerProvider
	traceEnabled    bool
	sendSpanName    string
	processSpanName string
}

type clientOptionFunc func(*clientConfig)

func (f clientOptionFunc) apply(cfg *clientConfig) {
	f(cfg)
}

// WithTrace enables OpenTelemetry tracing for AWS SDK requests and SQS message
// propagation created by the client. If no provider is supplied, the globally
// configured OpenTelemetry provider is used. Datadog v2 spans in request
// contexts are also accepted as trace parents.
func WithTrace(providers ...oteltrace.TracerProvider) ClientOption {
	var provider oteltrace.TracerProvider
	if len(providers) > 0 {
		provider = providers[0]
	}
	return clientOptionFunc(func(cfg *clientConfig) {
		cfg.traceProvider = provider
		cfg.traceEnabled = true
	})
}

// WithSendSpanName configures the name used for SQS send spans. The value is
// used as-is instead of the default "send <queue>" name.
func WithSendSpanName(name string) ClientOption {
	return clientOptionFunc(func(cfg *clientConfig) {
		cfg.sendSpanName = name
	})
}

// WithProcessSpanName configures the name used for SQS process spans. The value
// is used as-is instead of the default "process <queue>" name.
func WithProcessSpanName(name string) ClientOption {
	return clientOptionFunc(func(cfg *clientConfig) {
		cfg.processSpanName = name
	})
}
