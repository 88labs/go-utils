// Package backoff implements exponential backoff with jitter for retrying
// transient-error-prone operations such as HTTP requests, gRPC calls, and
// database queries.
//
// The default algorithm uses the "Full Jitter" strategy recommended by AWS:
// https://aws.amazon.com/blogs/architecture/exponential-backoff-and-jitter/
//
// Full jitter distributes retry timing randomly across clients, which prevents
// the thundering-herd problem that occurs when many callers retry in lockstep.
//
// Basic usage:
//
//	err := backoff.Do(ctx, func(ctx context.Context) error {
//	    return client.Call(ctx, req)
//	})
//
// Custom options:
//
//	err := backoff.Do(ctx, func(ctx context.Context) error {
//	    return client.Call(ctx, req)
//	},
//	    backoff.WithMaxRetries(5),
//	    backoff.WithInitialInterval(200*time.Millisecond),
//	    backoff.WithRetryIf(func(err error) bool {
//	        return !errors.Is(err, ErrNotFound) // skip retries for 404s
//	    }),
//	    backoff.WithOnRetry(func(attempt int, err error) {
//	        slog.Warn("retrying", "attempt", attempt, "error", err)
//	    }),
//	)
package backoff

import (
	"context"
	"math"
	"math/rand/v2"
	"time"
)

const (
	defaultMaxRetries      = 3
	defaultInitialInterval = 100 * time.Millisecond
	defaultMaxInterval     = 30 * time.Second
	defaultMultiplier      = 2.0
	defaultJitter          = 1.0
)

// config holds the resolved backoff configuration.
type config struct {
	maxRetries      int
	initialInterval time.Duration
	maxInterval     time.Duration
	multiplier      float64
	jitter          float64
	retryIf         func(error) bool
	onRetry         func(attempt int, err error)
}

func defaultConfig() config {
	return config{
		maxRetries:      defaultMaxRetries,
		initialInterval: defaultInitialInterval,
		maxInterval:     defaultMaxInterval,
		multiplier:      defaultMultiplier,
		jitter:          defaultJitter,
		retryIf:         func(error) bool { return true },
	}
}

// Option configures the backoff behavior.
type Option func(*config)

// WithMaxRetries sets the maximum number of retry attempts (default: 3).
// The operation is attempted at most n+1 times in total (1 initial + n retries).
// Pass 0 to run the operation exactly once with no retries.
func WithMaxRetries(n int) Option {
	return func(c *config) { c.maxRetries = n }
}

// WithInitialInterval sets the base wait duration before the first retry (default: 100ms).
func WithInitialInterval(d time.Duration) Option {
	return func(c *config) { c.initialInterval = d }
}

// WithMaxInterval caps the maximum wait duration between retries (default: 30s).
func WithMaxInterval(d time.Duration) Option {
	return func(c *config) { c.maxInterval = d }
}

// WithMultiplier sets the growth factor applied to the interval after each attempt
// (default: 2.0). A multiplier of 2 doubles the cap each retry.
func WithMultiplier(m float64) Option {
	return func(c *config) { c.multiplier = m }
}

// WithJitter sets the fraction of randomness applied to wait times (default: 1.0).
//
//   - 1.0 (full jitter): sleep is uniformly random in [0, cap].
//   - 0.0 (no jitter):   sleep equals cap (deterministic exponential backoff).
//   - Values in between: linearly interpolated between the two extremes.
func WithJitter(j float64) Option {
	return func(c *config) { c.jitter = j }
}

// WithRetryIf sets a predicate that determines whether an error should trigger
// a retry (default: all errors are retried). Return false to stop retrying
// immediately, e.g. for HTTP 4xx or gRPC InvalidArgument errors.
func WithRetryIf(fn func(error) bool) Option {
	return func(c *config) { c.retryIf = fn }
}

// WithOnRetry registers a callback invoked before each retry.
// attempt is 1-indexed: attempt=1 means one failure has occurred and the first
// retry is about to start. Useful for logging or recording metrics.
func WithOnRetry(fn func(attempt int, err error)) Option {
	return func(c *config) { c.onRetry = fn }
}

// Do calls fn, retrying on error with exponential backoff and jitter.
//
// Retrying stops when any of the following conditions are met:
//   - fn returns nil (success).
//   - The context is done (returns ctx.Err()).
//   - WithRetryIf returns false for the error (returns the error immediately).
//   - The retry limit is exhausted (returns the last error from fn).
func Do(ctx context.Context, fn func(ctx context.Context) error, opts ...Option) error {
	c := defaultConfig()
	for _, opt := range opts {
		opt(&c)
	}

	var lastErr error
	for attempt := range c.maxRetries + 1 {
		if attempt > 0 {
			timer := time.NewTimer(sleepDuration(c, attempt))
			select {
			case <-ctx.Done():
				timer.Stop()
				return ctx.Err()
			case <-timer.C:
			}
		}

		lastErr = fn(ctx)
		if lastErr == nil {
			return nil
		}

		// Respect context cancellation even if fn did not propagate it.
		if ctx.Err() != nil {
			return ctx.Err()
		}

		// Non-retryable error: surface immediately.
		if !c.retryIf(lastErr) {
			return lastErr
		}

		// Fire the callback for every failure that will be followed by a retry.
		if c.onRetry != nil && attempt < c.maxRetries {
			c.onRetry(attempt+1, lastErr)
		}
	}

	return lastErr
}

// sleepDuration returns the wait time before retry number attempt (1-indexed).
//
// Formula:
//
//	cap   = min(maxInterval, initialInterval × multiplier^(attempt-1))
//	sleep = cap×(1−jitter) + rand(0,cap)×jitter
//
// With jitter=1 (full): sleep is uniform in [0, cap].
// With jitter=0 (none): sleep equals cap deterministically.
func sleepDuration(c config, attempt int) time.Duration {
	cap := min(
		float64(c.maxInterval),
		float64(c.initialInterval)*math.Pow(c.multiplier, float64(attempt-1)),
	)
	sleep := cap*(1-c.jitter) + rand.Float64()*cap*c.jitter
	return time.Duration(sleep)
}
