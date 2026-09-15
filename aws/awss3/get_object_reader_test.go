package awss3_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"gotest.tools/v3/assert"

	"github.com/88labs/go-utils/aws/awss3"
	"github.com/88labs/go-utils/aws/ctxawslocal"
)

func TestClient_GetObjectReader_usesSingleGetRequest(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)
		if r.Method != http.MethodGet || r.URL.Path != "/test/path/to/object.csv" {
			http.Error(w, "unexpected request", http.StatusBadRequest)
			return
		}
		_, _ = io.WriteString(w, "header\nvalue\n")
	}))
	t.Cleanup(server.Close)

	ctx := ctxawslocal.WithContext(
		context.Background(),
		ctxawslocal.WithS3Endpoint(server.URL),
		ctxawslocal.WithAccessKey("DUMMYACCESSKEYEXAMPLE"),
		ctxawslocal.WithSecretAccessKey("DUMMYSECRETKEYEXAMPLE"),
	)
	client, err := awss3.NewClient(ctx, TestRegion)
	assert.NilError(t, err)

	reader, err := client.GetObjectReader(ctx, TestBucket, "path/to/object.csv")
	assert.NilError(t, err)
	body, err := io.ReadAll(reader)
	assert.NilError(t, err)
	assert.Equal(t, "header\nvalue\n", string(body))
	assert.NilError(t, reader.Close())
	assert.Equal(t, int32(1), requestCount.Load())
}

func TestClient_GetObjectReader_notFound(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "unexpected request", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/xml")
		w.WriteHeader(http.StatusNotFound)
		_, _ = io.WriteString(w, `<Error><Code>NoSuchKey</Code><Message>not found</Message></Error>`)
	}))
	t.Cleanup(server.Close)

	ctx := ctxawslocal.WithContext(
		context.Background(),
		ctxawslocal.WithS3Endpoint(server.URL),
		ctxawslocal.WithAccessKey("DUMMYACCESSKEYEXAMPLE"),
		ctxawslocal.WithSecretAccessKey("DUMMYSECRETKEYEXAMPLE"),
	)
	client, err := awss3.NewClient(ctx, TestRegion)
	assert.NilError(t, err)

	reader, err := client.GetObjectReader(ctx, TestBucket, "missing.csv")
	assert.Assert(t, reader == nil)
	assert.ErrorIs(t, err, awss3.ErrNotFound)
}
