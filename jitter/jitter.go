// Package jitter provides functions for adding randomized jitter to time durations.
// This is commonly used in retry mechanisms (like Exponential Backoff) to prevent
// thundering-herd problems by spreading out the execution times of concurrent clients.
package jitter

import (
	"math/rand/v2"
	"time"
)

// Apply applies a jitter factor to a given duration.
// The factor determines the degree of randomness added to the wait time.
//
// The applied formula is:
//
//		sleep = duration × (1 − factor) + rand[0, duration) × factor
//
//	  - factor = 1.0 (Full Jitter): result is uniform in [0, duration].
//	  - factor = 0.0 (No Jitter): result equals duration deterministically.
//	  - 0 < factor < 1 (Partial Jitter): duration × (1 − factor) acts as the deterministic floor.
//
// Edge cases and thresholds:
//   - If duration is less than or equal to 0 (including negative values), it returns 0 immediately.
//   - If factor is less than or equal to 0.0, duration is returned unmodified.
//   - If factor is greater than 1.0, it is capped at 1.0.
func Apply(duration time.Duration, factor float64) time.Duration {
	if duration <= 0 {
		return 0
	}
	if factor <= 0.0 {
		return duration
	}
	if factor > 1.0 {
		factor = 1.0
	}
	d := float64(duration)
	sleep := d*(1-factor) + rand.Float64()*d*factor
	return time.Duration(sleep)
}
