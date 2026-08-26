# aws-sdk-go v2 wrapper library

A collection of thin, idiomatic Go wrappers around [aws-sdk-go v2](https://github.com/aws/aws-sdk-go-v2) services.
Each package exposes both **package-level functions** (backed by a per-process singleton client) and a **`Client` struct** that you can instantiate independently for advanced lifecycle management.

---

## Table of Contents

- [Requirements](#requirements)
  - [Tracing](#tracing)
- [Packages](#packages)
  - [awsconfig](#awsconfig)
  - [ctxawslocal](#ctxawslocal)
  - [awss3](#awss3)
  - [awsdynamo](#awsdynamo)
  - [awssqs](#awssqs)
  - [awscognito](#awscognito)
- [Local Development](#local-development)

---

## Requirements

- Go 1.24+
- aws-sdk-go v2

### Tracing

Tracing is designed as a cross-AWS concern with service-specific transport
adapters:

1. S3, DynamoDB, SQS, and Cognito use the shared
   [`otelaws`](https://pkg.go.dev/go.opentelemetry.io/contrib/instrumentation/github.com/aws/aws-sdk-go-v2/otelaws)
   middleware to create AWS SDK spans and propagate context in AWS SDK HTTP
   requests. New AWS SDK wrappers should use the same shared middleware
   boundary.
2. SQS message attributes are an SQS-specific adapter. They carry the W3C
   context required by application-managed workers and are independent of
   AWS SDK HTTP header propagation.
3. Lambda invocation tracing is a runtime concern, not an SQS client concern.
   This module does not instrument Lambda handlers yet; use the
   [AWS Distro for OpenTelemetry (ADOT) Lambda integration](https://docs.aws.amazon.com/lambda/latest/dg/golang-tracing.html)
   or the applicable [OpenTelemetry AWS Lambda instrumentation](https://opentelemetry.io/docs/specs/semconv/faas/aws-lambda/)
   when that integration is added. For an SQS-triggered Lambda, the standard
   AWS propagation mechanism is `AWSTraceHeader`, not the custom SQS Message
   Attributes described below.

The shared [`tracers` package](../tracers) is only a compatibility bridge for
native Datadog v2 spans that are present in a request context. OTel-native
applications can use `otelaws` with an OTel SDK or Datadog's OTel-compatible
provider without depending on SQS-specific tracing code. The bridge does not
create or finish Datadog spans.

This separation is intentional: the provider and propagator are application
configuration, AWS SDK instrumentation is shared, and each transport or
runtime owns only the propagation rules that are specific to it.

#### Extending tracing to another AWS integration

When adding an AWS SDK-based package, enable tracing at client construction
and connect it to the shared `otelaws` middleware. Do not copy the SQS
Message Attribute injection or `ProcessMessage` lifecycle into that package.
When adding a runtime integration such as Lambda, add a runtime-specific
adapter that receives the application context and uses the same provider
configuration; keep AWS-managed event propagation (for example,
`AWSTraceHeader`) separate from application-defined transport attributes.
This keeps the public tracing contract small while allowing new AWS services
to share the same instrumentation boundary.

`WithTrace` does not create or register a `TracerProvider`. When a provider is
nil (or the SQS `TraceConfig.TracerProvider` field is omitted), configure an
SDK or Datadog-compatible provider and register it from the application's
entry point, such as `main`, before constructing a client or making the first
package-level `GetClient` call. The application startup code should call
`otel.SetTracerProvider(provider)` after constructing the SDK or
Datadog-compatible provider.

If the global provider is not configured, OpenTelemetry uses its no-op
provider and traced SQS operations return
`awssqs.ErrTraceProviderNotConfigured`. Passing
`awssqs.TraceConfig{TracerProvider: provider}` explicitly does not require a
global provider.

```go
import (
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
)

otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
	propagation.TraceContext{},
	propagation.Baggage{},
))
provider := otel.GetTracerProvider()

s3Client, err := awss3.NewClient(ctx, region, awss3.WithTrace(provider))
sqsClient, err := awssqs.NewClient(ctx, region, awssqs.WithTrace(awssqs.TraceConfig{}))
dynamoClient, err := awsdynamo.NewClient(ctx, region, awsdynamo.WithTrace(provider))
cognitoClient, err := awscognito.NewClient(ctx, region, awscognito.WithTrace(provider))
```

For the SQS package, `awssqs.WithTrace(awssqs.TraceConfig{})` uses the globally
configured OpenTelemetry `TracerProvider` and the standard W3C Trace Context
propagator. The global provider must be configured before
`NewClient` or the first `GetClient` call. To inject a custom provider, use
`awssqs.WithTrace(awssqs.TraceConfig{TracerProvider: provider})`. To add a
custom SQS propagator, use
`awssqs.WithTrace(awssqs.TraceConfig{Propagator: propagator})`; W3C Trace
Context is always included and the supplied propagator is composed after it.
For example, pass `propagation.Baggage{}` explicitly when baggage should also
be propagated; baggage is not included by the default SQS configuration.
When a request context contains a Datadog v2 span but no valid OpenTelemetry
`SpanContext`, the compatibility bridge converts that span's W3C propagation
fields into the parent context used by the AWS span.

#### SQS message-attribute trace propagation rules

When SQS message trace propagation is enabled with the standard configuration,
`traceparent` and `tracestate` are reserved message attributes. They use the
SQS `String` data type and consume two of SQS's ten attribute slots, leaving
at most eight for application-defined attributes. Baggage is not propagated
by the standard configuration; it is included only when a custom propagator
explicitly supplies it.
The final attribute count and injected W3C context are validated before the
SQS request is sent; invalid trace attributes, reserved caller attributes, or
an over-limit request return an error without sending a partial context.
Custom propagators may reserve additional fields, so the available application
attribute count is reduced accordingly. These rules preserve W3C Trace Context
while respecting the
[Amazon SQS message-attribute limit](https://docs.aws.amazon.com/AWSSimpleQueueService/latest/SQSDeveloperGuide/sqs-message-metadata.html).
This is an application-level SQS worker contract; it is not the propagation
mechanism used by an AWS-managed SQS-to-Lambda event source.

Package-level helpers use singleton clients. Initialize each singleton with
`WithTrace` before its first request; options passed after initialization do
not reconfigure an existing client. S3 and SQS are initialized through
`GetClient`, while DynamoDB operations and Cognito's package-level helper
accept `WithTrace` directly.

```go
_, err := awss3.GetClient(ctx, region, awss3.WithTrace(provider))
_, err = awssqs.GetClient(ctx, region, awssqs.WithTrace(awssqs.TraceConfig{}))
err = awsdynamo.PutItem(ctx, region, table, item, awsdynamo.WithTrace(provider))
_, err = awscognito.GetCredentialsForIdentity(ctx, region, identityID, logins,
    awscognito.WithTrace(provider))
```

---

## Packages

### awsconfig

Typed constants for AWS regions.

```go
import "github.com/88labs/go-utils/aws/awsconfig"

region := awsconfig.RegionTokyo   // "ap-northeast-1"
region := awsconfig.RegionOsaka   // "ap-northeast-3"
```

---

### ctxawslocal

> **This package is intended for use in tests only.**
> It is not meant to be used in production code.

Injects local-mock endpoint configuration into a `context.Context`.
All packages in this library check the context before dialling AWS, so your tests can redirect traffic to [LocalStack](https://localstack.cloud/), [MinIO](https://min.io/), or [ElasticMQ](https://github.com/softwaremill/elasticmq) without modifying any production code.

Wrap the context at the top of your test and pass it through to any function call:

```go
import "github.com/88labs/go-utils/aws/ctxawslocal"

func TestSomething(t *testing.T) {
    ctx := ctxawslocal.WithContext(
        context.Background(),
        ctxawslocal.WithS3Endpoint("http://127.0.0.1:9000"),    // MinIO
        ctxawslocal.WithSQSEndpoint("http://127.0.0.1:9324"),   // ElasticMQ
        ctxawslocal.WithDynamoEndpoint("http://127.0.0.1:8000"),
        ctxawslocal.WithAccessKey("test"),
        ctxawslocal.WithSecretAccessKey("test"),
    )
    // ctx is now wired to local services; pass it to awss3, awssqs, awsdynamo, etc.
    _, err := awss3.PutObject(ctx, awsconfig.RegionTokyo, awss3.BucketName("my-bucket"), awss3.Key("key.txt"), body)
    ...
}
```

| Option | Default (LocalStack) |
|---|---|
| `WithS3Endpoint` | `http://127.0.0.1:4566` |
| `WithSQSEndpoint` | `http://127.0.0.1:4566` |
| `WithDynamoEndpoint` | `http://127.0.0.1:4566` |
| `WithAccessKey` | `"test"` |
| `WithSecretAccessKey` | `"test"` |
| `WithSessionToken` | `""` |

---

### awss3

Wrapper for Amazon S3. Supports upload, download, presigning, multipart upload, and S3 Select.

#### Package-level functions (singleton client)

```go
import (
    "github.com/88labs/go-utils/aws/awss3"
    "github.com/88labs/go-utils/aws/awsconfig"
    "github.com/88labs/go-utils/aws/awss3/options/s3upload"
    "github.com/88labs/go-utils/aws/awss3/options/s3presigned"
)

const (
    region = awsconfig.RegionTokyo
    bucket = awss3.BucketName("my-bucket")
)

// Upload (multipart, recommended)
_, err := awss3.UploadManager(ctx, region, bucket, awss3.Key("path/to/key.txt"), body)

// Upload (single PUT – ContentLength must be known)
_, err = awss3.PutObject(ctx, region, bucket, awss3.Key("path/to/key.txt"), body,
    s3upload.WithS3Expires(24*time.Hour),
)

// Check object metadata
head, err := awss3.HeadObject(ctx, region, bucket, awss3.Key("path/to/key.txt"))

// List objects
objects, err := awss3.ListObjects(ctx, region, bucket,
    s3list.WithPrefix("path/to/"),
)

// Download to io.Writer
var buf bytes.Buffer
err = awss3.GetObjectWriter(ctx, region, bucket, awss3.Key("path/to/key.txt"), &buf)

// Download multiple objects to a directory (sequential)
paths, err := awss3.DownloadFiles(ctx, region, bucket, keys, "/tmp/out")

// Download multiple objects to a directory (parallel)
paths, err = awss3.DownloadFilesParallel(ctx, region, bucket, keys, "/tmp/out")

// Delete an object
_, err = awss3.DeleteObject(ctx, region, bucket, awss3.Key("path/to/key.txt"))

// Copy within the same bucket
err = awss3.Copy(ctx, region, bucket, awss3.Key("src/key.txt"), awss3.Key("dst/key.txt"))

// Presign a GET URL (default expiry: 15 minutes)
url, err := awss3.Presign(ctx, region, bucket, awss3.Key("path/to/key.txt"),
    s3presigned.WithPresignExpires(1*time.Hour),
    s3presigned.WithPresignFileName("download.txt"),
    s3presigned.WithContentDispositionType(s3presigned.ContentDispositionTypeAttachment),
)

// Presign a PUT URL
url, err = awss3.PresignPutObject(ctx, region, bucket, awss3.Key("path/to/key.txt"))

// Build a Content-Disposition header value
disposition := awss3.ResponseContentDisposition(
    s3presigned.ContentDispositionTypeAttachment,
    "レポート.pdf",
) // → `attachment; filename*=UTF-8''%E3%83%AC%E3%83%9D%E3%83%BC%E3%83%88.pdf`
```

#### Multipart upload

```go
uploadID, err := awss3.CreateMultipartUpload(ctx, region, bucket, awss3.Key("large/file.bin"))

part, err := awss3.UploadPart(ctx, region, bucket, awss3.Key("large/file.bin"), uploadID, 1, partBody)

_, err = awss3.CompleteMultipartUpload(ctx, region, bucket, awss3.Key("large/file.bin"), uploadID,
    []s3types.CompletedPart{{ETag: part.ETag, PartNumber: aws.Int32(1)}},
)

// Cancel an incomplete upload
err = awss3.AbortMultipartUpload(ctx, region, bucket, awss3.Key("large/file.bin"), uploadID)
```

#### S3 Select (CSV)

```go
var buf bytes.Buffer
err := awss3.SelectCSVAll(ctx, region, bucket, awss3.Key("data.csv"), awss3.SelectCSVAllQuery, &buf)

headers, err := awss3.SelectCSVHeaders(ctx, region, bucket, awss3.Key("data.csv"))
```

#### Client struct (independent lifecycle)

Use `NewClient` when you need multiple isolated clients or want to manage the lifecycle explicitly.

```go
client, err := awss3.NewClient(ctx, region)

_, err = client.PutObject(ctx, bucket, awss3.Key("key.txt"), body)
_, err = client.HeadObject(ctx, bucket, awss3.Key("key.txt"))
_, err = client.DeleteObject(ctx, bucket, awss3.Key("key.txt"))

// Access the underlying *s3.Client for operations not wrapped here
raw := client.S3Client()
```

#### Logging

Logging is opt-in. By default, `awss3` does not emit any logs.

When configured, wrapper methods emit structured `slog` records with fields such as `component`, `operation`, `bucket`, `key`, and `duration`.

Passing `nil` to `WithLogger`, `WithZapLogger`, or `NewLoggerFromZap` falls back to a no-op logger.

```go
import (
    "log/slog"
    "os"

    "go.uber.org/zap"
)

// Package-level helpers use the global logger.
awss3.GlobalLogger = slog.New(slog.NewJSONHandler(os.Stdout, nil))
_, err := awss3.HeadObject(ctx, region, bucket, awss3.Key("path/to/key.txt"))

// Clients can receive a logger explicitly.
client, err := awss3.NewClient(ctx, region,
    awss3.WithLogger(slog.New(slog.NewTextHandler(os.Stdout, nil))),
)

// Zap can be used via the built-in bridge helpers.
zapLogger := zap.NewExample()

client, err = awss3.NewClient(ctx, region, awss3.WithZapLogger(zapLogger))
awss3.GlobalLogger = awss3.NewLoggerFromZap(zapLogger)
```

#### Error handling

```go
_, err := awss3.HeadObject(ctx, region, bucket, awss3.Key("missing.txt"))
if errors.Is(err, awss3.ErrNotFound) {
    // object does not exist
}
```

---

### awsdynamo

Wrapper for Amazon DynamoDB with generic helpers. Because Go does not allow type parameters on methods, generic operations are only available as package-level functions.

#### Package-level functions (singleton client)

```go
import (
    "github.com/88labs/go-utils/aws/awsdynamo"
    "github.com/88labs/go-utils/aws/awsconfig"
    "github.com/88labs/go-utils/aws/awsdynamo/dynamooptions"
    "github.com/aws/aws-sdk-go-v2/feature/dynamodb/expression"
)

const (
    region = awsconfig.RegionTokyo
    table  = awsdynamo.TableName("users")
)

type User struct {
    ID   string `dynamodbav:"id"`
    Name string `dynamodbav:"name"`
}

// Upsert
err := awsdynamo.PutItem(ctx, region, table, User{ID: "u1", Name: "Alice"})

// Get (returns ErrNotFound if missing)
user, err := awsdynamo.GetItem[User](ctx, region, table, "id", "u1")

// Update specific attributes
update := expression.Set(expression.Name("name"), expression.Value("Bob"))
updated, err := awsdynamo.UpdateItem[User](ctx, region, table, "id", "u1", update)

// Delete (returns the deleted item)
deleted, err := awsdynamo.DeleteItem[User](ctx, region, table, "id", "u1")

// Batch get (automatically splits into 100-item chunks)
users, err := awsdynamo.BatchGetItem[User](ctx, region, table, "id", []string{"u1", "u2"})

// Batch write (automatically splits into 25-item chunks)
err = awsdynamo.BatchWriteItem(ctx, region, table, []User{{ID: "u2", Name: "Carol"}})
```

Custom retry configuration:

```go
err := awsdynamo.PutItem(ctx, region, table, item,
    dynamooptions.WithMaxAttempts(5),
    dynamooptions.WithMaxBackoffDelay(30*time.Second),
)
```

#### Client struct (independent lifecycle)

```go
client, err := awsdynamo.NewClient(ctx, region)

// Access the underlying *dynamodb.Client for SDK calls not wrapped here
raw := client.DynamoDBClient()
```

#### Error handling

```go
_, err := awsdynamo.GetItem[User](ctx, region, table, "id", "unknown")
if errors.Is(err, awsdynamo.ErrNotFound) {
    // item does not exist
}
```

---

### awssqs

Wrapper for Amazon SQS. Messages can be serialised as JSON or [gob](https://pkg.go.dev/encoding/gob).

#### Package-level functions (singleton client)

```go
import (
    "github.com/88labs/go-utils/aws/awssqs"
    "github.com/88labs/go-utils/aws/awsconfig"
    "github.com/88labs/go-utils/aws/awssqs/options/sqssend"
    "github.com/88labs/go-utils/aws/awssqs/options/sqsreceive"
)

const (
    region   = awsconfig.RegionTokyo
    queueURL = awssqs.QueueURL("https://sqs.ap-northeast-1.amazonaws.com/123456789012/my-queue")
)

type Task struct {
    ID   string
    Name string
}

// Send as JSON (default DelaySeconds=0)
_, err := awssqs.SendMessage(ctx, region, queueURL, Task{ID: "t1", Name: "job"},
    sqssend.WithDelaySeconds(5),
)

// Send as gob (binary encoding – more efficient for complex structs)
_, err = awssqs.SendMessageGob(ctx, region, queueURL, Task{ID: "t1", Name: "job"})

// Receive (default: MaxNumberOfMessages=1, WaitTimeSeconds=20, VisibilityTimeout=30)
out, err := awssqs.ReceiveMessage(ctx, region, queueURL,
    sqsreceive.WithMaxNumberOfMessages(10),
    sqsreceive.WithWaitTimeSeconds(5),
    sqsreceive.WithVisibilityTimeout(60),
)
for _, msg := range out.Messages {
    // process msg
    _ = awssqs.DeleteMessage(ctx, region, queueURL, msg)
}

// Receive gob-encoded messages and decode them in one step
// (generic – only available at the package level)
tasks, out, err := awssqs.ReceiveMessageGob(ctx, region, queueURL, Task{})
for i, task := range tasks {
    fmt.Println(task.Name)
    _ = awssqs.DeleteMessage(ctx, region, queueURL, out.Messages[i])
}
```

#### Client struct (independent lifecycle)

```go
client, err := awssqs.NewClient(ctx, region)

_, err = client.SendMessage(ctx, queueURL, Task{ID: "t2"})
out, err := client.ReceiveMessage(ctx, queueURL)
err = client.DeleteMessage(ctx, queueURL, out.Messages[0])

// Access the underlying *sqs.Client
raw := client.SQSClient()
```

#### Trace propagation and message processing

Tracing is opt-in. `WithTrace(TraceConfig{})` uses the global TracerProvider
and automatically configures W3C Trace Context for SQS message attributes.
The option does not initialize the provider; configure the global
provider in the application's entry point, such as `main`, before constructing
the client. If it is not configured, traced operations return
`ErrTraceProviderNotConfigured`. For explicit dependency injection, use
`WithTrace(TraceConfig{TracerProvider: provider})`. For a custom additional
propagator, use
`WithTrace(TraceConfig{TracerProvider: provider, Propagator: propagator})`;
W3C Trace Context remains enabled automatically. Send spans are named `send
<queue>`, process spans are named `process <queue>`, and the sender's context
is linked to the worker process span rather than used as its parent. To
customize only the operation portion for one call, use
`sqssend.WithOperationName("publish")` or
`sqsprocess.WithOperationName("consume")`; the queue name remains in the final
span name.

```go
import (
    "context"

    "github.com/aws/aws-sdk-go-v2/service/sqs/types"
    "github.com/88labs/go-utils/aws/awssqs"
    "github.com/88labs/go-utils/tracers"
)

client, err := awssqs.NewClient(ctx, region, awssqs.WithTrace(awssqs.TraceConfig{}))

_, err = client.SendMessage(ctx, queueURL, Task{ID: "t3"})
out, err := client.ReceiveMessage(ctx, queueURL)
for _, msg := range out.Messages {
    err = client.ProcessMessage(ctx, queueURL, msg, func(ctx context.Context, msg types.Message) error {
        // The active span in ctx is the worker process span.
        if messageTrace, ok := awssqs.ExtractMessageTraceContext(ctx); ok {
            sourceTraceID := messageTrace.GetTraceID(tracers.FormatString)
            sourceSpanID := messageTrace.GetSpanIDUInt64()
            _ = sourceTraceID // add these values to the application log
            _ = sourceSpanID
        }
        // process msg using ctx
        return nil // ProcessMessage deletes the message after a nil result
    })
}
```

##### `ProcessMessage` lifecycle

`ProcessMessage` is a single-message helper that coordinates handler execution
and acknowledgement. It does not call `ReceiveMessage`, start a polling loop,
or process messages concurrently. The caller owns receiving messages and
choosing the polling, concurrency, visibility-timeout, retry, and DLQ policies.

For each message, the processing sequence is:

1. A nil handler returns `ErrNilMessageHandler` without invoking SQS.
2. When tracing is enabled, the client extracts the sender context from the
   message attributes and starts a `process <queue>` consumer span. The
   context passed to `ProcessMessage` is the process span's parent. A valid
   sender context is recorded as a span link, not used as the process span's
   parent. The handler receives the derived process context and can retrieve
   the sender's context with `ExtractMessageTraceContext`. When tracing is
   disabled, it receives the context passed by the caller unchanged.
3. The handler is called with the message and the context from step 2. Handler
   code should use that context when creating child spans or emitting
   correlated logs. `ExtractMessageTraceContext` returns the sender's
   `tracers.TraceInfo`, not the active process span; use it to add separate
   source trace fields to logs without overwriting the worker trace fields.
4. If the handler returns an error, `ProcessMessage` records the error on the
   process span (when tracing is enabled), returns the error, and does not call
   `DeleteMessage`. The message is therefore left to the queue's normal retry
   and DLQ behavior.
5. If the handler returns nil, the client calls `DeleteMessage` using the
   derived process context. The delete operation is represented by a child
   `delete <queue>` span when tracing is enabled. A delete error is returned
   and recorded on the process span.
6. The process span is ended after handler execution and acknowledgement,
   including all error paths.

##### `ExtractMessageTraceContext`

`ExtractMessageTraceContext` is available from the context passed to a
`ProcessMessage` handler. It returns the valid W3C `traceparent` and optional
`tracestate` propagated by the sender. The active span in the same context is
still the worker's `process <queue>` span, so the two values have different
purposes:

- Use `tracers.ExtractTraceContext(ctx)` for the worker process trace and span.
- Use `awssqs.ExtractMessageTraceContext(ctx)` for the source message trace and
  span, for example as `source_trace_id` and `source_span_id` log fields.

The accessor returns `(tracers.TraceInfo{}, false)` when tracing is disabled,
when the message has no valid trace context, or when it is called outside a
`ProcessMessage` handler. `TraceInfo.GetTraceID(tracers.FormatString)` keeps
the complete W3C 128-bit trace ID, while `GetSpanIDUInt64()` provides the
numeric span ID representation used by Datadog log correlation.

Malformed or incomplete incoming trace attributes are recorded as process-span
errors, but do not prevent the handler from running. This keeps message
processing independent from trace-data validity while still making the issue
visible in telemetry.

The standard tracing configuration reserves two message attributes:
`traceparent` and `tracestate`. They are added as SQS `String` attributes and
cannot be supplied by the caller, leaving at most eight application
attributes. Baggage is propagated only when a custom propagator includes it;
custom propagators may reserve additional attributes. All fields declared by
the propagator are reserved even when an optional field is not injected for a
particular context. Application attribute maps are copied before trace
attributes are added, and the final count is validated before sending.
`SendMessageBatch` creates a `create <queue>` producer span for every entry and
a `send <queue>` client span linked to those creation contexts. Partial failures
returned in the batch response are recorded on the send span. The SDK response
and its `Failed` entries are returned unchanged with a nil Go error, matching
the AWS SDK contract.
The package-level helpers use the same behavior when the singleton is first
initialized with `awssqs.GetClient(ctx, region, awssqs.WithTrace(awssqs.TraceConfig{}))`.

`ReceiveMessage` remains a thin SDK wrapper. Calls through `SQSClient()` use the
raw AWS SDK and do not receive message-attribute propagation or the
`ProcessMessage` acknowledgement contract.

---

### awscognito

Wrapper for Amazon Cognito Identity.

#### Package-level function (singleton client)

```go
import (
    "github.com/88labs/go-utils/aws/awscognito"
    "github.com/88labs/go-utils/aws/awsconfig"
)

out, err := awscognito.GetCredentialsForIdentity(
    ctx,
    awsconfig.RegionTokyo,
    "ap-northeast-1:xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx",
    map[string]string{
        "cognito-idp.ap-northeast-1.amazonaws.com/ap-northeast-1_XXXXXXXXX": idToken,
    },
)
creds := out.Credentials // AccessKeyId, SecretKey, SessionToken, Expiration
```

#### Client struct (independent lifecycle)

```go
client, err := awscognito.NewClient(ctx, awsconfig.RegionTokyo)

out, err := client.GetCredentialsForIdentity(ctx, identityID, logins)

// Access the underlying *cognitoidentity.Client
raw := client.CognitoClient()
```

---

## Local Development

### Prerequisites

- Docker Compose v2
- [LocalStack](https://localstack.cloud/) – SQS, DynamoDB
- [MinIO](https://min.io/) – S3-compatible object storage
- [ElasticMQ](https://github.com/softwaremill/elasticmq) – SQS-compatible queue

### Running tests

```shell
# Start all local services
docker compose up -d

# Run all tests
go test ./...
```
