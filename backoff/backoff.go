// Package backoff implements Exponential Backoff with Jitter for retrying
// operations that may fail transiently, such as HTTP requests, gRPC calls, and
// database queries.
//
// # Overview
//
// Two entry points are provided, mirroring the design of sourcegraph/conc:
//
//   - [New] — retry an operation that returns only an error.
//   - [NewWithResult] — retry an operation that returns a typed value alongside
//     an error; the typed result is forwarded to the caller on success.
//
// Both return a configurable struct whose With* methods build up the retry
// policy via method chaining.  Configuration methods panic immediately if they
// receive an out-of-range argument — misconfiguration is a programming error,
// not a runtime condition.
//
// # Algorithm
//
// The default strategy is Full Jitter, as recommended by the AWS architecture
// blog (https://aws.amazon.com/blogs/architecture/exponential-backoff-and-jitter/).
// Full Jitter randomises retry timing across clients, preventing the
// thundering-herd effect that arises when many callers retry in lockstep.
//
// The sleep duration before retry i (1-indexed) is:
//
//	cap_i = min(maxInterval, initialInterval × multiplier^(i-1))
//	sleep = cap_i×(1−jitter) + rand[0, cap_i)×jitter
//
// With jitter=1.0 (default), sleep is uniform in [0, cap_i].
// With jitter=0.0, sleep equals cap_i deterministically.
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
// # Basic Usage — error only
//
//	err := backoff.New().
//	    WithMaxRetries(5).
//	    WithInitialInterval(200 * time.Millisecond).
//	    Do(ctx, func(ctx context.Context) error {
//	        return client.Call(ctx, req)
//	    })
//
// # Basic Usage — typed result
//
//	result, err := backoff.NewWithResult[*MyResponse]().
//	    WithMaxRetries(5).
//	    WithRetryIf(func(err error) bool {
//	        var httpErr *HTTPError
//	        return !errors.As(err, &httpErr) || httpErr.StatusCode >= 500
//	    }).
//	    Do(ctx, func(ctx context.Context) (*MyResponse, error) {
//	        return client.GetData(ctx, req)
//	    })
//
// # Reusable Presets with Configurator (Go 1.26)
//
// Go 1.26 lifted the restriction on self-referential type constraints, making
// it possible to define [Configurator] — an interface whose With* methods
// return the concrete type C.  Both [*Retryer] and [*ResultRetryer][T] satisfy
// Configurator, so a single generic preset function applies to both:
//
//	func GRPCPolicy[C backoff.Configurator[C]](c C) C {
//	    return c.
//	        WithMaxRetries(3).
//	        WithInitialInterval(100 * time.Millisecond).
//	        WithMaxInterval(5 * time.Second).
//	        WithRetryIf(isGRPCRetryable)
//	}
//
//	// Works with both retryer types without duplication:
//	err := GRPCPolicy(backoff.New()).Do(ctx, fn)
//	result, err := GRPCPolicy(backoff.NewWithResult[*pb.Reply]()).Do(ctx, fn)
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

// config holds the fully resolved backoff parameters after all With* methods
// have been applied.
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

// validate checks cross-field constraints that cannot be enforced by
// individual With* methods in isolation.
func (c config) validate() error {
	if c.maxInterval < c.initialInterval {
		return fmt.Errorf("backoff: maxInterval (%v) must be >= initialInterval (%v)",
			c.maxInterval, c.initialInterval)
	}
	return nil
}

// ── Configurator ──────────────────────────────────────────────────────────────

