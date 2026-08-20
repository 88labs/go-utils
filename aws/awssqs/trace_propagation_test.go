package awssqs_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	oteltrace "go.opentelemetry.io/otel/trace"

	"github.com/88labs/go-utils/aws/awsconfig"
	"github.com/88labs/go-utils/aws/awssqs"
	"github.com/88labs/go-utils/aws/awssqs/options/sqssend"
	"github.com/88labs/go-utils/aws/ctxawslocal"
)

func TestSendMessageWithTraceInjectsMessageAttributes(t *testing.T) {
	var requestBody []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestBody, _ = io.ReadAll(r.Body)
		writeSQSResponse(w, `{"MessageId":"message-id"}`)
	}))
	t.Cleanup(server.Close)

	provider, recorder := newTraceProvider(t)
	restorePropagator := setTraceContextPropagator()
	t.Cleanup(restorePropagator)

	parentCtx, parentSpan := provider.Tracer("test").Start(context.Background(), "parent")
	t.Cleanup(func() { parentSpan.End() })
	ctx := localSQSContext(parentCtx, server.URL)
	client, err := awssqs.NewClient(ctx, awsconfig.RegionTokyo, awssqs.WithTrace(provider))
	require.NoError(t, err)

	attributes := map[string]types.MessageAttributeValue{
		"business": stringAttribute("value"),
	}
	_, err = client.SendMessage(
		ctx,
		awssqs.QueueURL(server.URL+"/000000000000/orders"),
		"message",
		sqssend.WithMessageAttributes(attributes),
	)
	require.NoError(t, err)

	var request struct {
		MessageAttributes map[string]types.MessageAttributeValue `json:"MessageAttributes"`
	}
	require.NoError(t, json.Unmarshal(requestBody, &request))
	require.Len(t, request.MessageAttributes, 3)
	require.Equal(t, "value", *request.MessageAttributes["business"].StringValue)
	require.Equal(t, "String", *request.MessageAttributes["traceparent"].DataType)
	require.Equal(t, "String", *request.MessageAttributes["tracestate"].DataType)
	require.Len(t, attributes, 1)
	_, hasTraceParent := attributes["traceparent"]
	require.False(t, hasTraceParent)

	traceParent := *request.MessageAttributes["traceparent"].StringValue
	propagated := propagation.TraceContext{}.Extract(
		context.Background(),
		propagation.MapCarrier{"traceparent": traceParent},
	)
	propagatedSpanContext := oteltrace.SpanContextFromContext(propagated)
	require.True(t, propagatedSpanContext.IsValid())

	sendSpan := endedSpan(t, recorder, "send orders")
	require.Equal(t, parentSpan.SpanContext().TraceID(), sendSpan.Parent().TraceID())
	require.Equal(t, propagatedSpanContext.SpanID(), sendSpan.SpanContext().SpanID())
	require.Equal(t, propagatedSpanContext.TraceID(), sendSpan.SpanContext().TraceID())
	require.Equal(t, "aws_sqs", spanAttribute(sendSpan, "messaging.system"))
	require.Equal(t, "send", spanAttribute(sendSpan, "messaging.operation.name"))
	require.Equal(t, "send", spanAttribute(sendSpan, "messaging.operation.type"))
	require.Equal(t, "orders", spanAttribute(sendSpan, "messaging.destination.name"))
}

func TestSendMessageWithoutTraceDoesNotInjectMessageAttributes(t *testing.T) {
	var requestBody []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestBody, _ = io.ReadAll(r.Body)
		writeSQSResponse(w, `{"MessageId":"message-id"}`)
	}))
	t.Cleanup(server.Close)

	ctx := localSQSContext(context.Background(), server.URL)
	client, err := awssqs.NewClient(ctx, awsconfig.RegionTokyo)
	require.NoError(t, err)
	_, err = client.SendMessage(
		ctx,
		awssqs.QueueURL(server.URL+"/000000000000/orders"),
		"message",
		sqssend.WithMessageAttributes(map[string]types.MessageAttributeValue{
			"business": stringAttribute("value"),
		}),
	)
	require.NoError(t, err)

	var request struct {
		MessageAttributes map[string]types.MessageAttributeValue `json:"MessageAttributes"`
	}
	require.NoError(t, json.Unmarshal(requestBody, &request))
	require.Len(t, request.MessageAttributes, 1)
	_, hasTraceParent := request.MessageAttributes["traceparent"]
	require.False(t, hasTraceParent)
}

