package awss3_test

import (
	"context"
	"testing"

	"go.opentelemetry.io/otel/trace/noop"

	"github.com/88labs/go-utils/aws/awsconfig"
	"github.com/88labs/go-utils/aws/awss3"
	"github.com/88labs/go-utils/aws/ctxawslocal"
)

func TestNewClient_WithTraceAddsSDKMiddleware(t *testing.T) {
	ctx := ctxawslocal.WithContext(
		context.Background(),
		ctxawslocal.WithS3Endpoint("http://127.0.0.1:29000"),
		ctxawslocal.WithAccessKey("test"),
		ctxawslocal.WithSecretAccessKey("test"),
	)

	client, err := awss3.NewClient(ctx, awsconfig.RegionTokyo, awss3.WithTrace(noop.NewTracerProvider()))
	if err != nil {
		t.Fatal(err)
	}
	if len(client.S3Client().Options().APIOptions) == 0 {
		t.Fatal("expected trace middleware to be added")
	}
}
