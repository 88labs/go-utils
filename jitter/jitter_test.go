package jitter_test

import (
	"errors"
	"testing"
	"time"

	"github.com/88labs/go-utils/jitter"
)

// TestApply_Deterministic verifies the edge cases and deterministic boundaries.
func TestApply_Deterministic(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		duration time.Duration
		factor   float64
		want     time.Duration
	}{
		{"Zero factor", 100 * time.Millisecond, 0.0, 100 * time.Millisecond},
		{"Negative factor", 50 * time.Millisecond, -0.5, 50 * time.Millisecond},
		{"Negative duration", -10 * time.Millisecond, 1.0, 0},
		{"Zero duration", 0, 0.5, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := jitter.Apply(tt.duration, tt.factor)
			if got != tt.want {
				t.Errorf("Apply(%v, %v) = %v; want exactly %v", tt.duration, tt.factor, got, tt.want)
			}
		})
	}
}

// TestApply_Bounds verifies that the applied jitter stays within the expected min/max boundaries.
func TestApply_Bounds(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		duration time.Duration
		factor   float64
		wantMin  time.Duration
	}{
		{"Partial Jitter 50%", 100 * time.Millisecond, 0.5, 50 * time.Millisecond},
		{"Partial Jitter 10%", 100 * time.Millisecond, 0.1, 90 * time.Millisecond},
		{"Full Jitter 100%", 100 * time.Millisecond, 1.0, 0},
		{"Factor above 1.0 is capped at 1.0", 100 * time.Millisecond, 1.5, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			const iterations = 1000
			for i := 0; i < iterations; i++ {
				got := jitter.Apply(tt.duration, tt.factor)
				if got < tt.wantMin || got > tt.duration {
					t.Errorf("Apply(%v, %v) returned %v on iteration %d; want between %v and %v",
						tt.duration, tt.factor, got, i, tt.wantMin, tt.duration)
				}
			}
		})
	}
}

// TestApply_Concurrent ensures that jitter.Apply is safe for concurrent use.
// It leverages t.Context() to gracefully manage the lifecycle of background goroutines.
func TestApply_Concurrent(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	const workers = 10
	errc := make(chan error, workers)

	for i := 0; i < workers; i++ {
		go func() {
			var err error
			for j := 0; j < 1000; j++ {
				if ctx.Err() != nil {
					err = ctx.Err()
					break
				}

				// Apply should not panic or cause race conditions
				_ = jitter.Apply(time.Second, 0.5)
			}
			errc <- err
		}()
	}

	// Verify all workers completed without unexpected errors.
	for i := 0; i < workers; i++ {
		if err := <-errc; err != nil && !errors.Is(err, ctx.Err()) {
			t.Errorf("worker failed with unexpected error: %v", err)
		}
	}
}
