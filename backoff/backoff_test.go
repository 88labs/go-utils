package backoff_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/88labs/go-utils/backoff"
)

var errTransient = errors.New("transient error")
var errPermanent = errors.New("permanent error")

// mustPanic asserts that fn panics.  It is used to test invalid With* arguments.
func mustPanic(t *testing.T, fn func()) {
	t.Helper()
	defer func() {
		if recover() == nil {
			t.Error("expected panic, but none occurred")
		}
	}()
	fn()
}

// ── Retryer.Do ───────────────────────────────────────────────────────────────

func TestRetryer_SuccessOnFirstAttempt(t *testing.T) {
	calls := 0
	err := backoff.New().Do(context.Background(), func(_ context.Context) error {
		calls++
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if calls != 1 {
		t.Fatalf("expected 1 call, got %d", calls)
	}
}

func TestRetryer_SuccessAfterRetries(t *testing.T) {
	calls := 0
	err := backoff.New().
		WithMaxRetries(5).
		WithInitialInterval(time.Millisecond).
		Do(context.Background(), func(_ context.Context) error {
			calls++
			if calls < 3 {
				return errTransient
			}
			return nil
		})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if calls != 3 {
		t.Fatalf("expected 3 calls, got %d", calls)
	}
}

func TestRetryer_MaxRetriesExhausted(t *testing.T) {
	const maxRetries = 3
	calls := 0
	err := backoff.New().
		WithMaxRetries(maxRetries).
		WithInitialInterval(time.Millisecond).
		Do(context.Background(), func(_ context.Context) error {
			calls++
			return errTransient
		})
	if !errors.Is(err, errTransient) {
		t.Fatalf("expected errTransient, got %v", err)
	}
	if calls != maxRetries+1 {
		t.Fatalf("expected %d calls, got %d", maxRetries+1, calls)
	}
}

func TestRetryer_ZeroRetries(t *testing.T) {
	calls := 0
	err := backoff.New().
		WithMaxRetries(0).
		Do(context.Background(), func(_ context.Context) error {
			calls++
			return errTransient
		})
	if !errors.Is(err, errTransient) {
		t.Fatalf("expected errTransient, got %v", err)
	}
	if calls != 1 {
		t.Fatalf("expected exactly 1 call, got %d", calls)
	}
}

func TestRetryer_ContextCanceledDuringSleep(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	calls := 0
	err := backoff.New().
		WithMaxRetries(5).
		WithInitialInterval(time.Hour). // long interval — sleep will be interrupted
		WithMaxInterval(2 * time.Hour).
		Do(ctx, func(_ context.Context) error {
			calls++
			cancel()
			return errTransient
		})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
	if calls != 1 {
		t.Fatalf("expected 1 call before context canceled, got %d", calls)
	}
}

func TestRetryer_ContextAlreadyCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := backoff.New().
		WithMaxRetries(3).
		WithInitialInterval(time.Millisecond).
		Do(ctx, func(_ context.Context) error {
			return errTransient
		})
	// fn is called once (no sleep before attempt 0), then context cancellation
	// is detected before the sleep for attempt 1.
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}

func TestRetryer_ContextCanceledInsideFn(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	calls := 0
	err := backoff.New().
		WithMaxRetries(5).
		WithInitialInterval(time.Millisecond).
		Do(ctx, func(ctx context.Context) error {
			calls++
			cancel()
			return ctx.Err() // fn propagates the cancellation
		})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
	if calls != 1 {
		t.Fatalf("expected 1 call, got %d", calls)
	}
}

func TestRetryer_NonRetryableError(t *testing.T) {
	calls := 0
	err := backoff.New().
		WithMaxRetries(10).
		WithInitialInterval(time.Millisecond).
		WithRetryIf(func(err error) bool {
			return !errors.Is(err, errPermanent)
		}).
		Do(context.Background(), func(_ context.Context) error {
			calls++
			return errPermanent
		})
	if !errors.Is(err, errPermanent) {
		t.Fatalf("expected errPermanent, got %v", err)
	}
	if calls != 1 {
		t.Fatalf("expected 1 call (no retry on permanent error), got %d", calls)
	}
}

func TestRetryer_RetryIfSelectiveRetry(t *testing.T) {
	calls := 0
	err := backoff.New().
		WithMaxRetries(10).
		WithInitialInterval(time.Millisecond).
		WithRetryIf(func(err error) bool {
			return errors.Is(err, errTransient)
		}).
		Do(context.Background(), func(_ context.Context) error {
			calls++
			if calls == 1 {
				return errTransient // retried
			}
			return errPermanent // not retried
		})
	if !errors.Is(err, errPermanent) {
		t.Fatalf("expected errPermanent, got %v", err)
	}
	if calls != 2 {
		t.Fatalf("expected 2 calls, got %d", calls)
	}
}

func TestRetryer_OnRetryCallback(t *testing.T) {
	const maxRetries = 3
	var retryAttempts []int
	var retryErrs []error

	_ = backoff.New().
		WithMaxRetries(maxRetries).
		WithInitialInterval(time.Millisecond).
		WithOnRetry(func(attempt int, err error) {
			retryAttempts = append(retryAttempts, attempt)
			retryErrs = append(retryErrs, err)
		}).
		Do(context.Background(), func(_ context.Context) error {
			return errTransient
		})

	// OnRetry is called before each retry but not after the final failure.
	if len(retryAttempts) != maxRetries {
		t.Fatalf("expected %d OnRetry calls, got %d", maxRetries, len(retryAttempts))
	}
	for i, a := range retryAttempts {
		if a != i+1 {
			t.Errorf("retryAttempts[%d] = %d, want %d", i, a, i+1)
		}
	}
	for _, e := range retryErrs {
		if !errors.Is(e, errTransient) {
			t.Errorf("expected errTransient in OnRetry, got %v", e)
		}
	}
}

func TestRetryer_OnRetryNotCalledOnSuccess(t *testing.T) {
	onRetryCalled := false
	_ = backoff.New().
		WithOnRetry(func(_ int, _ error) { onRetryCalled = true }).
		Do(context.Background(), func(_ context.Context) error { return nil })
	if onRetryCalled {
		t.Fatal("OnRetry must not be called when fn succeeds on the first attempt")
	}
}

func TestRetryer_OnRetryNotCalledAfterFinalFailure(t *testing.T) {
	const maxRetries = 2
	var calls []int
	_ = backoff.New().
		WithMaxRetries(maxRetries).
		WithInitialInterval(time.Millisecond).
		WithOnRetry(func(attempt int, _ error) { calls = append(calls, attempt) }).
		Do(context.Background(), func(_ context.Context) error { return errTransient })

	// Called before retry 1 and retry 2, but not after the 3rd (final) failure.
	if len(calls) != maxRetries {
		t.Fatalf("expected %d OnRetry calls, got %d", maxRetries, len(calls))
	}
}

func TestRetryer_DeadlineExceeded(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	err := backoff.New().
		WithMaxRetries(100).
		WithInitialInterval(5 * time.Millisecond).
		WithJitter(0). // deterministic: sleep = exactly 5ms each time
		Do(ctx, func(_ context.Context) error { return errTransient })
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected DeadlineExceeded, got %v", err)
	}
}

