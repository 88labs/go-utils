package dynamooptions

import (
	"time"

	oteltrace "go.opentelemetry.io/otel/trace"
)

type OptionDynamo interface {
	Apply(*confDynamo)
}

type confDynamo struct {
	MaxAttempts     int
	MaxBackoffDelay time.Duration
	traceProvider   oteltrace.TracerProvider
	traceEnabled    bool
}

type OptionMaxAttempts int

func (o OptionMaxAttempts) Apply(c *confDynamo) {
	c.MaxAttempts = int(o)
}

func WithMaxAttempts(maxAttempts int) OptionMaxAttempts {
	return OptionMaxAttempts(maxAttempts)
}

type OptionMaxBackoffDelay time.Duration

func (o OptionMaxBackoffDelay) Apply(c *confDynamo) {
	c.MaxBackoffDelay = time.Duration(o)
}

func WithMaxBackoffDelay(maxBackoffDelay time.Duration) OptionMaxBackoffDelay {
	return OptionMaxBackoffDelay(maxBackoffDelay)
}

type optionTrace struct {
	provider oteltrace.TracerProvider
}

func (o optionTrace) Apply(c *confDynamo) {
	c.traceProvider = o.provider
	c.traceEnabled = true
}

// WithTrace enables OpenTelemetry tracing for an independently created
// DynamoDB client. A nil provider uses the globally configured provider.
// Datadog v2 spans in request contexts are also accepted as trace parents.
func WithTrace(provider oteltrace.TracerProvider) OptionDynamo {
	return optionTrace{provider: provider}
}

// TraceProvider returns the provider configured for the DynamoDB client.
func (c confDynamo) TraceProvider() oteltrace.TracerProvider {
	return c.traceProvider
}

// TraceEnabled reports whether tracing was explicitly enabled.
func (c confDynamo) TraceEnabled() bool {
	return c.traceEnabled
}

// nolint:revive
func GetDynamoConf(opts ...OptionDynamo) confDynamo {
	// default
	// https://aws.github.io/aws-sdk-go-v2/docs/configuring-sdk/retries-timeouts/#standard-retryer
	c := confDynamo{
		MaxAttempts:     3,
		MaxBackoffDelay: 20 * time.Second,
	}
	for _, opt := range opts {
		if opt != nil {
			opt.Apply(&c)
		}
	}
	return c
}