// Configurator is a self-referential generic interface, enabled by the Go 1.26
// relaxation of restrictions on type parameters that refer to the type being
// constrained.  It is satisfied by both [*Retryer] and [*ResultRetryer][T].
//
// The self-reference `C Configurator[C]` guarantees that every With* method
// returns the *same concrete type* C, so a generic preset function preserves
// the exact retryer type through the chain:
//
//	type Configurator[C Configurator[C]] interface {
//	    WithMaxRetries(n int) C
//	    ...
//	}
//
// This lets users write reusable policy functions once and apply them to any
// retryer type:
//
//	func HTTPPolicy[C backoff.Configurator[C]](c C) C {
//	    return c.
//	        WithMaxRetries(4).
//	        WithInitialInterval(200 * time.Millisecond).
//	        WithMaxInterval(10 * time.Second).
//	        WithRetryIf(func(err error) bool {
//	            var e *HTTPError
//	            return !errors.As(err, &e) || e.StatusCode >= 500
//	        })
//	}
//
//	// Apply the same preset to both retryer types:
//	err          := HTTPPolicy(backoff.New()).Do(ctx, fn)
//	result, err  := HTTPPolicy(backoff.NewWithResult[*Response]()).Do(ctx, fn)
type Configurator[C Configurator[C]] interface {
	WithMaxRetries(n int) C
	WithInitialInterval(d time.Duration) C
	WithMaxInterval(d time.Duration) C
	WithMultiplier(m float64) C
	WithJitter(j float64) C
	WithRetryIf(fn func(error) bool) C
	WithOnRetry(fn func(attempt int, err error)) C
}

// Compile-time assertions: verify that both concrete types implement Configurator.
var (
	_ Configurator[*Retryer]              = (*Retryer)(nil)
	_ Configurator[*ResultRetryer[any]]   = (*ResultRetryer[any])(nil)
)

// ── Retryer ──────────────────────────────────────────────────────────────────

// Retryer retries an operation that returns only an error.
// Create one with [New] and configure it using the With* methods.
//
// The configuration methods (With*) panic if called with invalid arguments.
// Do returns an error if the cross-field constraint maxInterval >= initialInterval
// is violated after all configuration is applied.
//
// A zero Retryer is not valid; always use [New].
type Retryer struct {
	cfg config
}

// New returns a new [Retryer] with default configuration.
//
// Use method chaining to customise the retry policy:
//
//	err := backoff.New().
//	    WithMaxRetries(5).
//	    WithInitialInterval(200 * time.Millisecond).
//	    Do(ctx, func(ctx context.Context) error {
//	        return client.Call(ctx, req)
//	    })
func New() *Retryer {
	return &Retryer{cfg: defaultConfig()}
}

// WithMaxRetries sets the maximum number of retries after the initial attempt
// (default: 3).  The operation is called at most n+1 times in total.
// Pass 0 to run the operation exactly once with no retries.
//
// Panics if n < 0.
func (r *Retryer) WithMaxRetries(n int) *Retryer {
	if n < 0 {
		panic(fmt.Sprintf("backoff: WithMaxRetries: n must be >= 0, got %d", n))
	}
	r.cfg.maxRetries = n
	return r
}

// WithInitialInterval sets the base wait duration before the first retry
// (default: 100ms).  The cap for retry i grows as:
//
//	cap_i = min(maxInterval, initialInterval × multiplier^(i-1))
//
// Panics if d <= 0.
func (r *Retryer) WithInitialInterval(d time.Duration) *Retryer {
	if d <= 0 {
		panic(fmt.Sprintf("backoff: WithInitialInterval: d must be > 0, got %v", d))
	}
	r.cfg.initialInterval = d
	return r
}

// WithMaxInterval caps the maximum wait duration between retries (default: 30s).
// Once the exponentially computed cap exceeds maxInterval, subsequent retries
// wait at most maxInterval (subject to jitter).
//
// The constraint maxInterval >= initialInterval is enforced by [Retryer.Do],
// not here, because both values may be set in any order.
//
// Panics if d <= 0.
func (r *Retryer) WithMaxInterval(d time.Duration) *Retryer {
	if d <= 0 {
		panic(fmt.Sprintf("backoff: WithMaxInterval: d must be > 0, got %v", d))
	}
	r.cfg.maxInterval = d
	return r
}

// WithMultiplier sets the exponential growth factor applied to the interval cap
// after each attempt (default: 2.0).
//
//	2.0 → intervals double each retry:    100ms, 200ms, 400ms, 800ms, …
//	1.5 → intervals grow by 50% each:     100ms, 150ms, 225ms, 337ms, …
//	1.0 → constant interval (no growth):  100ms, 100ms, 100ms, …
//
// Panics if m < 1.0.
func (r *Retryer) WithMultiplier(m float64) *Retryer {
	if m < 1.0 {
		panic(fmt.Sprintf("backoff: WithMultiplier: m must be >= 1.0, got %v", m))
	}
	r.cfg.multiplier = m
	return r
}

