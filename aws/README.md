# aws-sdk-go v2 wrapper library

A collection of thin, idiomatic Go wrappers around [aws-sdk-go v2](https://github.com/aws/aws-sdk-go-v2) services.
Each package exposes both **package-level functions** (backed by a per-process singleton client) and a **`Client` struct** that you can instantiate independently for advanced lifecycle management.

---

## Table of Contents

- [Requirements](#requirements)
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
