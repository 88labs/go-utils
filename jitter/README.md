# jitter

The `jitter` package provides functions for adding randomized jitter to time durations. It is completely independent and can be used in retry mechanisms, polling loops, or any other scenario where spreading out concurrent actions is necessary to prevent thundering-herd problems.

## Installation

```bash
go get github.com/88labs/go-utils/jitter
```

## Overview

In distributed systems, when multiple clients or processes retry a failed operation or poll a resource at the exact same time, they can overwhelm the system (the "thundering herd" problem). Adding randomness, or "jitter", to the wait time helps distribute these requests evenly over time.

This package provides a simple, dependency-free `Apply` function to add jitter to a `time.Duration` based on a configurable factor.

## Usage

### Basic Example

```go
package main

import (
	"fmt"
	"time"

	"github.com/88labs/go-utils/jitter"
)

func main() {
	duration := 1 * time.Second

	// Full Jitter: wait time is a uniform random value between 0 and 1s.
	sleepFull := jitter.Apply(duration, 1.0)
	fmt.Printf("Full Jitter: %v\n", sleepFull)

	// Partial Jitter: wait time is at least 500ms, plus up to 500ms of random jitter.
	sleepPartial := jitter.Apply(duration, 0.5)
	fmt.Printf("Partial Jitter: %v\n", sleepPartial)
}
```

## API Reference

### `Apply(duration time.Duration, factor float64) time.Duration`

`Apply` takes a base `duration` and a jitter `factor`, and returns a new randomized duration.

The applied formula is:

```
sleep = duration × (1 − factor) + rand[0, duration) × factor
```

#### Behavior based on `factor`:

*   **`factor = 1.0` (Full Jitter):**
    The resulting duration is a uniformly distributed random value in the range `[0, duration]`. This is the recommended approach for most exponential backoff implementations.
*   **`0 < factor < 1.0` (Partial Jitter):**
    A portion of the duration is deterministic, and the rest is random. For example, if `factor = 0.2`, the minimum wait time is `0.8 * duration`, and an additional random wait up to `0.2 * duration` is added. `duration × (1 − factor)` acts as the deterministic floor.

#### Edge Cases & Thresholds:

To prevent unintended behavior, the function safely handles edge cases deterministically:

*   **`duration <= 0` (Negative or Zero Duration):**
    If the specified duration is `0` or negative, the function **returns `0` immediately**.
*   **`factor <= 0.0` (Negative or Zero Factor):**
    If the factor is `0.0` or negative, no jitter is applied, and the function returns the original `duration` unmodified.
*   **`factor > 1.0`:**
    If the factor exceeds `1.0`, it is silently capped and treated as `1.0` (Full Jitter).

## Integration Example (Custom Polling)

You can easily integrate `jitter` into a simple polling loop without needing a full backoff library:

```go
func pollWithJitter(ctx context.Context, interval time.Duration, jitterFactor float64) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Calculate jittered wait time based on the interval
			sleepTime := jitter.Apply(interval, jitterFactor)
			
			// Sleep for the randomized duration before executing the task
			time.Sleep(sleepTime)
			
			// Perform task...
			fmt.Println("Polling...")
		}
	}
}
```

## Concurrency

The `jitter` package uses the standard library's `math/rand/v2` which is safe for concurrent use by multiple goroutines. You can call `jitter.Apply` simultaneously from as many workers as needed.
