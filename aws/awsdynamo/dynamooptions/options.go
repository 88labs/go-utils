package dynamooptions

import "time"

type OptionDynamo interface {
	Apply(*confDynamo)
}

type confDynamo struct {
	MaxAttempts     int
	MaxBackoffDelay time.Duration
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

// nolint:revive
func GetDynamoConf(opts ...OptionDynamo) confDynamo {
	// default
	// https://aws.github.io/aws-sdk-go-v2/docs/configuring-sdk/retries-timeouts/#standard-retryer
	c := confDynamo{
		MaxAttempts:     3,
		MaxBackoffDelay: 20 * time.Second,
	}
	for _, opt := range opts {
		opt.Apply(&c)
	}
	return c
}
