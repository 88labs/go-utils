# backoff

HTTP リクエスト・gRPC 呼び出し・DB クエリなど、一時的なエラーが発生し得るあらゆる操作に使える、汎用的な Exponential Backoff with Jitter の実装です。

デフォルトは AWS ブログが推奨する [Full Jitter](https://aws.amazon.com/blogs/architecture/exponential-backoff-and-jitter/) 戦略を使用します。  
Full Jitter はリトライのタイミングをランダムに分散させることで、多数のクライアントが一斉にリトライする「Thundering Herd」問題を防ぎます。

## Usage

### 基本

```go
err := backoff.Do(ctx, func(ctx context.Context) error {
    return client.Call(ctx, req)
})
```

### オプション指定

```go
err := backoff.Do(ctx, func(ctx context.Context) error {
    return client.Call(ctx, req)
},
    backoff.WithMaxRetries(5),
    backoff.WithInitialInterval(200*time.Millisecond),
    backoff.WithMaxInterval(10*time.Second),
    backoff.WithMultiplier(1.5),
    backoff.WithJitter(1.0),
)
```

### 恒久的なエラーはリトライしない

```go
err := backoff.Do(ctx, func(ctx context.Context) error {
    resp, err := httpClient.Do(req)
    if err != nil {
        return err
    }
    if resp.StatusCode >= 400 && resp.StatusCode < 500 {
        return backoff.WithRetryIf(func(err error) bool { return false }) // 4xx はリトライしない
    }
    return nil
},
    backoff.WithRetryIf(func(err error) bool {
        var httpErr *HTTPError
        if errors.As(err, &httpErr) {
            return httpErr.StatusCode >= 500 // 5xx のみリトライ
        }
        return true
    }),
)
```

### ログ・メトリクス記録

```go
err := backoff.Do(ctx, func(ctx context.Context) error {
    return db.QueryContext(ctx, query)
},
    backoff.WithOnRetry(func(attempt int, err error) {
        slog.Warn("db query failed, retrying",
            "attempt", attempt,
            "error", err,
        )
    }),
)
```

## オプション一覧

| オプション | デフォルト | 説明 |
|---|---|---|
| `WithMaxRetries(n)` | `3` | 最大リトライ回数。合計試行回数は n+1 回 |
| `WithInitialInterval(d)` | `100ms` | 初回リトライ前の待ち時間 |
| `WithMaxInterval(d)` | `30s` | リトライ間隔の上限 |
| `WithMultiplier(m)` | `2.0` | 各リトライで間隔を何倍に増やすか |
| `WithJitter(j)` | `1.0` | ランダム性の強さ（0.0=なし / 1.0=Full Jitter） |
| `WithRetryIf(fn)` | 全エラーをリトライ | リトライ判定関数 |
| `WithOnRetry(fn)` | なし | リトライ前に呼ばれるコールバック（ログ・メトリクス用） |

## バックオフの計算式

```
cap   = min(maxInterval, initialInterval × multiplier^(attempt-1))
sleep = cap × (1 − jitter) + rand(0, cap) × jitter
```

- `jitter = 1.0`（Full Jitter）: sleep は `[0, cap]` の一様乱数
- `jitter = 0.0`（No Jitter）: sleep は常に `cap`（決定論的）