func TestSendMessageBatchWithTraceUsesIndependentContexts(t *testing.T) {
	var requestBody []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestBody, _ = io.ReadAll(r.Body)
		writeSQSResponse(w, `{"Successful":[{"Id":"one","MessageId":"message-one"},{"Id":"two","MessageId":"message-two"}],"Failed":[]}`)
	}))
	t.Cleanup(server.Close)

	provider, recorder := newTraceProvider(t)
	restorePropagator := setTraceContextPropagator()
	t.Cleanup(restorePropagator)
	ctx := localSQSContext(context.Background(), server.URL)
	client, err := awssqs.NewClient(ctx, awsconfig.RegionTokyo, awssqs.WithTrace(provider))
	require.NoError(t, err)

	entries := []types.SendMessageBatchRequestEntry{
		{Id: aws.String("one"), MessageBody: aws.String("one")},
		{Id: aws.String("two"), MessageBody: aws.String("two")},
	}
	_, err = client.SendMessageBatch(ctx, awssqs.QueueURL(server.URL+"/000000000000/orders"), entries)
	require.NoError(t, err)

	var request struct {
		Entries []struct {
			MessageAttributes map[string]types.MessageAttributeValue `json:"MessageAttributes"`
		} `json:"Entries"`
	}
	require.NoError(t, json.Unmarshal(requestBody, &request))
	require.Len(t, request.Entries, 2)
	traceParents := make(map[string]struct{}, 2)
	for _, entry := range request.Entries {
		require.Len(t, entry.MessageAttributes, 2)
		traceParents[*entry.MessageAttributes["traceparent"].StringValue] = struct{}{}
	}
	require.Len(t, traceParents, 2)

	createSpans := spansNamed(recorder, "create orders")
	require.Len(t, createSpans, 2)
	sendSpan := endedSpan(t, recorder, "send orders")
	require.Equal(t, oteltrace.SpanKindClient, sendSpan.SpanKind())
	require.Len(t, sendSpan.Links(), 2)
	for _, createSpan := range createSpans {
		require.Contains(t, linkedSpanIDs(sendSpan), createSpan.SpanContext().SpanID())
	}
	require.Equal(t, "aws_sqs", spanAttribute(sendSpan, "messaging.system"))
	require.Equal(t, "send", spanAttribute(sendSpan, "messaging.operation.name"))
	require.Equal(t, "send", spanAttribute(sendSpan, "messaging.operation.type"))
	require.Equal(t, "orders", spanAttribute(sendSpan, "messaging.destination.name"))
	require.Equal(t, int64(2), spanIntAttribute(sendSpan, "messaging.batch.message_count"))
}

func TestSendMessageBatchWithTraceRecordsPartialFailures(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeSQSResponse(w, `{"Successful":[{"Id":"one","MessageId":"message-one"}],"Failed":[{"Id":"two","Code":"InvalidMessageContents","Message":"invalid body","SenderFault":true}]}`)
	}))
	t.Cleanup(server.Close)

	provider, recorder := newTraceProvider(t)
	restorePropagator := setTraceContextPropagator()
	t.Cleanup(restorePropagator)
	ctx := localSQSContext(context.Background(), server.URL)
	client, err := awssqs.NewClient(ctx, awsconfig.RegionTokyo, awssqs.WithTrace(provider))
	require.NoError(t, err)

	_, err = client.SendMessageBatch(ctx, awssqs.QueueURL(server.URL+"/000000000000/orders"), []types.SendMessageBatchRequestEntry{
		{Id: aws.String("one"), MessageBody: aws.String("one")},
		{Id: aws.String("two"), MessageBody: aws.String("two")},
	})
	require.NoError(t, err)

	sendSpan := endedSpan(t, recorder, "send orders")
	require.Equal(t, "Error", sendSpan.Status().Code.String())
	require.Contains(t, sendSpan.Status().Description, "1 batch message(s) failed")
	require.Contains(t, sendSpan.Status().Description, "id=two")
	require.Contains(t, sendSpan.Status().Description, "InvalidMessageContents")
	require.Contains(t, sendSpan.Status().Description, "invalid body")
}