// WithJitter controls the degree of randomness added to wait times (default: 1.0).
//
//	sleep = cap_i×(1−j) + rand[0, cap_i)×j
//
//   - j = 1.0 (Full Jitter, default): sleep is uniform in [0, cap_i].
//     Recommended for distributed systems to avoid thundering herds.
//   - j = 0.0 (No Jitter): sleep equals cap_i deterministically.
//     Useful in tests or when a predictable schedule is required.
//   - 0 < j < 1 (Partial Jitter): cap_i×(1−j) is the deterministic floor.
//
// Panics if j is outside [0.0, 1.0].
func (r *Retryer) WithJitter(j float64) *Retryer {
	if j < 0.0 || j > 1.0 {
		panic(fmt.Sprintf("backoff: WithJitter: j must be in [0.0, 1.0], got %v", j))
	}
	r.cfg.jitter = j
	return r
}

// WithRetryIf sets a predicate that determines whether a given error warrants a
// retry (default: all errors are retried).  Return false to surface the error
// immediately without further retries.
//
// Typical patterns:
//   - HTTP:  retry on 5xx; return false for 4xx.
//   - gRPC:  retry on Unavailable/DeadlineExceeded; return false for InvalidArgument.
//   - DB:    retry on connection errors; return false for constraint violations.
//
// Panics if fn is nil.
func (r *Retryer) WithRetryIf(fn func(error) bool) *Retryer {
	if fn == nil {
		panic("backoff: WithRetryIf: fn must not be nil")
	}
	r.cfg.retryIf = fn
	return r
}

// WithOnRetry registers a hook invoked before each retry attempt.
//
// attempt is 1-indexed: attempt=1 means fn has failed once and the first retry
// is about to start.  The hook is NOT called after the final failure, so the
// number of invocations equals the number of retries that actually occur.
//
// Typical uses: logging, metrics, distributed tracing.  The hook runs
// synchronously and should not block for a significant amount of time.
//
// Panics if fn is nil.
func (r *Retryer) WithOnRetry(fn func(attempt int, err error)) *Retryer {
	if fn == nil {
		panic("backoff: WithOnRetry: fn must not be nil")
	}
	r.cfg.onRetry = fn
	return r
}

// Do calls fn and retries it on error according to the configured policy.
//
// Retrying stops when the first of the following conditions is met:
//  1. fn returns nil — Do returns nil (success).
//  2. ctx is done — Do returns ctx.Err().  Checked both before each sleep and
//     after each fn call, so cancellation inside fn is detected promptly.
//  3. WithRetryIf returns false — Do returns the error immediately.
//  4. The retry budget is exhausted — Do returns the last error from fn.
//
// Do returns an error without calling fn if the cross-field constraint
// maxInterval >= initialInterval is violated.
func (r *Retryer) Do(ctx context.Context, fn func(ctx context.Context) error) error {
	if err := r.cfg.validate(); err != nil {
		return err
	}
	_, err := execute[struct{}](ctx, r.cfg, func(ctx context.Context) (struct{}, error) {
		return struct{}{}, fn(ctx)
	})
	return err
}

// ── ResultRetryer ─────────────────────────────────────────────────────────────

// ResultRetryer retries an operation that returns a typed result (T, error).
// On success, the typed value returned by fn is forwarded to the caller.
// Create one with [NewWithResult] and configure it using the With* methods.
//
// ResultRetryer has the same configuration API as [Retryer]; each With* method
// returns *ResultRetryer[T] to preserve the generic type through method chains.
//
// A zero ResultRetryer is not valid; always use [NewWithResult].
type ResultRetryer[T any] struct {
	r Retryer
}

// NewWithResult returns a new [ResultRetryer][T] with default configuration.
//
// Use method chaining to customise the retry policy:
//
//	result, err := backoff.NewWithResult[*MyResponse]().
//	    WithMaxRetries(5).
//	    WithRetryIf(func(err error) bool {
//	        var httpErr *HTTPError
//	        return !errors.As(err, &httpErr) || httpErr.StatusCode >= 500
//	    }).
//	    Do(ctx, func(ctx context.Context) (*MyResponse, error) {
//	        return client.GetData(ctx, req)
//	    })
func NewWithResult[T any]() *ResultRetryer[T] {
	return &ResultRetryer[T]{r: Retryer{cfg: defaultConfig()}}
}

