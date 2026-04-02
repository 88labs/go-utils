# backoff

Package `backoff` provides a generic Exponential Backoff with Jitter implementation
for retrying operations that may fail transiently — HTTP requests, gRPC calls,
database queries, or any other fallible I/O.

The default strategy is **Full Jitter**, as recommended by the
[AWS Architecture Blog](https://aws.amazon.com/blogs/architecture/exponential-backoff-and-jitter/).
Full Jitter distributes retry timing uniformly at random across clients, which
prevents the thundering-herd effect that arises when many callers fail
simultaneously and then retry in lockstep.

## Algorithm

```
cap_i = min(maxInterval, initialInterval × multiplier^(i-1))
sleep = cap_i × (1 − jitter) + rand[0, cap_i) × jitter
```

| jitter | Behaviour |
|--------|-----------|
| `1.0` (Full Jitter, default) | sleep is uniformly random in `[0, cap_i]` |
| `0.0` (No Jitter) | sleep equals `cap_i` exactly — deterministic |
| `0 < j < 1` (Partial Jitter) | `cap_i×(1−j)` is the deterministic floor; randomness is scaled by `j` |

## Defaults

| Option | Default |
|--------|---------|
| `MaxRetries` | `3` (4 total attempts) |
| `InitialInterval` | `100ms` |
| `MaxInterval` | `30s` |
| `Multiplier` | `2.0` |
| `Jitter` | `1.0` (Full Jitter) |
| `RetryIf` | retry all errors |
| `OnRetry` | none |

## Usage

### Minimal

```go
err := backoff.Do(ctx, func(ctx context.Context) error {
    return client.Call(ctx, req)
})
```

### All options

```go
err := backoff.Do(ctx, func(ctx context.Context) error {
    return client.Call(ctx, req)
},
    backoff.WithMaxRetries(5),
    backoff.WithInitialInterval(200*time.Millisecond),
    backoff.WithMaxInterval(10*time.Second),
    backoff.WithMultiplier(1.5),
    backoff.WithJitter(1.0),
    backoff.WithRetryIf(func(err error) bool {
        // Only retry server errors; surface client errors immediately.
        var httpErr *HTTPError
        return !errors.As(err, &httpErr) || httpErr.StatusCode >= 500
    }),
    backoff.WithOnRetry(func(attempt int, err error) {
        slog.Warn("retrying request", "attempt", attempt, "error", err)
    }),
)
```

### Skip retries for non-transient errors

```go
// HTTP: retry 5xx, surface 4xx immediately.
backoff.WithRetryIf(func(err error) bool {
    var httpErr *HTTPError
    return !errors.As(err, &httpErr) || httpErr.StatusCode >= 500
})

// gRPC: retry Unavailable and DeadlineExceeded only.
backoff.WithRetryIf(func(err error) bool {
    code := status.Code(err)
    return code == codes.Unavailable || code == codes.DeadlineExceeded
})

// DB: retry connection errors, not constraint violations.
backoff.WithRetryIf(func(err error) bool {
    return errors.Is(err, driver.ErrBadConn)
})
```

### Logging and metrics

```go
backoff.WithOnRetry(func(attempt int, err error) {
    slog.Warn("operation failed, will retry",
        "attempt", attempt,
        "error",   err,
    )
    retryCounter.Add(ctx, 1)
})
```

## Options

| Option | Constraint | Default | Description |
|--------|-----------|---------|-------------|
| `WithMaxRetries(n int)` | `n >= 0` | `3` | Maximum number of retries. Total attempts = n+1. Pass `0` to disable retries. |
| `WithInitialInterval(d Duration)` | `d > 0` | `100ms` | Base wait duration before the first retry. |
| `WithMaxInterval(d Duration)` | `d > 0`, `d >= initialInterval` | `30s` | Upper bound on the wait duration between retries. |
| `WithMultiplier(m float64)` | `m >= 1.0` | `2.0` | Exponential growth factor. `2.0` doubles the interval each retry. |
| `WithJitter(j float64)` | `0.0 <= j <= 1.0` | `1.0` | Fraction of randomness. `1.0` = Full Jitter, `0.0` = No Jitter. |
| `WithRetryIf(fn func(error) bool)` | `fn != nil` | retry all | Predicate to decide whether to retry a given error. |
| `WithOnRetry(fn func(int, error))` | `fn != nil` | none | Hook called before each retry (1-indexed attempt number). Not called on the final failure. |

## Option Validation

Each option validates its argument and returns an error immediately if the
value is out of range.  `Do` also enforces the cross-field constraint
`maxInterval >= initialInterval` after all options are applied.

```go
// These all return an error from Do without calling fn:
backoff.Do(ctx, fn, backoff.WithMaxRetries(-1))
backoff.Do(ctx, fn, backoff.WithInitialInterval(0))
backoff.Do(ctx, fn, backoff.WithMaxInterval(-1*time.Second))
backoff.Do(ctx, fn, backoff.WithMultiplier(0.5))
backoff.Do(ctx, fn, backoff.WithJitter(1.5))
backoff.Do(ctx, fn, backoff.WithRetryIf(nil))
backoff.Do(ctx, fn, backoff.WithOnRetry(nil))

// Cross-field violation:
backoff.Do(ctx, fn,
    backoff.WithInitialInterval(10*time.Second),
    backoff.WithMaxInterval(1*time.Second), // maxInterval < initialInterval
)
```

## Context Cancellation

`Do` respects context cancellation both **before each sleep** (so a cancelled
context is noticed even when the sleep interval is very long) and **after each
`fn` invocation** (so a context cancelled inside `fn` is surfaced promptly even
if `fn` does not propagate the error itself).
