package awssqs_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/DataDog/dd-trace-go/v2/ddtrace/ext"
	ddmocktracer "github.com/DataDog/dd-trace-go/v2/ddtrace/mocktracer"
	"github.com/DataDog/dd-trace-go/v2/ddtrace/tracer"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"

	"github.com/88labs/go-utils/aws/awsconfig"
	"github.com/88labs/go-utils/aws/awssqs"
	"github.com/88labs/go-utils/aws/ctxawslocal"
)

func TestNewClient_WithTraceAddsSDKMiddleware(t *testing.T) {
	ctx := ctxawslocal.WithContext(
		context.Background(),
		ctxawslocal.WithSQSEndpoint("http://127.0.0.1:29324"),
		ctxawslocal.WithAccessKey("test"),
		ctxawslocal.WithSecretAccessKey("test"),
	)

	client, err := awssqs.NewClient(ctx, awsconfig.RegionTokyo, awssqs.WithTrace(awssqs.TraceConfig{
		TracerProvider: noop.NewTracerProvider(),
	}))
	if err != nil {
		t.Fatal(err)
	}
	if len(client.SQSClient().Options().APIOptions) == 0 {
		t.Fatal("expected trace middleware to be added")
	}
}

func TestNewClient_PropagatesDatadogParentToRequest(t *testing.T) {
	var requestHeaders http.Header
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestHeaders = r.Header.Clone()
		w.Header().Set("Content-Type", "application/x-amz-json-1.0")
		_, _ = w.Write([]byte(`{"MD5OfMessageBody":"78e731027d8fd50ed642340b7c9a63b3","MessageId":"message-id"}`))
	}))
	t.Cleanup(server.Close)

	spanRecorder := tracetest.NewSpanRecorder()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(spanRecorder))
	t.Cleanup(func() { _ = provider.Shutdown(context.Background()) })
	previousPropagator := otel.GetTextMapPropagator()
	otel.SetTextMapPropagator(propagation.TraceContext{})
	t.Cleanup(func() { otel.SetTextMapPropagator(previousPropagator) })
	ddMockTracer := ddmocktracer.Start()
	t.Cleanup(ddMockTracer.Stop)
	parent, ctx := tracer.StartSpanFromContext(
		context.Background(),
		"parent",
		tracer.WithSpanID(0x1234),
		tracer.Tag(ext.ManualKeep, true),
	)
	t.Cleanup(func() { parent.Finish() })
	ctx = ctxawslocal.WithContext(
		ctx,
		ctxawslocal.WithSQSEndpoint(server.URL),
		ctxawslocal.WithAccessKey("test"),
		ctxawslocal.WithSecretAccessKey("test"),
	)

	client, err := awssqs.NewClient(ctx, awsconfig.RegionTokyo, awssqs.WithTrace(awssqs.TraceConfig{
		TracerProvider: provider,
	}))
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.SQSClient().SendMessage(ctx, &sqs.SendMessageInput{
		QueueUrl:    aws.String(server.URL),
		MessageBody: aws.String("message"),
	})
	if err != nil {
		t.Fatal(err)
	}

	spans := spanRecorder.Ended()
	if len(spans) != 1 {
		t.Fatalf("ended spans = %d, want 1", len(spans))
	}
	awsSpan := spans[0]
	if got, want := awsSpan.Parent().TraceID(), trace.TraceID(parent.Context().TraceIDBytes()); got != want {
		t.Fatalf("AWS parent trace ID = %s, want %s", got, want)
	}
	if got, want := awsSpan.Parent().SpanID().String(), "0000000000001234"; got != want {
		t.Fatalf("AWS parent span ID = %s, want %s", got, want)
	}
	if !awsSpan.Parent().IsRemote() {
		t.Fatal("expected the Datadog parent to be marked remote")
	}
	if !awsSpan.Parent().IsSampled() {
		t.Fatal("expected the Datadog sampling decision to be preserved")
	}

	propagated := propagation.TraceContext{}.Extract(
		context.Background(),
		propagation.HeaderCarrier(requestHeaders),
	)
	propagatedSpanContext := trace.SpanContextFromContext(propagated)
	if !propagatedSpanContext.IsValid() {
		t.Fatal("expected a valid traceparent header")
	}
	if !propagatedSpanContext.IsSampled() {
		t.Fatal("expected the request traceparent to be sampled")
	}
	if got, want := propagatedSpanContext.TraceID(), awsSpan.SpanContext().TraceID(); got != want {
		t.Fatalf("request trace ID = %s, want AWS span trace ID %s", got, want)
	}
	if got, want := propagatedSpanContext.SpanID(), awsSpan.SpanContext().SpanID(); got != want {
		t.Fatalf("request span ID = %s, want AWS span span ID %s", got, want)
	}
}
