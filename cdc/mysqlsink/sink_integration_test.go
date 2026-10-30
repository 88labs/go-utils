// The integration test needs the stack from aws/compose.yaml (profile "cdc") and
// skips itself unless these variables are set:
//
//	CDC_IT_SOURCE_DSN    MySQL DSN of the database Debezium reads
//	CDC_IT_TARGET_DSN    MySQL DSN of the database mysqlsink writes
//	CDC_IT_AWS_ENDPOINT  SNS/SQS endpoint (floci)
//
// The DSNs must use parseTime=true&loc=UTC, see the mysqlsink package doc.
// CDC_IT_QUEUE_NAME optionally overrides the queue name.
// `task test-cdc-integration` starts the stack, sets the variables and tears it down.
//
// It exercises the real pipeline:
// MySQL binlog -> Debezium Server -> SNS -> SQS -> cdc.Consumer -> mysqlsink -> MySQL.
package mysqlsink_test

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	_ "github.com/go-sql-driver/mysql"

	"github.com/88labs/go-utils/cdc"
	"github.com/88labs/go-utils/cdc/mysqlsink"
)

const (
	defaultQueueName = "cdc-orders"
	syncTimeout      = 90 * time.Second
)

// integrationEnv returns the addresses of the stack, or skips the test.
func integrationEnv(t *testing.T) (sourceDSN, targetDSN, awsEndpoint string) {
	t.Helper()
	keys := []string{"CDC_IT_SOURCE_DSN", "CDC_IT_TARGET_DSN", "CDC_IT_AWS_ENDPOINT"}
	vals := make([]string, len(keys))
	for i, k := range keys {
		if vals[i] = os.Getenv(k); vals[i] == "" {
			t.Skipf("set %s to run this test (task test-cdc-integration sets all of them)", k)
		}
	}
	return vals[0], vals[1], vals[2]
}

func TestIntegrationSourceUpdateIsSyncedToTarget(t *testing.T) {
	sourceDSN, targetDSN, awsEndpoint := integrationEnv(t)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	source := openDB(t, sourceDSN)
	target := openDB(t, targetDSN)
	sqsClient := newSQSClient(ctx, t, awsEndpoint)
	queueURL := queueURL(ctx, t, sqsClient)

	router := cdc.NewRouter()
	mysqlsink.New(target).Table(router, "orders", mysqlsink.TableConfig{
		Include: []string{"id", "name", "status", "updated_at"},
	})
	consumer := cdc.NewConsumer(sqsClient, router, cdc.Config{
		QueueURL:        queueURL,
		WaitTimeSeconds: 1,
		Concurrency:     2,
		HandlerTimeout:  30 * time.Second,
		OnError: func(_ context.Context, err error, meta cdc.Meta) {
			t.Logf("consumer error (sqs=%s receive=%d): %v", meta.SQSMessageID, meta.ReceiveCount, err)
		},
	})

	runCtx, stop := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = consumer.Run(runCtx)
	}()
	t.Cleanup(func() {
		stop()
		<-done
	})

	// Debezium takes an initial snapshot of the seeded row (op=r). Seeing it in
	// the target proves the whole pipeline is up before we change anything.
	eventually(t, syncTimeout, "snapshot row to reach the target", func() (bool, error) {
		name, found, err := targetName(ctx, target, 1)
		return found && name == "initial", err
	})

	// The actual scenario: update the source row, expect the target to follow.
	want := fmt.Sprintf("updated-%d", time.Now().UnixNano())
	if _, err := source.ExecContext(ctx, "UPDATE orders SET name = ?, status = 'shipped' WHERE id = 1", want); err != nil {
		t.Fatalf("update source: %v", err)
	}

	eventually(t, syncTimeout, "source update to reach the target", func() (bool, error) {
		name, found, err := targetName(ctx, target, 1)
		return found && name == want, err
	})

	var (
		status     string
		secretNote sql.NullString
	)
	if err := target.QueryRowContext(ctx, "SELECT status, secret_note FROM orders WHERE id = 1").Scan(&status, &secretNote); err != nil {
		t.Fatalf("read target row: %v", err)
	}
	if status != "shipped" {
		t.Fatalf("target status = %q, want %q", status, "shipped")
	}
	// The source row has a secret_note, but Include does not list it.
	if secretNote.Valid {
		t.Fatalf("secret_note must not be copied, got %q", secretNote.String)
	}
}

func targetName(ctx context.Context, db *sql.DB, id int) (name string, found bool, err error) {
	err = db.QueryRowContext(ctx, "SELECT name FROM orders WHERE id = ?", id).Scan(&name)
	if err == sql.ErrNoRows {
		return "", false, nil
	}
	return name, err == nil, err
}

func eventually(t *testing.T, timeout time.Duration, what string, cond func() (bool, error)) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	var lastErr error
	for time.Now().Before(deadline) {
		ok, err := cond()
		if ok {
			return
		}
		lastErr = err
		time.Sleep(500 * time.Millisecond)
	}
	t.Fatalf("timed out after %s waiting for %s (last error: %v)", timeout, what, lastErr)
}

func openDB(t *testing.T, dsn string) *sql.DB {
	t.Helper()
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatalf("open mysql: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	deadline := time.Now().Add(60 * time.Second)
	for {
		err = db.Ping()
		if err == nil {
			return db
		}
		if time.Now().After(deadline) {
			t.Fatalf("mysql is not reachable: %v", err)
		}
		time.Sleep(time.Second)
	}
}

func newSQSClient(ctx context.Context, t *testing.T, endpoint string) *sqs.Client {
	t.Helper()
	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion("ap-northeast-1"),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider("test", "test", "")),
	)
	if err != nil {
		t.Fatalf("load aws config: %v", err)
	}
	return sqs.NewFromConfig(cfg, func(o *sqs.Options) { o.BaseEndpoint = aws.String(endpoint) })
}

func queueURL(ctx context.Context, t *testing.T, client *sqs.Client) string {
	t.Helper()
	name := envOr("CDC_IT_QUEUE_NAME", defaultQueueName)
	var url string
	eventually(t, 60*time.Second, "SQS queue "+name, func() (bool, error) {
		out, err := client.GetQueueUrl(ctx, &sqs.GetQueueUrlInput{QueueName: aws.String(name)})
		if err != nil {
			return false, err
		}
		url = aws.ToString(out.QueueUrl)
		return true, nil
	})
	return url
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
