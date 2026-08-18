package awscognito_test

import (
	"context"
	"testing"

	"go.opentelemetry.io/otel/trace/noop"

	"github.com/88labs/go-utils/aws/awscognito"
	"github.com/88labs/go-utils/aws/awsconfig"
	"github.com/88labs/go-utils/aws/ctxawslocal"
)

func TestNewClient_WithTraceAddsSDKMiddleware(t *testing.T) {
	ctx := ctxawslocal.WithContext(
		context.Background(),
		ctxawslocal.WithCognitoEndpoint("http://127.0.0.1:4566"),
		ctxawslocal.WithAccessKey("test"),
		ctxawslocal.WithSecretAccessKey("test"),
	)

	client, err := awscognito.NewClient(ctx, awsconfig.RegionTokyo, awscognito.WithTrace(noop.NewTracerProvider()))
	if err != nil {
		t.Fatal(err)
	}
	if len(client.CognitoClient().Options().APIOptions) == 0 {
		t.Fatal("expected trace middleware to be added")
	}
}
