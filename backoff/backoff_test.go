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

func TestDo_SuccessOnFirstAttempt(t *testing.T) {
	calls := 0
	err := backoff.Do(context.Background(), func(_ context.Context) error {
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

func TestDo_SuccessAfterRetries(t *testing.T) {
	calls := 0
	err := backoff.Do(context.Background(), func(_ context.Context) error {
		calls++
		if calls < 3 {
			return errTransient
		}
		return nil
	},
		backoff.WithMaxRetries(5),
		backoff.WithInitialInterval(time.Millisecond),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if calls != 3 {
		t.Fatalf("expected 3 calls, got %d", calls)
	}
}

func TestDo_MaxRetriesExhausted(t *testing.T) {
	const maxRetries = 3
	calls := 0
	err := backoff.Do(context.Background(), func(_ context.Context) error {
		calls++
		return errTransient
	},
		backoff.WithMaxRetries(maxRetries),
		backoff.WithInitialInterval(time.Millisecond),
	)
	if !errors.Is(err, errTransient) {
		t.Fatalf("expected errTransient, got %v", err)
	}
	if calls != maxRetries+1 {
		t.Fatalf("expected %d calls, got %d", maxRetries+1, calls)
	}
}

func TestDo_ZeroRetries(t *testing.T) {
	calls := 0
	err := backoff.Do(context.Background(), func(_ context.Context) error {
		calls++
		return errTransient
	},
		backoff.WithMaxRetries(0),
	)
	if !errors.Is(err, errTransient) {
		t.Fatalf("expected errTransient, got %v", err)
	}
	if calls != 1 {
		t.Fatalf("expected exactly 1 call, got %d", calls)
	}
}

func TestDo_ContextCanceledDuringSleep(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	calls := 0
	err := backoff.Do(ctx, func(_ context.Context) error {
		calls++
		cancel() // cancel after first failure so the sleep select fires ctx.Done
		return errTransient
	},
		backoff.WithMaxRetries(5),
		backoff.WithInitialInterval(time.Hour), // long interval ensures sleep is interrupted
		backoff.WithMaxInterval(2*time.Hour),
	)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
	if calls != 1 {
		t.Fatalf("expected 1 call before context canceled, got %d", calls)
	}
}

func TestDo_ContextAlreadyCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	calls := 0
	err := backoff.Do(ctx, func(_ context.Context) error {
		calls++
		return errTransient
	},
		backoff.WithMaxRetries(3),
		backoff.WithInitialInterval(time.Millisecond),
	)
	// fn is called once on attempt=0 (no sleep before first attempt),
	// then context cancel is detected before the sleep for attempt=1.
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}

func TestDo_ContextCanceledInsideFn(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	calls := 0
	err := backoff.Do(ctx, func(ctx context.Context) error {
		calls++
		cancel()
		return ctx.Err() // fn propagates context cancellation
	},
		backoff.WithMaxRetries(5),
		backoff.WithInitialInterval(time.Millisecond),
	)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
	if calls != 1 {
		t.Fatalf("expected 1 call, got %d", calls)
	}
}

func TestDo_NonRetryableError(t *testing.T) {
	calls := 0
	err := backoff.Do(context.Background(), func(_ context.Context) error {
		calls++
		return errPermanent
	},
		backoff.WithMaxRetries(10),
		backoff.WithInitialInterval(time.Millisecond),
		backoff.WithRetryIf(func(err error) bool {
			return !errors.Is(err, errPermanent)
		}),
	)
	if !errors.Is(err, errPermanent) {
		t.Fatalf("expected errPermanent, got %v", err)
	}
	if calls != 1 {
		t.Fatalf("expected 1 call (no retry on permanent error), got %d", calls)
	}
}

func TestDo_RetryIfSelectiveRetry(t *testing.T) {
	calls := 0
	err := backoff.Do(context.Background(), func(_ context.Context) error {
		calls++
		if calls == 1 {
			return errTransient // retried
		}
		return errPermanent // not retried
	},
		backoff.WithMaxRetries(10),
		backoff.WithInitialInterval(time.Millisecond),
		backoff.WithRetryIf(func(err error) bool {
			return errors.Is(err, errTransient)
		}),
	)
	if !errors.Is(err, errPermanent) {
		t.Fatalf("expected errPermanent, got %v", err)
	}
	if calls != 2 {
		t.Fatalf("expected 2 calls, got %d", calls)
	}
}

func TestDo_OnRetryCallback(t *testing.T) {
	const maxRetries = 3
	var retryAttempts []int
	var retryErrs []error

	calls := 0
	_ = backoff.Do(context.Background(), func(_ context.Context) error {
		calls++
		return errTransient
	},
		backoff.WithMaxRetries(maxRetries),
		backoff.WithInitialInterval(time.Millisecond),
		backoff.WithOnRetry(func(attempt int, err error) {
			retryAttempts = append(retryAttempts, attempt)
			retryErrs = append(retryErrs, err)
		}),
	)

	// OnRetry is called before each retry, not after the last failure.
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

func TestDo_OnRetryNotCalledOnSuccess(t *testing.T) {
	onRetryCalled := false
	_ = backoff.Do(context.Background(), func(_ context.Context) error {
		return nil
	},
		backoff.WithOnRetry(func(_ int, _ error) {
			onRetryCalled = true
		}),
	)
	if onRetryCalled {
		t.Fatal("OnRetry should not be called when fn succeeds on first attempt")
	}
}

func TestDo_OnRetryNotCalledOnLastFailure(t *testing.T) {
	const maxRetries = 2
	var onRetryAttempts []int

	_ = backoff.Do(context.Background(), func(_ context.Context) error {
		return errTransient
	},
		backoff.WithMaxRetries(maxRetries),
		backoff.WithInitialInterval(time.Millisecond),
		backoff.WithOnRetry(func(attempt int, _ error) {
			onRetryAttempts = append(onRetryAttempts, attempt)
		}),
	)

	// Only called before retries 1 and 2, not after the final (3rd) failure.
	if len(onRetryAttempts) != maxRetries {
		t.Fatalf("expected %d OnRetry calls, got %d", maxRetries, len(onRetryAttempts))
	}
}

func TestDo_DeadlineExceeded(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	err := backoff.Do(ctx, func(_ context.Context) error {
		return errTransient
	},
		backoff.WithMaxRetries(100),
		backoff.WithInitialInterval(5*time.Millisecond),
		backoff.WithJitter(0), // deterministic so sleep = 5ms
	)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected DeadlineExceeded, got %v", err)
	}
}

// TestSleepDuration verifies the backoff interval grows correctly.
func TestDo_BackoffIntervalsGrow(t *testing.T) {
	const initialInterval = 10 * time.Millisecond
	const multiplier = 2.0
	const maxInterval = 1 * time.Second

	var sleepTimes []time.Duration
	prevTime := time.Now()

	calls := 0
	backoff.Do(context.Background(), func(_ context.Context) error {
		now := time.Now()
		if calls > 0 {
			sleepTimes = append(sleepTimes, now.Sub(prevTime))
		}
		prevTime = now
		calls++
		return errTransient
	},
		backoff.WithMaxRetries(3),
		backoff.WithInitialInterval(initialInterval),
		backoff.WithMaxInterval(maxInterval),
		backoff.WithMultiplier(multiplier),
		backoff.WithJitter(0), // no jitter: deterministic
	)

	// With no jitter: sleep[i] = initialInterval * multiplier^i
	// sleep[0] ≈ 10ms, sleep[1] ≈ 20ms, sleep[2] ≈ 40ms
	expected := []time.Duration{10 * time.Millisecond, 20 * time.Millisecond, 40 * time.Millisecond}
	for i, got := range sleepTimes {
		want := expected[i]
		// Allow 50% tolerance for scheduler jitter in CI.
		if got < want/2 || got > want*3 {
			t.Errorf("sleep[%d] = %v, want ~%v", i, got, want)
		}
	}
}

// ── Option validation ───────────────────────────────────────────────────────

func TestWithMaxRetries_Negative(t *testing.T) {
	err := backoff.Do(context.Background(), func(_ context.Context) error { return nil },
		backoff.WithMaxRetries(-1),
	)
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
}

func TestWithInitialInterval_Zero(t *testing.T) {
	err := backoff.Do(context.Background(), func(_ context.Context) error { return nil },
		backoff.WithInitialInterval(0),
	)
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
}

func TestWithInitialInterval_Negative(t *testing.T) {
	err := backoff.Do(context.Background(), func(_ context.Context) error { return nil },
		backoff.WithInitialInterval(-time.Millisecond),
	)
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
}

func TestWithMaxInterval_Zero(t *testing.T) {
	err := backoff.Do(context.Background(), func(_ context.Context) error { return nil },
		backoff.WithMaxInterval(0),
	)
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
}

func TestWithMaxInterval_Negative(t *testing.T) {
	err := backoff.Do(context.Background(), func(_ context.Context) error { return nil },
		backoff.WithMaxInterval(-time.Second),
	)
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
}

func TestWithMultiplier_BelowOne(t *testing.T) {
	err := backoff.Do(context.Background(), func(_ context.Context) error { return nil },
		backoff.WithMultiplier(0.9),
	)
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
}

func TestWithMultiplier_Zero(t *testing.T) {
	err := backoff.Do(context.Background(), func(_ context.Context) error { return nil },
		backoff.WithMultiplier(0),
	)
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
}

func TestWithJitter_AboveOne(t *testing.T) {
	err := backoff.Do(context.Background(), func(_ context.Context) error { return nil },
		backoff.WithJitter(1.1),
	)
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
}

func TestWithJitter_Negative(t *testing.T) {
	err := backoff.Do(context.Background(), func(_ context.Context) error { return nil },
		backoff.WithJitter(-0.1),
	)
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
}

func TestWithRetryIf_Nil(t *testing.T) {
	err := backoff.Do(context.Background(), func(_ context.Context) error { return nil },
		backoff.WithRetryIf(nil),
	)
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
}

func TestWithOnRetry_Nil(t *testing.T) {
	err := backoff.Do(context.Background(), func(_ context.Context) error { return nil },
		backoff.WithOnRetry(nil),
	)
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
}

// TestDo_MaxIntervalLessThanInitialInterval verifies the cross-field constraint
// that maxInterval must be >= initialInterval.
func TestDo_MaxIntervalLessThanInitialInterval(t *testing.T) {
	err := backoff.Do(context.Background(), func(_ context.Context) error { return nil },
		backoff.WithInitialInterval(10*time.Second),
		backoff.WithMaxInterval(1*time.Second),
	)
	if err == nil {
		t.Fatal("expected cross-field validation error, got nil")
	}
}

// TestDo_ValidationErrorDoesNotCallFn verifies that fn is never invoked when
// option validation fails.
func TestDo_ValidationErrorDoesNotCallFn(t *testing.T) {
	called := false
	_ = backoff.Do(context.Background(), func(_ context.Context) error {
		called = true
		return nil
	}, backoff.WithMaxRetries(-1))
	if called {
		t.Fatal("fn should not be called when option validation fails")
	}
}

// ── Benchmark ───────────────────────────────────────────────────────────────

func BenchmarkDo(b *testing.B) {	ctx := context.Background()
	b.ResetTimer()
	b.ReportAllocs()
	for range b.N {
		backoff.Do(ctx, func(_ context.Context) error { return nil })
	}
}
