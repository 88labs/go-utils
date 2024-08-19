package s3head

import "time"

type OptionS3Head interface {
	Apply(*confS3Head)
}

type confS3Head struct {
	// Timeout can specify the number of seconds to wait when using a waiter
	// to check for the existence of an object.
	Timeout time.Duration

	// MinDelay is the minimum amount of time to delay between retries. If unset,
	// ObjectExistsWaiter will use default minimum delay of 5 seconds. Note that
	// MinDelay must resolve to a value lesser than or equal to the MaxDelay.
	MinDelay time.Duration

	// MaxDelay is the maximum amount of time to delay between retries. If unset or set
	// to zero, ObjectExistsWaiter will use default max delay of 120 seconds. Note that
	// MaxDelay must resolve to value greater than or equal to the MinDelay.
	MaxDelay time.Duration

	// LogWaitAttempts is used to enable logging for waiter retry attempts
	LogWaitAttempts bool
}

type OptionTimeout time.Duration

func (o OptionTimeout) Apply(c *confS3Head) {
	c.Timeout = time.Duration(o)
}

// WithTimeout
// Timeout can specify the number of seconds to wait when using a waiter
// to check for the existence of an object.
func WithTimeout(duration time.Duration) OptionTimeout {
	return OptionTimeout(duration)
}

type OptionMinDelay time.Duration

func (o OptionMinDelay) Apply(c *confS3Head) {
	c.MinDelay = time.Duration(o)
}

// WithMinDelay
// MinDelay is the minimum amount of time to delay between retries. If unset,
// ObjectExistsWaiter will use default minimum delay of 5 seconds. Note that
// MinDelay must resolve to a value lesser than or equal to the MaxDelay.
func WithMinDelay(minDelay time.Duration) OptionMinDelay {
	return OptionMinDelay(minDelay)
}

type OptionMaxDelay time.Duration

func (o OptionMaxDelay) Apply(c *confS3Head) {
	c.MaxDelay = time.Duration(o)
}

// WithMaxDelay
// MaxDelay is the maximum amount of time to delay between retries. If unset or set
// to zero, ObjectExistsWaiter will use default max delay of 120 seconds. Note that
// MaxDelay must resolve to value greater than or equal to the MinDelay.
func WithMaxDelay(maxDelay time.Duration) OptionMaxDelay {
	return OptionMaxDelay(maxDelay)
}

type OptionLogWaitAttempts bool

func (o OptionLogWaitAttempts) Apply(c *confS3Head) {
	c.LogWaitAttempts = bool(o)
}

// WithLogWaitAttempts
// LogWaitAttempts is used to enable logging for waiter retry attempts
func WithLogWaitAttempts(logWaitAttempts bool) OptionLogWaitAttempts {
	return OptionLogWaitAttempts(logWaitAttempts)
}

// nolint:revive
func GetS3HeadConf(opts ...OptionS3Head) confS3Head {
	c := confS3Head{
		MinDelay:        5 * time.Second,
		MaxDelay:        120 * time.Second,
		LogWaitAttempts: false,
	}
	for _, opt := range opts {
		opt.Apply(&c)
	}
	return c
}
