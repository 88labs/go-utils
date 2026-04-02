// Package backoff implements Exponential Backoff with Jitter for retrying
// operations that may fail transiently, such as HTTP requests, gRPC calls, and
// database queries.
//
// # Algorithm
//
// The default strategy is "Full Jitter", as recommended by the AWS architecture
// blog (https://aws.amazon.com/blogs/architecture/exponential-backoff-and-jitter/).
// Full Jitter randomises retry timing across clients to prevent the
// thundering-herd effect that arises when many callers fail simultaneously and
// then retry in lockstep.
//
// The sleep duration before retry attempt i (1-indexed) is computed as:
//
//	cap   = min(maxInterval, initialInterval × multiplier^(i-1))
//	sleep = cap×(1−jitter) + rand[0,cap)×jitter
//
// With the default jitter of 1.0 (Full Jitter), sleep is uniformly distributed
// in [0, cap].  With jitter 0.0, sleep equals cap exactly (deterministic
// exponential backoff with no randomness).
//
// # Defaults
//
//	MaxRetries:      3   (4 total attempts: 1 initial + 3 retries)
//	InitialInterval: 100ms
//	MaxInterval:     30s
//	Multiplier:      2.0
//	Jitter:          1.0 (Full Jitter)
//	RetryIf:         all errors are retried
//	OnRetry:         nil (no callback)
//
// # Option Validation
//
// Each option function validates its argument before updating the configuration.
// Do also performs cross-field validation (maxInterval >= initialInterval) after
// all options have been applied.  Any validation failure causes Do to return an
// error immediately without invoking fn.
//
// # Basic Usage
//
//	err := backoff.Do(ctx, func(ctx context.Context) error {
//	    return client.Call(ctx, req)
//	})
//
// # Custom Options
//
//	err := backoff.Do(ctx, func(ctx context.Context) error {
//	    return client.Call(ctx, req)
//	},
//	    backoff.WithMaxRetries(5),
//	    backoff.WithInitialInterval(200*time.Millisecond),
//	    backoff.WithMaxInterval(10*time.Second),
//	    backoff.WithMultiplier(1.5),
//	    backoff.WithRetryIf(func(err error) bool {
//	        // Only retry on 5xx; surface 4xx immediately.
//	        var httpErr *HTTPError
//	        return errors.As(err, &httpErr) && httpErr.StatusCode >= 500
//	    }),
//	    backoff.WithOnRetry(func(attempt int, err error) {
//	        slog.Warn("retrying", "attempt", attempt, "error", err)
//	    }),
//	)
package backoff

