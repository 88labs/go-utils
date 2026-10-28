# Change Data Capture (CDC)

Syncs data from a source MySQL database to a target database using [Debezium](https://debezium.io/) change events delivered through AWS SNS and SQS.

Proposal: https://88-oct.atlassian.net/wiki/spaces/OCT/pages/5278925102/Sync+Data+using+Debezium+CDC

## Overview

```text
MySQL (source) -> Debezium Server -> SNS topic -> SQS queue -> cdc.Consumer -> MySQL (target)
```

Debezium Server reads the MySQL binlog and publishes one SNS topic per table. An SQS queue subscribed to that topic buffers the events, and the Go consumer in this module routes each event to a per-table handler that applies it to the target database.

## Infrastructure

The examples below target a local development environment (Floci for AWS, `--profile local` for the AWS CLI).

### 1. Debezium Server

Add the following service to your `docker-compose.yml`:

```yaml
debezium:
  image: quay.io/debezium/server:nightly
  volumes:
    - "debezium_data:/debezium/data"
  environment:
    AWS_ACCESS_KEY_ID: test
    AWS_SECRET_ACCESS_KEY: test
    AWS_REGION: ap-northeast-1
    DEBEZIUM_SOURCE_CONNECTOR_CLASS: io.debezium.connector.mysql.MySqlConnector
    DEBEZIUM_SOURCE_DATABASE_HOSTNAME: host.docker.internal
    DEBEZIUM_SOURCE_DATABASE_PORT: "3306"
    DEBEZIUM_SOURCE_DATABASE_USER: root
    DEBEZIUM_SOURCE_DATABASE_PASSWORD: "${DB_PASSWORD}"
    DEBEZIUM_SOURCE_DATABASE_SERVER_ID: 184055
    DEBEZIUM_SOURCE_TOPIC_PREFIX: cdc
    DEBEZIUM_SOURCE_DATABASE_INCLUDE_LIST: andpad_development
    DEBEZIUM_SOURCE_TABLE_INCLUDE_LIST: andpad_development.orders
    DEBEZIUM_SOURCE_INCLUDE_SCHEMA_CHANGES: "false"
    DEBEZIUM_SOURCE_OFFSET_STORAGE_FILE_FILENAME: /debezium/data/offsets.dat
    DEBEZIUM_SOURCE_SCHEMA_HISTORY_INTERNAL: io.debezium.storage.file.history.FileSchemaHistory
    DEBEZIUM_SOURCE_SCHEMA_HISTORY_INTERNAL_FILE_FILENAME: /debezium/data/schema-history.dat
    DEBEZIUM_SINK_TYPE: sns
    DEBEZIUM_SINK_SNS_REGION: ap-northeast-1
    DEBEZIUM_SINK_SNS_ENDPOINT: http://host.docker.internal:4566
    DEBEZIUM_SINK_SNS_TOPIC_ARN_PREFIX: "arn:aws:sns:ap-northeast-1:000000000000:"

volumes:
  debezium_data:
```

Debezium names each topic `<topic prefix>.<database>.<table>`, so the `orders` table above publishes to `cdc.andpad_development.orders`.

### 2. SNS topic

```sh
aws --profile local sns create-topic --name cdc.andpad_development.orders
```

### 3. SQS queue

```sh
aws --profile local sqs create-queue --queue-name cdc.andpad_development.orders-remote
```

### 4. Subscription

Subscribe the queue to the topic:

```sh
aws --profile local sns subscribe \
  --protocol sqs \
  --topic-arn arn:aws:sns:ap-northeast-1:000000000000:cdc.andpad_development.orders \
  --notification-endpoint arn:aws:sqs:ap-northeast-1:000000000000:cdc.andpad_development.orders-remote
```

## Consumer

The following example registers a handler for the `orders` table and starts consuming events from the queue:

```go
db, err := sql.Open("mysql", os.Getenv("TARGET_DSN"))
if err != nil {
	panic(err)
}
db.SetMaxOpenConns(20)

syncer := mysqlsink.New(db)
r := cdc.NewRouter()

syncer.Table(r, "orders", mysqlsink.TableConfig{
	AllColumns:    true,
	VersionColumn: "updated_at",
	Exclude:       []string{"guarantee_tel", "billing_tel", "billing_email"}, // Example of excluding PII columns.
})

awsCfg, err := config.LoadDefaultConfig(ctx)
if err != nil {
	panic(err)
}

c := cdc.NewConsumer(sqs.NewFromConfig(awsCfg), r, cdc.Config{
	QueueURL:       os.Getenv("QUEUE_URL"),
	Concurrency:    10,
	HandlerTimeout: 30 * time.Second,
	// Quadratic backoff (n² × 5 seconds), capped at 300 seconds.
	RetryDelay: func(n int) time.Duration { return time.Duration(min(n*n*5, 300)) * time.Second },
})
if err := c.Run(ctx); err != nil {
	panic(err)
}
```

### Configuration

| Option | Description |
| --- | --- |
| `QUEUE_URL` | URL of the SQS queue to consume from. |
| `TARGET_DSN` | DSN of the target MySQL database. |
| `Concurrency` | Number of events handled in parallel. |
| `HandlerTimeout` | Maximum time a single handler may run. |
| `RetryDelay` | Delay before an event is retried, given the number of previous attempts `n`. |
| `TableConfig.VersionColumn` | Column used to version rows (here, `updated_at`). |
| `TableConfig.Exclude` | Columns that are never copied to the target, for example PII. |
