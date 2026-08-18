package awsdynamo_test

import (
	"context"
	"testing"

	"go.opentelemetry.io/otel/trace/noop"

	"github.com/88labs/go-utils/aws/awsconfig"
	"github.com/88labs/go-utils/aws/awsdynamo"
	"github.com/88labs/go-utils/aws/ctxawslocal"
)

func TestNewClient_WithTraceAddsSDKMiddleware(t *testing.T) {
	ctx := ctxawslocal.WithContext(
		context.Background(),
		ctxawslocal.WithDynamoEndpoint("http://127.0.0.1:28000"),
		ctxawslocal.WithAccessKey("test"),
		ctxawslocal.WithSecretAccessKey("test"),
	)

	client, err := awsdynamo.NewClient(ctx, awsconfig.RegionTokyo, awsdynamo.WithTrace(noop.NewTracerProvider()))
	if err != nil {
		t.Fatal(err)
	}
	if len(client.DynamoDBClient().Options().APIOptions) == 0 {
		t.Fatal("expected trace middleware to be added")
	}
}