func TestSendMessageWithTraceRejectsInvalidConfigurationBeforeSQS(t *testing.T) {
	var requestCount atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)
		writeSQSResponse(w, `{"MessageId":"message-id"}`)
	}))
	t.Cleanup(server.Close)

	provider, _ := newTraceProvider(t)
	previousPropagator := otel.GetTextMapPropagator()
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator())
	t.Cleanup(func() { otel.SetTextMapPropagator(previousPropagator) })
	ctx := localSQSContext(context.Background(), server.URL)
	client, err := awssqs.NewClient(ctx, awsconfig.RegionTokyo, awssqs.WithTrace(provider))
	require.NoError(t, err)

	_, err = client.SendMessage(ctx, awssqs.QueueURL(server.URL+"/000000000000/orders"), "message")
	require.ErrorIs(t, err, awssqs.ErrTracePropagatorNotConfigured)
	require.Zero(t, requestCount.Load())
}

func TestSendMessageWithTraceValidatesReservedAttributesAndLimit(t *testing.T) {
	var requestCount atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)
		writeSQSResponse(w, `{"MessageId":"message-id"}`)
	}))
	t.Cleanup(server.Close)

	provider, _ := newTraceProvider(t)
	restorePropagator := setTraceContextPropagator()
	t.Cleanup(restorePropagator)
	ctx := localSQSContext(context.Background(), server.URL)
	client, err := awssqs.NewClient(ctx, awsconfig.RegionTokyo, awssqs.WithTrace(provider))
	require.NoError(t, err)
	queueURL := awssqs.QueueURL(server.URL + "/000000000000/orders")

	_, err = client.SendMessage(ctx, queueURL, "message", sqssend.WithMessageAttributes(
		map[string]types.MessageAttributeValue{
			"traceparent": stringAttribute("caller-value"),
		},
	))
	require.ErrorIs(t, err, awssqs.ErrReservedMessageAttribute)

	tooMany := make(map[string]types.MessageAttributeValue, 9)
	for i := 0; i < 9; i++ {
		tooMany[string(rune('a'+i))] = stringAttribute("value")
	}
	_, err = client.SendMessage(ctx, queueURL, "message", sqssend.WithMessageAttributes(tooMany))
	require.ErrorIs(t, err, awssqs.ErrTooManyMessageAttributes)
	require.Zero(t, requestCount.Load())
}

func TestSendMessageWithTraceUsesCustomSpanName(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeSQSResponse(w, `{"MessageId":"message-id"}`)
	}))
	t.Cleanup(server.Close)

	provider, recorder := newTraceProvider(t)
	restorePropagator := setTraceContextPropagator()
	t.Cleanup(restorePropagator)
	ctx := localSQSContext(context.Background(), server.URL)
	client, err := awssqs.NewClient(
		ctx,
		awsconfig.RegionTokyo,
		awssqs.WithTrace(provider),
		awssqs.WithSendSpanName("custom send"),
	)
	require.NoError(t, err)
	_, err = client.SendMessage(ctx, awssqs.QueueURL(server.URL+"/000000000000/orders"), "message")
	require.NoError(t, err)
	require.Len(t, spansNamed(recorder, "custom send"), 1)
}