// TestRetryer_BackoffIntervalsGrow verifies that sleep durations grow
// exponentially when jitter is disabled.
func TestRetryer_BackoffIntervalsGrow(t *testing.T) {
	const (
		initialInterval = 10 * time.Millisecond
		multiplier      = 2.0
		maxInterval     = 1 * time.Second
	)

	var sleepTimes []time.Duration
	prev := time.Now()
	calls := 0

	backoff.New().
		WithMaxRetries(3).
		WithInitialInterval(initialInterval).
		WithMaxInterval(maxInterval).
		WithMultiplier(multiplier).
		WithJitter(0). // no randomness: deterministic schedule
		Do(context.Background(), func(_ context.Context) error {
			now := time.Now()
			if calls > 0 {
				sleepTimes = append(sleepTimes, now.Sub(prev))
			}
			prev = now
			calls++
			return errTransient
		})

	// Expected sleeps (no jitter): 10ms, 20ms, 40ms.
	expected := []time.Duration{10 * time.Millisecond, 20 * time.Millisecond, 40 * time.Millisecond}
	for i, got := range sleepTimes {
		want := expected[i]
		// Allow 50% tolerance for scheduler jitter in CI environments.
		if got < want/2 || got > want*3 {
			t.Errorf("sleep[%d] = %v, want ~%v", i, got, want)
		}
	}
}

