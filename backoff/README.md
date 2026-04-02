# backoff

`backoff` implements **Exponential Backoff with Full Jitter** for retrying
operations that may fail transiently — HTTP requests, gRPC calls, database
queries, or any function that returns an error.

The API is intentionally minimal and inspired by
[sourcegraph/conc](https://github.com/sourcegraph/conc):

- **[`New()`]** — retry an operation that returns only an `error`.
- **[`NewWithResult[T]()`]** — retry an operation that returns a typed value
  alongside an `error`; the result is forwarded to the caller on success.

---

## Algorithm

The sleep duration before retry *i* (1-indexed) is:

```
cap_i = min(maxInterval, initialInterval × multiplier^(i-1))
sleep = cap_i × (1 − jitter) + rand[0, cap_i) × jitter
```

With the default `jitter = 1.0` (Full Jitter), `sleep` is uniformly
distributed in `[0, cap_i]`, which is the strategy recommended by the
[AWS Architecture Blog](https://aws.amazon.com/blogs/architecture/exponential-backoff-and-jitter/)
for preventing thundering-herd effects in distributed systems.

---

## Defaults

| Parameter         | Default | Description                                  |
|-------------------|---------|----------------------------------------------|
| `MaxRetries`      | `3`     | Up to 4 total calls (1 initial + 3 retries)  |
| `InitialInterval` | `100ms` | Base cap before the first retry              |
| `MaxInterval`     | `30s`   | Upper bound on the sleep cap                 |
| `Multiplier`      | `2.0`   | Exponential growth factor                    |
| `Jitter`          | `1.0`   | Full Jitter — uniform in `[0, cap_i]`        |
| `RetryIf`         | all     | All errors are retried by default            |
| `OnRetry`         | `nil`   | No callback                                  |

---

## Usage

### Error-only retry

```go
import (
    "context"
    "time"

    "github.com/88labs/go-utils/backoff"
)

err := backoff.New().
    WithMaxRetries(5).
    WithInitialInterval(200 * time.Millisecond).
    Do(ctx, func(ctx context.Context) error {
        return client.Call(ctx, req)
    })
```

### Typed-result retry

```go
result, err := backoff.NewWithResult[*MyResponse]().
    WithMaxRetries(5).
    WithRetryIf(func(err error) bool {
        var httpErr *HTTPError
        return !errors.As(err, &httpErr) || httpErr.StatusCode >= 500
    }).
    Do(ctx, func(ctx context.Context) (*MyResponse, error) {
        return client.GetData(ctx, req)
    })
```

### HTTP request

```go
type Response struct{ Body []byte }

resp, err := backoff.NewWithResult[*Response]().
    WithMaxRetries(4).
    WithInitialInterval(100 * time.Millisecond).
    WithMaxInterval(5 * time.Second).
    WithRetryIf(func(err error) bool {
        var e *HTTPError
        if errors.As(err, &e) {
            return e.StatusCode >= 500 // retry server errors only
        }
        return true
    }).
    WithOnRetry(func(attempt int, err error) {
        log.Printf("HTTP retry #%d: %v", attempt, err)
    }).
    Do(ctx, func(ctx context.Context) (*Response, error) {
        return httpClient.Do(ctx, req)
    })
```

### gRPC call

```go
import "google.golang.org/grpc/codes"
import "google.golang.org/grpc/status"

resp, err := backoff.NewWithResult[*pb.Reply]().
    WithMaxRetries(3).
    WithRetryIf(func(err error) bool {
        switch status.Code(err) {
        case codes.Unavailable, codes.DeadlineExceeded, codes.ResourceExhausted:
            return true
        default:
            return false
        }
    }).
    Do(ctx, func(ctx context.Context) (*pb.Reply, error) {
        return grpcClient.Call(ctx, req)
    })
```

### Database query

```go
row, err := backoff.NewWithResult[*sql.Rows]().
    WithMaxRetries(3).
    WithInitialInterval(50 * time.Millisecond).
    WithRetryIf(func(err error) bool {
        return isTransientDBError(err) // e.g. connection reset, lock timeout
    }).
    Do(ctx, func(ctx context.Context) (*sql.Rows, error) {
        return db.QueryContext(ctx, query, args...)
    })
```

---

## Options

| Method                     | Default  | Constraint           | Description                                        |
|----------------------------|----------|----------------------|----------------------------------------------------|
| `WithMaxRetries(n)`        | `3`      | `n >= 0`             | Maximum retries after the first attempt            |
| `WithInitialInterval(d)`   | `100ms`  | `d > 0`              | Base wait cap before the first retry               |
| `WithMaxInterval(d)`       | `30s`    | `d > 0`              | Upper bound on the wait cap                        |
| `WithMultiplier(m)`        | `2.0`    | `m >= 1.0`           | Exponential growth factor                          |
| `WithJitter(j)`            | `1.0`    | `0.0 <= j <= 1.0`    | Randomness factor (0 = none, 1 = Full Jitter)      |
| `WithRetryIf(fn)`          | all      | `fn != nil`          | Predicate deciding whether to retry an error       |
| `WithOnRetry(fn)`          | `nil`    | `fn != nil`          | Callback invoked before each retry (1-indexed)     |

> **Cross-field constraint:** `maxInterval >= initialInterval`.  
> This is enforced by `Do`, not by individual `With*` calls, because both
> values can be set in any order.

---

## Reusable Presets with `Configurator` (Go 1.26)

Go 1.26 lifted the restriction on self-referential type parameter constraints,
enabling the [`Configurator[C Configurator[C]]`] interface in this package.

`Configurator` is satisfied by both `*Retryer` and `*ResultRetryer[T]`.  The
self-referential constraint ensures each `With*` method returns the *same
concrete type* `C`, so a single generic preset function applies to **both**
retryer types without any duplication or loss of type information:

```go
// Define a preset once — works for any retryer type.
func GRPCPolicy[C backoff.Configurator[C]](c C) C {
    return c.
        WithMaxRetries(4).
        WithInitialInterval(100 * time.Millisecond).
        WithMaxInterval(5 * time.Second).
        WithRetryIf(func(err error) bool {
            switch status.Code(err) {
            case codes.Unavailable, codes.ResourceExhausted:
                return true
            }
            return false
        })
}

// Apply to *Retryer — returns *Retryer.
err := GRPCPolicy(backoff.New()).Do(ctx, fn)

// Apply to *ResultRetryer[*pb.Reply] — returns *ResultRetryer[*pb.Reply].
reply, err := GRPCPolicy(backoff.NewWithResult[*pb.Reply]()).Do(ctx, fn)
```

The Go 1.26 self-referential constraint `C Configurator[C]` would not have
been legal in Go 1.25 or earlier.

---

## Validation

**Per-option violations** are *programming errors* and **panic** immediately:

```go
// panics: "backoff: WithMaxRetries: n must be >= 0, got -1"
backoff.New().WithMaxRetries(-1)

// panics: "backoff: WithInitialInterval: d must be > 0, got 0s"
backoff.New().WithInitialInterval(0)

// panics: "backoff: WithJitter: j must be in [0.0, 1.0], got 1.5"
backoff.New().WithJitter(1.5)
```

**Cross-field violations** are returned as an `error` from `Do`:

```go
err := backoff.New().
    WithInitialInterval(10 * time.Second).
    WithMaxInterval(1 * time.Second). // invalid: maxInterval < initialInterval
    Do(ctx, fn)
// err == "backoff: maxInterval (1s) must be >= initialInterval (10s)"
```

---

## Context cancellation

`Do` respects `ctx` cancellation in two places:

1. **Before each sleep** — `ctx.Done()` is checked in a `select`; a closed
   context exits immediately with `ctx.Err()`.
2. **After each `fn` call** — if `fn` returns an error *and* `ctx.Err() != nil`,
   `Do` returns `ctx.Err()` rather than the error from `fn`.

This ensures that a cancelled context is always surfaced promptly, even when
`fn` does not propagate the context cancellation itself.

```go
ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
defer cancel()

err := backoff.New().
    WithMaxRetries(10).
    Do(ctx, func(ctx context.Context) error {
        return client.Call(ctx, req) // context is forwarded to the call
    })
// err is context.DeadlineExceeded when the timeout fires
```