import (
	"context"
	"fmt"
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

// config holds the fully resolved backoff configuration after all options have
// been applied.
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

// Option is a functional option that mutates a backoff configuration.
// Each Option validates its argument and returns a non-nil error if the value
// is out of the allowed range.  Do returns that error immediately without
// invoking fn, so callers should treat an Option error as a programming
// mistake rather than a transient failure.
type Option func(*config) error

// WithMaxRetries sets the maximum number of retry attempts after the initial
// call (default: 3).  The total number of times fn is invoked is at most
// n+1 (one initial attempt plus up to n retries).
//
// Pass 0 to disable retries: fn is called exactly once regardless of the
// error it returns.
//
// Constraint: n >= 0.
func WithMaxRetries(n int) Option {
	return func(c *config) error {
		if n < 0 {
			return fmt.Errorf("backoff: WithMaxRetries: n must be >= 0, got %d", n)
		}
		c.maxRetries = n
		return nil
	}
}

// WithInitialInterval sets the base wait duration applied before the first
// retry (default: 100ms).  The interval for retry i grows as:
//
//	cap_i = min(maxInterval, initialInterval × multiplier^(i-1))
//
// Smaller values make the backoff more aggressive; larger values give the
// downstream system more time to recover before the first retry.
//
// Constraint: d > 0.
func WithInitialInterval(d time.Duration) Option {
	return func(c *config) error {
		if d <= 0 {
			return fmt.Errorf("backoff: WithInitialInterval: d must be > 0, got %v", d)
		}
		c.initialInterval = d
		return nil
	}
}

// WithMaxInterval caps the maximum wait duration between retries (default: 30s).
// Once the exponentially computed cap exceeds maxInterval, every subsequent
// retry waits at most maxInterval (subject to jitter).  This prevents
// unbounded growth of wait times for operations with many retries.
//
// Constraint: d > 0 and d >= initialInterval (the latter checked by Do after
// all options are applied).
func WithMaxInterval(d time.Duration) Option {
	return func(c *config) error {
		if d <= 0 {
			return fmt.Errorf("backoff: WithMaxInterval: d must be > 0, got %v", d)
		}
		c.maxInterval = d
		return nil
	}
}

// WithMultiplier sets the exponential growth factor applied to the interval cap
// after each attempt (default: 2.0).
//
// Examples:
//   - 2.0: intervals double each retry  → 100ms, 200ms, 400ms, 800ms, …
//   - 1.5: intervals grow by 50%        → 100ms, 150ms, 225ms, 337ms, …
//   - 1.0: constant interval (no growth) → 100ms, 100ms, 100ms, …
//
// Constraint: m >= 1.0.  Values below 1.0 would shrink the interval on each
// retry, which contradicts the purpose of exponential backoff.
func WithMultiplier(m float64) Option {
	return func(c *config) error {
		if m < 1.0 {
			return fmt.Errorf("backoff: WithMultiplier: m must be >= 1.0, got %v", m)
		}
		c.multiplier = m
		return nil
	}
}

// WithJitter controls the degree of randomness added to computed wait times
// (default: 1.0, Full Jitter).
//
// The sleep duration for retry i is:
//
//	sleep = cap_i×(1−j) + rand[0,cap_i)×j
//
// Where:
//   - j = 1.0 (Full Jitter):     sleep is uniformly random in [0, cap_i].
//     Recommended for distributed systems to prevent thundering herds.
//   - j = 0.0 (No Jitter):       sleep equals cap_i exactly.
//     Useful in tests or when a deterministic schedule is required.
//   - 0.0 < j < 1.0 (Partial):   sleep is interpolated between the two
//     extremes; cap_i×(1−j) is the deterministic floor.
//
// Constraint: j must be in [0.0, 1.0].
func WithJitter(j float64) Option {
	return func(c *config) error {
		if j < 0.0 || j > 1.0 {
			return fmt.Errorf("backoff: WithJitter: j must be in [0.0, 1.0], got %v", j)
		}
		c.jitter = j
		return nil
	}
}

// WithRetryIf sets a predicate that determines whether a given error should
// trigger a retry (default: all errors are retried).
//
// Return false from fn to abort retrying immediately and surface the error to
// the caller.  Typical use cases:
//   - HTTP: only retry on 5xx status codes; surface 4xx immediately.
//   - gRPC: retry on Unavailable/DeadlineExceeded; surface InvalidArgument.
//   - DB:   retry on connection errors; surface constraint violations.
//
// fn receives the error returned by the operation.  It is never called with a
// nil error because Do returns nil immediately on success.
//
// Constraint: fn must not be nil.
func WithRetryIf(fn func(error) bool) Option {
	return func(c *config) error {
		if fn == nil {
			return fmt.Errorf("backoff: WithRetryIf: fn must not be nil")
		}
		c.retryIf = fn
		return nil
	}
}

// WithOnRetry registers a hook that is called before each retry attempt.
//
// attempt is 1-indexed: attempt=1 indicates that fn has failed once and the
// first retry is about to be executed.  The hook is NOT called after the final
// failure (when the retry budget is exhausted or WithRetryIf returns false),
// so the number of hook invocations equals the number of retries that actually
// occur, not the total number of failures.
//
// Typical uses:
//   - Logging: record which attempt failed and why.
//   - Metrics: increment a retry counter for observability.
//   - Tracing: annotate a span with retry information.
//
// The hook is called synchronously in the same goroutine as Do; it should not
// block for a significant amount of time.
//
// Constraint: fn must not be nil.
func WithOnRetry(fn func(attempt int, err error)) Option {
	return func(c *config) error {
		if fn == nil {
			return fmt.Errorf("backoff: WithOnRetry: fn must not be nil")
		}
		c.onRetry = fn
		return nil
	}
}

// Do calls fn and retries it on error using exponential backoff with jitter.
//
// # Termination Conditions
//
// Do stops and returns when the first of the following conditions is met:
//
//  1. fn returns nil — Do returns nil immediately (success).
//  2. ctx is cancelled or its deadline is exceeded — Do returns ctx.Err().
//     This is checked both before each sleep and after each fn invocation, so
//     a context cancelled inside fn is detected promptly even when fn does not
//     propagate the context error itself.
//  3. WithRetryIf(fn) returns false for the error — Do returns the error
//     returned by fn without further retries.
//  4. The retry budget is exhausted (fn has been called maxRetries+1 times in
//     total) — Do returns the last non-nil error from fn.
//
// # Validation
//
// Before invoking fn, Do applies all options in order.  If any option returns
// an error (e.g. WithMaxRetries(-1)), or if the resolved configuration fails
// the cross-field constraint maxInterval >= initialInterval, Do returns that
// error immediately without calling fn.
//
// # Concurrency
//
// Do is safe to call concurrently from multiple goroutines.  Each call
// maintains its own state and timer; no shared mutable state is accessed.
func Do(ctx context.Context, fn func(ctx context.Context) error, opts ...Option) error {
	c := defaultConfig()
	for _, opt := range opts {
		if err := opt(&c); err != nil {
			return err
		}
	}
	if c.maxInterval < c.initialInterval {
		return fmt.Errorf("backoff: maxInterval (%v) must be >= initialInterval (%v)", c.maxInterval, c.initialInterval)
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
//	sleep = cap×(1−jitter) + rand[0,cap)×jitter
//
// With jitter=1.0 (Full Jitter): sleep is uniform in [0, cap].
// With jitter=0.0 (No Jitter):   sleep equals cap deterministically.
func sleepDuration(c config, attempt int) time.Duration {
	cap := min(
		float64(c.maxInterval),
		float64(c.initialInterval)*math.Pow(c.multiplier, float64(attempt-1)),
	)
	sleep := cap*(1-c.jitter) + rand.Float64()*cap*c.jitter
	return time.Duration(sleep)
}
