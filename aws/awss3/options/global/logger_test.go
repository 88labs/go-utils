package global_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/s3/transfermanager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/stretchr/testify/assert"

	"github.com/88labs/go-utils/ulid"

	"github.com/88labs/go-utils/aws/awss3"
	"github.com/88labs/go-utils/aws/ctxawslocal"
)

func TestGlobalLoggerWithHeadObject(t *testing.T) {
	if _, ok := os.LookupEnv("CI"); ok {
		t.Skip("Skip the test in CI environment.")
		return
	}

	ctx := ctxawslocal.WithContext(
		context.Background(),
		ctxawslocal.WithS3Endpoint("http://127.0.0.1:29000"),
		ctxawslocal.WithAccessKey("DUMMYACCESSKEYEXAMPLE"),
		ctxawslocal.WithSecretAccessKey("DUMMYSECRETKEYEXAMPLE"),
	)
	s3Client, err := awss3.GetClient(ctx, TestRegion)
	assert.NoError(t, err)

	key := createLoggerFixture(t, ctx, s3Client, 100)

	var buf bytes.Buffer
	awss3.GlobalLogger = slog.New(slog.NewJSONHandler(&buf, nil))
	t.Cleanup(func() {
		awss3.GlobalLogger = nil
	})

	res, err := awss3.HeadObject(ctx, TestRegion, TestBucket, key)
	assert.NoError(t, err)
	assert.Equal(t, aws.Int64(100), res.ContentLength)

	entry := decodeLastGlobalLogEntry(t, buf.String())
	assert.Equal(t, "awss3 operation completed", entry["msg"])
	assert.Equal(t, "awss3", entry["component"])
	assert.Equal(t, "HeadObject", entry["operation"])
	assert.Equal(t, TestBucket, entry["bucket"])
	assert.Equal(t, key.String(), entry["key"])
}

func createLoggerFixture(t *testing.T, ctx context.Context, s3Client *s3.Client, fileSize int) awss3.Key {
	t.Helper()

	key := fmt.Sprintf("awstest/%s.txt", ulid.MustNew())
	uploader := transfermanager.New(s3Client)
	input := &transfermanager.UploadObjectInput{
		Body:    bytes.NewReader(bytes.Repeat([]byte{1}, fileSize)),
		Bucket:  aws.String(TestBucket),
		Key:     aws.String(key),
		Expires: aws.Time(time.Now().Add(10 * time.Minute)),
	}
	_, err := uploader.UploadObject(ctx, input)
	assert.NoError(t, err)

	return awss3.Key(key)
}

func decodeLastGlobalLogEntry(t *testing.T, raw string) map[string]any {
	t.Helper()

	lines := strings.Split(strings.TrimSpace(raw), "\n")
	if len(lines) == 0 {
		t.Fatal("expected at least one log line")
	}

	entry := make(map[string]any)
	err := json.Unmarshal([]byte(lines[len(lines)-1]), &entry)
	assert.NoError(t, err)

	return entry
}