func TestProcessMessageUsesWorkerParentAndSenderLink(t *testing.T) {
	var deleteCount atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Amz-Target") == "AmazonSQS.DeleteMessage" {
			deleteCount.Add(1)
		}
		writeSQSResponse(w, `{}`)
	}))
	t.Cleanup(server.Close)

	provider, recorder := newTraceProvider(t)
	restorePropagator := setTraceContextPropagator()
	t.Cleanup(restorePropagator)
	sourceCtx, sourceSpan := provider.Tracer("source").Start(context.Background(), "source")
	t.Cleanup(func() { sourceSpan.End() })
	sourceCarrier := propagation.MapCarrier{}
	propagation.TraceContext{}.Inject(sourceCtx, sourceCarrier)
	workerCtx, workerSpan := provider.Tracer("worker").Start(context.Background(), "worker")
	t.Cleanup(func() { workerSpan.End() })

	ctx := localSQSContext(workerCtx, server.URL)
	client, err := awssqs.NewClient(ctx, awsconfig.RegionTokyo, awssqs.WithTrace(provider))
	require.NoError(t, err)
	message := types.Message{
		ReceiptHandle: aws.String("receipt"),
		MessageAttributes: map[string]types.MessageAttributeValue{
			"traceparent": stringAttribute(sourceCarrier.Get("traceparent")),
		},
	}
	var handlerSpanContext oteltrace.SpanContext
	err = client.ProcessMessage(
		ctx,
		awssqs.QueueURL(server.URL+"/000000000000/orders"),
		message,
		func(handlerCtx context.Context, _ types.Message) error {
			handlerSpanContext = oteltrace.SpanContextFromContext(handlerCtx)
			return nil
		},
	)
	require.NoError(t, err)
	require.Equal(t, int32(1), deleteCount.Load())
	require.Equal(t, "process orders", endedSpan(t, recorder, "process orders").Name())
	processSpan := endedSpan(t, recorder, "process orders")
	require.Equal(t, workerSpan.SpanContext().SpanID(), processSpan.Parent().SpanID())
	require.Len(t, processSpan.Links(), 1)
	require.Equal(t, sourceSpan.SpanContext().TraceID(), processSpan.Links()[0].SpanContext.TraceID())
	require.Equal(t, sourceSpan.SpanContext().SpanID(), processSpan.Links()[0].SpanContext.SpanID())
	require.True(t, processSpan.Links()[0].SpanContext.IsRemote())
	require.Equal(t, processSpan.SpanContext(), endedSpan(t, recorder, "delete orders").Parent())
	require.Equal(t, processSpan.SpanContext(), handlerSpanContext)
	require.Equal(t, "aws_sqs", spanAttribute(processSpan, "messaging.system"))
	require.Equal(t, "process", spanAttribute(processSpan, "messaging.operation.name"))
	require.Equal(t, "process", spanAttribute(processSpan, "messaging.operation.type"))
	require.Equal(t, "orders", spanAttribute(processSpan, "messaging.destination.name"))
	require.Equal(t, "aws_sqs", spanAttribute(endedSpan(t, recorder, "delete orders"), "messaging.system"))
	require.Equal(t, "settle", spanAttribute(endedSpan(t, recorder, "delete orders"), "messaging.operation.type"))
}

func TestProcessMessageInvalidTraceContextContinuesProcessing(t *testing.T) {
	var deleteCount atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Amz-Target") == "AmazonSQS.DeleteMessage" {
			deleteCount.Add(1)
		}
		writeSQSResponse(w, `{}`)
	}))
	t.Cleanup(server.Close)

	provider, recorder := newTraceProvider(t)
	restorePropagator := setTraceContextPropagator()
	t.Cleanup(restorePropagator)
	workerCtx, workerSpan := provider.Tracer("worker").Start(context.Background(), "worker")
	t.Cleanup(func() { workerSpan.End() })
	ctx := localSQSContext(workerCtx, server.URL)
	client, err := awssqs.NewClient(ctx, awsconfig.RegionTokyo, awssqs.WithTrace(provider))
	require.NoError(t, err)

	var handlerSpanContext oteltrace.SpanContext
	err = client.ProcessMessage(
		ctx,
		awssqs.QueueURL(server.URL+"/000000000000/orders"),
		types.Message{
			ReceiptHandle: aws.String("receipt"),
			MessageAttributes: map[string]types.MessageAttributeValue{
				"traceparent": stringAttribute("00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01"),
				"tracestate":  stringAttribute("invalid"),
			},
		},
		func(handlerCtx context.Context, _ types.Message) error {
			handlerSpanContext = oteltrace.SpanContextFromContext(handlerCtx)
			return nil
		},
	)
	require.NoError(t, err)
	require.Equal(t, int32(1), deleteCount.Load())

	processSpan := endedSpan(t, recorder, "process orders")
	require.Equal(t, workerSpan.SpanContext().SpanID(), processSpan.Parent().SpanID())
	require.Empty(t, processSpan.Links())
	require.Equal(t, processSpan.SpanContext(), handlerSpanContext)
	require.Equal(t, "Error", processSpan.Status().Code.String())
}

