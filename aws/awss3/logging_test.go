package awss3_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/s3/transfermanager"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
	"gotest.tools/v3/assert"

	"github.com/88labs/go-utils/ulid"

	"github.com/88labs/go-utils/aws/awss3"
	"github.com/88labs/go-utils/aws/ctxawslocal"
)

func TestNewClient_WithLogger_logsHeadObject(t *testing.T) {
	t.Parallel()

	ctx := newLoggingTestContext()
	key := uploadLoggingFixture(t, ctx, 64)

	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	client, err := awss3.NewClient(ctx, TestRegion, awss3.WithLogger(logger))
	assert.NilError(t, err)

	res, err := client.HeadObject(ctx, TestBucket, key)
	assert.NilError(t, err)
	assert.DeepEqual(t, aws.Int64(64), res.ContentLength)

	entry := decodeLastJSONLogEntry(t, buf.String())
	assert.Equal(t, entry["msg"], "awss3 operation completed")
	assert.Equal(t, entry["component"], "awss3")
	assert.Equal(t, entry["operation"], "HeadObject")
	assert.Equal(t, entry["bucket"], TestBucket)
	assert.Equal(t, entry["key"], key.String())
}

func TestNewClient_WithZapLogger_logsHeadObject(t *testing.T) {
	t.Parallel()

	ctx := newLoggingTestContext()
	key := uploadLoggingFixture(t, ctx, 32)

	core, observedLogs := observer.New(zap.InfoLevel)
	client, err := awss3.NewClient(ctx, TestRegion, awss3.WithZapLogger(zap.New(core)))
	assert.NilError(t, err)

	res, err := client.HeadObject(ctx, TestBucket, key)
	assert.NilError(t, err)
	assert.DeepEqual(t, aws.Int64(32), res.ContentLength)

	entries := observedLogs.AllUntimed()
	assert.Equal(t, len(entries), 1)
	assert.Equal(t, entries[0].Message, "awss3 operation completed")

	fields := entries[0].ContextMap()
	assert.Equal(t, fields["component"], "awss3")
	assert.Equal(t, fields["operation"], "HeadObject")
	assert.Equal(t, fields["bucket"], TestBucket)
	assert.Equal(t, fields["key"], key.String())
}

func TestNewClient_WithNilLogger_usesNoopLogger(t *testing.T) {
	ctx := newLoggingTestContext()
	key := uploadLoggingFixture(t, ctx, 16)

	var buf bytes.Buffer
	awss3.GlobalLogger = slog.New(slog.NewJSONHandler(&buf, nil))
	t.Cleanup(func() {
		awss3.GlobalLogger = nil
	})

	client, err := awss3.NewClient(ctx, TestRegion, awss3.WithLogger(nil))
	assert.NilError(t, err)

	res, err := client.HeadObject(ctx, TestBucket, key)
	assert.NilError(t, err)
	assert.DeepEqual(t, aws.Int64(16), res.ContentLength)
	assert.Equal(t, strings.TrimSpace(buf.String()), "")
}

func TestNewClient_WithNilZapLogger_usesNoopLogger(t *testing.T) {
	ctx := newLoggingTestContext()
	key := uploadLoggingFixture(t, ctx, 24)

	var buf bytes.Buffer
	awss3.GlobalLogger = slog.New(slog.NewJSONHandler(&buf, nil))
	t.Cleanup(func() {
		awss3.GlobalLogger = nil
	})

	client, err := awss3.NewClient(ctx, TestRegion, awss3.WithZapLogger(nil))
	assert.NilError(t, err)

	res, err := client.HeadObject(ctx, TestBucket, key)
	assert.NilError(t, err)
	assert.DeepEqual(t, aws.Int64(24), res.ContentLength)
	assert.Equal(t, strings.TrimSpace(buf.String()), "")
}

func newLoggingTestContext() context.Context {
	return ctxawslocal.WithContext(
		context.Background(),
		ctxawslocal.WithS3Endpoint("http://127.0.0.1:29000"),
		ctxawslocal.WithAccessKey("DUMMYACCESSKEYEXAMPLE"),
		ctxawslocal.WithSecretAccessKey("DUMMYSECRETKEYEXAMPLE"),
	)
}

func uploadLoggingFixture(t *testing.T, ctx context.Context, size int) awss3.Key {
	t.Helper()

	s3Client, err := awss3.GetClient(ctx, TestRegion)
	assert.NilError(t, err)

	key := awss3.Key(fmt.Sprintf("awstest/%s.txt", ulid.MustNew()))
	uploader := transfermanager.New(s3Client)
	input := &transfermanager.UploadObjectInput{
		Body:    bytes.NewReader(bytes.Repeat([]byte{1}, size)),
		Bucket:  aws.String(TestBucket),
		Key:     aws.String(key.String()),
		Expires: aws.Time(time.Now().Add(10 * time.Minute)),
	}
	_, err = uploader.UploadObject(ctx, input)
	assert.NilError(t, err)

	return key
}

func decodeLastJSONLogEntry(t *testing.T, raw string) map[string]any {
	t.Helper()

	lines := strings.Split(strings.TrimSpace(raw), "\n")
	assert.Assert(t, len(lines) > 0)

	entry := make(map[string]any)
	err := json.Unmarshal([]byte(lines[len(lines)-1]), &entry)
	assert.NilError(t, err)

	return entry
}