// ── ResultRetryer.Do ─────────────────────────────────────────────────────────

func TestResultRetryer_SuccessOnFirstAttempt(t *testing.T) {
	result, err := backoff.NewWithResult[int]().
		Do(context.Background(), func(_ context.Context) (int, error) {
			return 42, nil
		})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != 42 {
		t.Fatalf("expected result 42, got %d", result)
	}
}

func TestResultRetryer_SuccessAfterRetries(t *testing.T) {
	calls := 0
	result, err := backoff.NewWithResult[string]().
		WithMaxRetries(5).
		WithInitialInterval(time.Millisecond).
		Do(context.Background(), func(_ context.Context) (string, error) {
			calls++
			if calls < 3 {
				return "", errTransient
			}
			return "ok", nil
		})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "ok" {
		t.Fatalf("expected result 'ok', got %q", result)
	}
	if calls != 3 {
		t.Fatalf("expected 3 calls, got %d", calls)
	}
}

func TestResultRetryer_MaxRetriesExhausted_ReturnsZeroValue(t *testing.T) {
	result, err := backoff.NewWithResult[int]().
		WithMaxRetries(2).
		WithInitialInterval(time.Millisecond).
		Do(context.Background(), func(_ context.Context) (int, error) {
			return 99, errTransient
		})
	if !errors.Is(err, errTransient) {
		t.Fatalf("expected errTransient, got %v", err)
	}
	if result != 0 {
		t.Fatalf("expected zero value on failure, got %d", result)
	}
}

func TestResultRetryer_NonRetryableError_ReturnsZeroValue(t *testing.T) {
	result, err := backoff.NewWithResult[string]().
		WithRetryIf(func(err error) bool { return false }).
		Do(context.Background(), func(_ context.Context) (string, error) {
			return "partial", errPermanent
		})
	if !errors.Is(err, errPermanent) {
		t.Fatalf("expected errPermanent, got %v", err)
	}
	if result != "" {
		t.Fatalf("expected zero value on non-retryable error, got %q", result)
	}
}

func TestResultRetryer_ContextCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	calls := 0
	result, err := backoff.NewWithResult[int]().
		WithMaxRetries(5).
		WithInitialInterval(time.Hour).
		WithMaxInterval(2 * time.Hour).
		Do(ctx, func(_ context.Context) (int, error) {
			calls++
			cancel()
			return 0, errTransient
		})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
	if result != 0 {
		t.Fatalf("expected zero value on context cancel, got %d", result)
	}
	if calls != 1 {
		t.Fatalf("expected 1 call before context canceled, got %d", calls)
	}
}

func TestResultRetryer_MethodChainingPreservesGenericType(t *testing.T) {
	// Verify that all With* methods return *ResultRetryer[T] and support chaining.
	result, err := backoff.NewWithResult[bool]().
		WithMaxRetries(2).
		WithInitialInterval(time.Millisecond).
		WithMaxInterval(10 * time.Millisecond).
		WithMultiplier(1.5).
		WithJitter(0.5).
		WithRetryIf(func(err error) bool { return true }).
		WithOnRetry(func(_ int, _ error) {}).
		Do(context.Background(), func(_ context.Context) (bool, error) {
			return true, nil
		})
	if err != nil || !result {
		t.Fatalf("chained call failed: result=%v, err=%v", result, err)
	}
}

// ── Validation: With* panics ──────────────────────────────────────────────────