func TestProcessMessageHandlerErrorDoesNotDelete(t *testing.T) {
	var requestCount atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)
		writeSQSResponse(w, `{}`)
	}))
	t.Cleanup(server.Close)

	provider, recorder := newTraceProvider(t)
	restorePropagator := setTraceContextPropagator()
	t.Cleanup(restorePropagator)
	ctx := localSQSContext(context.Background(), server.URL)
	client, err := awssqs.NewClient(ctx, awsconfig.RegionTokyo, awssqs.WithTrace(provider))
	require.NoError(t, err)
	handlerErr := errors.New("handler failed")
	err = client.ProcessMessage(
		ctx,
		awssqs.QueueURL(server.URL+"/000000000000/orders"),
		types.Message{ReceiptHandle: aws.String("receipt")},
		func(context.Context, types.Message) error { return handlerErr },
	)
	require.ErrorIs(t, err, handlerErr)
	require.Zero(t, requestCount.Load())
	require.Equal(t, "Error", endedSpan(t, recorder, "process orders").Status().Code.String())
}

func newTraceProvider(t *testing.T) (*sdktrace.TracerProvider, *tracetest.SpanRecorder) {
	t.Helper()
	recorder := tracetest.NewSpanRecorder()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))
	t.Cleanup(func() { _ = provider.Shutdown(context.Background()) })
	return provider, recorder
}

func setTraceContextPropagator() func() {
	previous := otel.GetTextMapPropagator()
	otel.SetTextMapPropagator(propagation.TraceContext{})
	return func() { otel.SetTextMapPropagator(previous) }
}

func localSQSContext(parent context.Context, endpoint string) context.Context {
	return ctxawslocal.WithContext(
		parent,
		ctxawslocal.WithSQSEndpoint(endpoint),
		ctxawslocal.WithAccessKey("test"),
		ctxawslocal.WithSecretAccessKey("test"),
	)
}

func stringAttribute(value string) types.MessageAttributeValue {
	return types.MessageAttributeValue{
		DataType:    aws.String("String"),
		StringValue: aws.String(value),
	}
}

func writeSQSResponse(w http.ResponseWriter, body string) {
	w.Header().Set("Content-Type", "application/x-amz-json-1.0")
	_, _ = w.Write([]byte(body))
}

func endedSpan(t *testing.T, recorder *tracetest.SpanRecorder, name string) sdktrace.ReadOnlySpan {
	t.Helper()
	spans := spansNamed(recorder, name)
	require.NotEmpty(t, spans)
	return spans[len(spans)-1]
}

func spansNamed(recorder *tracetest.SpanRecorder, name string) []sdktrace.ReadOnlySpan {
	var matches []sdktrace.ReadOnlySpan
	for _, span := range recorder.Ended() {
		if span.Name() == name {
			matches = append(matches, span)
		}
	}
	return matches
}

func linkedSpanIDs(span sdktrace.ReadOnlySpan) []oteltrace.SpanID {
	ids := make([]oteltrace.SpanID, 0, len(span.Links()))
	for _, link := range span.Links() {
		ids = append(ids, link.SpanContext.SpanID())
	}
	return ids
}

func spanAttribute(span sdktrace.ReadOnlySpan, key string) string {
	for _, attribute := range span.Attributes() {
		if string(attribute.Key) == key {
			return attribute.Value.AsString()
		}
	}
	return ""
}

func spanIntAttribute(span sdktrace.ReadOnlySpan, key string) int64 {
	for _, attribute := range span.Attributes() {
		if string(attribute.Key) == key {
			return attribute.Value.AsInt64()
		}
	}
	return 0
}