// WithMaxRetries sets the maximum number of retries (default: 3).
// Panics if n < 0.
func (r *ResultRetryer[T]) WithMaxRetries(n int) *ResultRetryer[T] {
	r.r.WithMaxRetries(n)
	return r
}

// WithInitialInterval sets the base wait duration before the first retry
// (default: 100ms). Panics if d <= 0.
func (r *ResultRetryer[T]) WithInitialInterval(d time.Duration) *ResultRetryer[T] {
	r.r.WithInitialInterval(d)
	return r
}

// WithMaxInterval caps the maximum wait duration between retries (default: 30s).
// Panics if d <= 0.
func (r *ResultRetryer[T]) WithMaxInterval(d time.Duration) *ResultRetryer[T] {
	r.r.WithMaxInterval(d)
	return r
}

// WithMultiplier sets the exponential growth factor (default: 2.0).
// Panics if m < 1.0.
func (r *ResultRetryer[T]) WithMultiplier(m float64) *ResultRetryer[T] {
	r.r.WithMultiplier(m)
	return r
}

// WithJitter controls the randomness of wait times (default: 1.0, Full Jitter).
// Panics if j is outside [0.0, 1.0].
func (r *ResultRetryer[T]) WithJitter(j float64) *ResultRetryer[T] {
	r.r.WithJitter(j)
	return r
}

// WithRetryIf sets a predicate that determines whether an error warrants a retry
// (default: all errors retried). Panics if fn is nil.
func (r *ResultRetryer[T]) WithRetryIf(fn func(error) bool) *ResultRetryer[T] {
	r.r.WithRetryIf(fn)
	return r
}

// WithOnRetry registers a hook invoked before each retry attempt.
// Panics if fn is nil.
func (r *ResultRetryer[T]) WithOnRetry(fn func(attempt int, err error)) *ResultRetryer[T] {
	r.r.WithOnRetry(fn)
	return r
}

// Do calls fn and retries it on error, returning the typed result from fn on
// success.  On any terminal failure the zero value of T is returned alongside
// the error.
//
// Termination conditions and validation behaviour are identical to [Retryer.Do].
func (r *ResultRetryer[T]) Do(ctx context.Context, fn func(ctx context.Context) (T, error)) (T, error) {
	if err := r.r.cfg.validate(); err != nil {
		var zero T
		return zero, err
	}
	return execute[T](ctx, r.r.cfg, fn)
}

// ── shared retry loop ─────────────────────────────────────────────────────────

// execute is the shared retry loop used by both Retryer.Do and ResultRetryer.Do.
// It is generic over T so that the zero value can be returned on failure without
// allocations for the non-result path (T = struct{}).
func execute[T any](ctx context.Context, cfg config, fn func(context.Context) (T, error)) (T, error) {
	var (
		zero    T
		lastErr error
	)
	for attempt := range cfg.maxRetries + 1 {
		if attempt > 0 {
			timer := time.NewTimer(sleepDuration(cfg, attempt))
			select {
			case <-ctx.Done():
				timer.Stop()
				return zero, ctx.Err()
			case <-timer.C:
			}
		}

		result, err := fn(ctx)
		if err == nil {
			return result, nil
		}
		lastErr = err

		// Respect context cancellation even when fn does not propagate it.
		if ctx.Err() != nil {
			return zero, ctx.Err()
		}

		// Non-retryable error: surface immediately without further retries.
		if !cfg.retryIf(lastErr) {
			return zero, lastErr
		}

		// Notify the caller before every retry, but not after the final failure.
		if cfg.onRetry != nil && attempt < cfg.maxRetries {
			cfg.onRetry(attempt+1, lastErr)
		}
	}
	return zero, lastErr
}

// sleepDuration returns the wait time before retry attempt (1-indexed).
//
//	cap   = min(maxInterval, initialInterval × multiplier^(attempt-1))
//	sleep = cap×(1−jitter) + rand[0, cap)×jitter
func sleepDuration(cfg config, attempt int) time.Duration {
	cap := min(
		float64(cfg.maxInterval),
		float64(cfg.initialInterval)*math.Pow(cfg.multiplier, float64(attempt-1)),
	)
	sleep := cap*(1-cfg.jitter) + rand.Float64()*cap*cfg.jitter
	return time.Duration(sleep)
}