func TestWithMaxRetries_Negative(t *testing.T) {
	mustPanic(t, func() { backoff.New().WithMaxRetries(-1) })
}

func TestWithInitialInterval_Zero(t *testing.T) {
	mustPanic(t, func() { backoff.New().WithInitialInterval(0) })
}

func TestWithInitialInterval_Negative(t *testing.T) {
	mustPanic(t, func() { backoff.New().WithInitialInterval(-time.Millisecond) })
}

func TestWithMaxInterval_Zero(t *testing.T) {
	mustPanic(t, func() { backoff.New().WithMaxInterval(0) })
}

func TestWithMaxInterval_Negative(t *testing.T) {
	mustPanic(t, func() { backoff.New().WithMaxInterval(-time.Second) })
}

func TestWithMultiplier_BelowOne(t *testing.T) {
	mustPanic(t, func() { backoff.New().WithMultiplier(0.9) })
}

func TestWithMultiplier_Zero(t *testing.T) {
	mustPanic(t, func() { backoff.New().WithMultiplier(0) })
}

func TestWithJitter_AboveOne(t *testing.T) {
	mustPanic(t, func() { backoff.New().WithJitter(1.1) })
}

func TestWithJitter_Negative(t *testing.T) {
	mustPanic(t, func() { backoff.New().WithJitter(-0.1) })
}

func TestWithRetryIf_Nil(t *testing.T) {
	mustPanic(t, func() { backoff.New().WithRetryIf(nil) })
}

func TestWithOnRetry_Nil(t *testing.T) {
	mustPanic(t, func() { backoff.New().WithOnRetry(nil) })
}

// Validation panics propagate through ResultRetryer's delegating With* methods.
func TestResultRetryer_WithMaxRetries_Negative(t *testing.T) {
	mustPanic(t, func() { backoff.NewWithResult[int]().WithMaxRetries(-1) })
}

func TestResultRetryer_WithJitter_OutOfRange(t *testing.T) {
	mustPanic(t, func() { backoff.NewWithResult[string]().WithJitter(2.0) })
}

// ── Validation: Do cross-field error ─────────────────────────────────────────

// TestDo_MaxIntervalLessThanInitialInterval verifies that Do returns an error
// when the cross-field constraint maxInterval >= initialInterval is violated.
// This cannot be enforced by individual With* methods because both values may
// be set in any order.
func TestRetryer_MaxIntervalLessThanInitialInterval(t *testing.T) {
	called := false
	err := backoff.New().
		WithInitialInterval(10 * time.Second).
		WithMaxInterval(1 * time.Second).
		Do(context.Background(), func(_ context.Context) error {
			called = true
			return nil
		})
	if err == nil {
		t.Fatal("expected cross-field validation error, got nil")
	}
	if called {
		t.Fatal("fn must not be called when validation fails")
	}
}

func TestResultRetryer_MaxIntervalLessThanInitialInterval(t *testing.T) {
	called := false
	result, err := backoff.NewWithResult[int]().
		WithInitialInterval(10 * time.Second).
		WithMaxInterval(1 * time.Second).
		Do(context.Background(), func(_ context.Context) (int, error) {
			called = true
			return 42, nil
		})
	if err == nil {
		t.Fatal("expected cross-field validation error, got nil")
	}
	if result != 0 {
		t.Fatalf("expected zero value on validation error, got %d", result)
	}
	if called {
		t.Fatal("fn must not be called when validation fails")
	}
}

// ── Benchmark ────────────────────────────────────────────────────────────────

func BenchmarkRetryer_Do(b *testing.B) {
	r := backoff.New()
	ctx := context.Background()
	b.ResetTimer()
	b.ReportAllocs()
	for range b.N {
		r.Do(ctx, func(_ context.Context) error { return nil })
	}
}

func BenchmarkResultRetryer_Do(b *testing.B) {
	r := backoff.NewWithResult[int]()
	ctx := context.Background()
	b.ResetTimer()
	b.ReportAllocs()
	for range b.N {
		r.Do(ctx, func(_ context.Context) (int, error) { return 1, nil })
	}
}
