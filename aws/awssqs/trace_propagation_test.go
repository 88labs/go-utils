package awssqs_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/baggage"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	oteltrace "go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"

	"github.com/88labs/go-utils/aws/awsconfig"
	"github.com/88labs/go-utils/aws/awssqs"
	"github.com/88labs/go-utils/aws/awssqs/options/sqsprocess"
	"github.com/88labs/go-utils/aws/awssqs/options/sqsreceive"
	"github.com/88labs/go-utils/aws/awssqs/options/sqssend"
	"github.com/88labs/go-utils/aws/ctxawslocal"
	commontracers "github.com/88labs/go-utils/tracers"
)

func TestSendMessageWithTraceInjectsMessageAttributes(t *testing.T) {
	var requestBody []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestBody, _ = io.ReadAll(r.Body)
		writeSQSResponse(w, `{"MessageId":"message-id"}`)
	}))
	t.Cleanup(server.Close)

	provider, recorder := newTraceProvider(t)
	parentCtx, parentSpan := provider.Tracer("test").Start(context.Background(), "parent")
	t.Cleanup(func() { parentSpan.End() })
	ctx := localSQSContext(parentCtx, server.URL)
	client, err := awssqs.NewClient(ctx, awsconfig.RegionTokyo, awssqs.WithTrace(awssqs.TraceConfig{
		TracerProvider: provider,
		Propagator:     propagation.Baggage{},
	}))
	assert.NilError(t, err)

	attributes := map[string]types.MessageAttributeValue{
		"business": stringAttribute("value"),
	}
	_, err = client.SendMessage(
		ctx,
		awssqs.QueueURL(server.URL+"/000000000000/orders"),
		"message",
		sqssend.WithMessageAttributes(attributes),
	)
	assert.NilError(t, err)

	var request struct {
		MessageAttributes map[string]types.MessageAttributeValue `json:"MessageAttributes"`
	}
	assert.NilError(t, json.Unmarshal(requestBody, &request))
	assert.Equal(t, len(request.MessageAttributes), 2)
	assert.Equal(t, *request.MessageAttributes["business"].StringValue, "value")
	assert.Equal(t, *request.MessageAttributes["traceparent"].DataType, "String")
	_, hasTraceState := request.MessageAttributes["tracestate"]
	assert.Assert(t, !hasTraceState)
	assert.Equal(t, len(attributes), 1)
	_, hasTraceParent := attributes["traceparent"]
	assert.Assert(t, !hasTraceParent)

	traceParent := *request.MessageAttributes["traceparent"].StringValue
	propagated := propagation.TraceContext{}.Extract(
		context.Background(),
		propagation.MapCarrier{"traceparent": traceParent},
	)
	propagatedSpanContext := oteltrace.SpanContextFromContext(propagated)
	assert.Assert(t, propagatedSpanContext.IsValid())

	sendSpan := endedSpan(t, recorder, "send orders")
	assert.Equal(t, sendSpan.Parent().TraceID(), parentSpan.SpanContext().TraceID())
	assert.Equal(t, sendSpan.SpanContext().SpanID(), propagatedSpanContext.SpanID())
	assert.Equal(t, sendSpan.SpanContext().TraceID(), propagatedSpanContext.TraceID())
	assert.Equal(t, spanAttribute(sendSpan, "messaging.system"), "aws_sqs")
	assert.Equal(t, spanAttribute(sendSpan, "messaging.operation.name"), "send")
	assert.Equal(t, spanAttribute(sendSpan, "messaging.operation.type"), "send")
	assert.Equal(t, spanAttribute(sendSpan, "messaging.destination.name"), "orders")
}

func TestSendReceiveProcessMessageWithTrace(t *testing.T) {
	const sqsEndpoint = "http://127.0.0.1:29324"
	ctx := localSQSContext(context.Background(), sqsEndpoint)
	provider, recorder := newTraceProvider(t)
	client, err := awssqs.NewClient(ctx, awsconfig.RegionTokyo, awssqs.WithTrace(awssqs.TraceConfig{
		TracerProvider: provider,
	}))
	if err != nil {
		t.Fatal(err)
	}

	queueName := fmt.Sprintf("trace-propagation-%d", time.Now().UnixNano())
	queueOutput, err := client.SQSClient().CreateQueue(ctx, &sqs.CreateQueueInput{
		QueueName: aws.String(queueName),
	})
	if err != nil {
		t.Fatal(err)
	}
	if queueOutput == nil || queueOutput.QueueUrl == nil {
		t.Fatal("CreateQueue returned no queue URL")
	}
	queueURLValue, err := url.Parse(*queueOutput.QueueUrl)
	if err != nil {
		t.Fatal(err)
	}
	endpointURL, err := url.Parse(sqsEndpoint)
	if err != nil {
		t.Fatal(err)
	}
	queueURLValue.Scheme = endpointURL.Scheme
	queueURLValue.Host = endpointURL.Host
	queueURL := awssqs.QueueURL(queueURLValue.String())
	t.Cleanup(func() {
		_, _ = client.SQSClient().DeleteQueue(context.Background(), &sqs.DeleteQueueInput{
			QueueUrl: queueURL.AWSString(),
		})
	})

	apiCtx, apiSpan := provider.Tracer("test").Start(context.Background(), "api")
	defer apiSpan.End()
	_, err = client.SendMessage(apiCtx, queueURL, "message")
	if err != nil {
		t.Fatal(err)
	}

	workerCtx, workerSpan := provider.Tracer("test").Start(context.Background(), "worker")
	defer workerSpan.End()
	received, err := client.ReceiveMessage(workerCtx, queueURL, sqsreceive.WithWaitTimeSeconds(0))
	if err != nil {
		t.Fatal(err)
	}
	if received == nil {
		t.Fatal("ReceiveMessage returned no response")
	}
	if len(received.Messages) != 1 {
		t.Fatalf("received messages = %d, want 1", len(received.Messages))
	}
	message := received.Messages[0]
	traceParent, ok := message.MessageAttributes["traceparent"]
	if !ok {
		t.Fatal("received message has no traceparent attribute")
	}
	if traceParent.StringValue == nil {
		t.Fatal("received traceparent attribute has no string value")
	}

	var handlerSpanContext oteltrace.SpanContext
	err = client.ProcessMessage(workerCtx, queueURL, message, func(handlerCtx context.Context, _ types.Message) error {
		handlerSpanContext = oteltrace.SpanFromContext(handlerCtx).SpanContext()
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	sendSpans := spansNamed(recorder, "send "+queueName)
	if len(sendSpans) != 1 {
		t.Fatalf("send spans = %d, want 1", len(sendSpans))
	}
	sendSpan := sendSpans[0]
	processSpans := spansNamed(recorder, "process "+queueName)
	if len(processSpans) != 1 {
		t.Fatalf("process spans = %d, want 1", len(processSpans))
	}
	processSpan := processSpans[0]
	if got, want := sendSpan.Parent().TraceID(), apiSpan.SpanContext().TraceID(); got != want {
		t.Fatalf("send parent trace ID = %s, want %s", got, want)
	}
	if !workerSpan.SpanContext().Equal(processSpan.Parent()) {
		t.Fatalf("process parent = %v, want worker span %v", processSpan.Parent(), workerSpan.SpanContext())
	}
	if !processSpan.SpanContext().Equal(handlerSpanContext) {
		t.Fatalf("handler span = %v, want process span %v", handlerSpanContext, processSpan.SpanContext())
	}
	if len(processSpan.Links()) != 1 {
		t.Fatalf("process links = %d, want 1", len(processSpan.Links()))
	}
	senderLink := processSpan.Links()[0].SpanContext
	if got, want := senderLink.TraceID(), sendSpan.SpanContext().TraceID(); got != want {
		t.Fatalf("sender link trace ID = %s, want %s", got, want)
	}
	if got, want := senderLink.SpanID(), sendSpan.SpanContext().SpanID(); got != want {
		t.Fatalf("sender link span ID = %s, want %s", got, want)
	}
	if !senderLink.IsRemote() {
		t.Fatal("sender link is not remote")
	}
	if deleteSpans := spansNamed(recorder, "delete "+queueName); len(deleteSpans) != 1 {
		t.Fatalf("delete spans = %d, want 1", len(deleteSpans))
	}
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
	assert.NilError(t, err)
	_, err = client.SendMessage(
		ctx,
		awssqs.QueueURL(server.URL+"/000000000000/orders"),
		"message",
		sqssend.WithMessageAttributes(map[string]types.MessageAttributeValue{
			"business": stringAttribute("value"),
		}),
	)
	assert.NilError(t, err)

	var request struct {
		MessageAttributes map[string]types.MessageAttributeValue `json:"MessageAttributes"`
	}
	assert.NilError(t, json.Unmarshal(requestBody, &request))
	assert.Equal(t, len(request.MessageAttributes), 1)
	_, hasTraceParent := request.MessageAttributes["traceparent"]
	assert.Assert(t, !hasTraceParent)
}

func TestSendMessageBatchWithTraceUsesIndependentContexts(t *testing.T) {
	var requestBody []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestBody, _ = io.ReadAll(r.Body)
		writeSQSResponse(w, `{"Successful":[{"Id":"one","MessageId":"message-one"},{"Id":"two","MessageId":"message-two"}],"Failed":[]}`)
	}))
	t.Cleanup(server.Close)

	provider, recorder := newTraceProvider(t)
	ctx := localSQSContext(context.Background(), server.URL)
	client, err := awssqs.NewClient(ctx, awsconfig.RegionTokyo, awssqs.WithTrace(awssqs.TraceConfig{
		TracerProvider: provider,
		Propagator:     propagation.Baggage{},
	}))
	assert.NilError(t, err)

	entries := []types.SendMessageBatchRequestEntry{
		{Id: aws.String("one"), MessageBody: aws.String("one")},
		{Id: aws.String("two"), MessageBody: aws.String("two")},
	}
	_, err = client.SendMessageBatch(
		ctx,
		awssqs.QueueURL(server.URL+"/000000000000/orders"),
		entries,
		sqssend.WithOperationName("publish"),
	)
	assert.NilError(t, err)

	var request struct {
		Entries []struct {
			MessageAttributes map[string]types.MessageAttributeValue `json:"MessageAttributes"`
		} `json:"Entries"`
	}
	assert.NilError(t, json.Unmarshal(requestBody, &request))
	assert.Equal(t, len(request.Entries), 2)
	traceParents := make(map[string]struct{}, 2)
	for _, entry := range request.Entries {
		assert.Equal(t, len(entry.MessageAttributes), 1)
		traceParents[*entry.MessageAttributes["traceparent"].StringValue] = struct{}{}
	}
	assert.Equal(t, len(traceParents), 2)

	createSpans := spansNamed(recorder, "create orders")
	assert.Equal(t, len(createSpans), 2)
	sendSpan := endedSpan(t, recorder, "publish orders")
	assert.Equal(t, sendSpan.SpanKind(), oteltrace.SpanKindClient)
	assert.Equal(t, len(sendSpan.Links()), 2)
	for _, createSpan := range createSpans {
		assert.Assert(t, cmp.Contains(linkedSpanIDs(sendSpan), createSpan.SpanContext().SpanID()))
	}
	assert.Equal(t, spanAttribute(sendSpan, "messaging.system"), "aws_sqs")
	assert.Equal(t, spanAttribute(sendSpan, "messaging.operation.name"), "send")
	assert.Equal(t, spanAttribute(sendSpan, "messaging.operation.type"), "send")
	assert.Equal(t, spanAttribute(sendSpan, "messaging.destination.name"), "orders")
	assert.Equal(t, spanIntAttribute(sendSpan, "messaging.batch.message_count"), int64(2))
}

func TestSendMessageBatchWithTraceRecordsPartialFailures(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeSQSResponse(w, `{"Successful":[{"Id":"one","MessageId":"message-one"}],"Failed":[{"Id":"two","Code":"InvalidMessageContents","Message":"invalid body","SenderFault":true}]}`)
	}))
	t.Cleanup(server.Close)

	provider, recorder := newTraceProvider(t)
	ctx := localSQSContext(context.Background(), server.URL)
	client, err := awssqs.NewClient(ctx, awsconfig.RegionTokyo, awssqs.WithTrace(awssqs.TraceConfig{
		TracerProvider: provider,
		Propagator:     propagation.Baggage{},
	}))
	assert.NilError(t, err)

	_, err = client.SendMessageBatch(ctx, awssqs.QueueURL(server.URL+"/000000000000/orders"), []types.SendMessageBatchRequestEntry{
		{Id: aws.String("one"), MessageBody: aws.String("one")},
		{Id: aws.String("two"), MessageBody: aws.String("two")},
	})
	assert.NilError(t, err)

	sendSpan := endedSpan(t, recorder, "send orders")
	assert.Equal(t, sendSpan.Status().Code.String(), "Error")
	assert.Assert(t, cmp.Contains(sendSpan.Status().Description, "1 batch message(s) failed"))
	assert.Assert(t, cmp.Contains(sendSpan.Status().Description, "id=two"))
	assert.Assert(t, cmp.Contains(sendSpan.Status().Description, "InvalidMessageContents"))
	assert.Assert(t, cmp.Contains(sendSpan.Status().Description, "invalid body"))
}

func TestWithTraceUsesDefaultConfig(t *testing.T) {
	var requestBody []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestBody, _ = io.ReadAll(r.Body)
		writeSQSResponse(w, `{"MessageId":"message-id"}`)
	}))
	t.Cleanup(server.Close)

	provider, recorder := newTraceProvider(t)
	previousProvider := otel.GetTracerProvider()
	previousPropagator := otel.GetTextMapPropagator()
	otel.SetTracerProvider(provider)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator())
	t.Cleanup(func() {
		otel.SetTextMapPropagator(previousPropagator)
		otel.SetTracerProvider(previousProvider)
	})

	parentCtx, parentSpan := provider.Tracer("test").Start(context.Background(), "parent")
	t.Cleanup(func() { parentSpan.End() })
	member, err := baggage.NewMember("tenant", "board")
	assert.NilError(t, err)
	bag, err := baggage.New(member)
	assert.NilError(t, err)
	ctx := localSQSContext(baggage.ContextWithBaggage(parentCtx, bag), server.URL)
	client, err := awssqs.NewClient(ctx, awsconfig.RegionTokyo, awssqs.WithTrace(awssqs.TraceConfig{}))
	assert.NilError(t, err)

	_, err = client.SendMessage(ctx, awssqs.QueueURL(server.URL+"/000000000000/orders"), "message")
	assert.NilError(t, err)

	var request struct {
		MessageAttributes map[string]types.MessageAttributeValue `json:"MessageAttributes"`
	}
	assert.NilError(t, json.Unmarshal(requestBody, &request))
	assert.Assert(t, cmp.Contains(request.MessageAttributes, "traceparent"))
	_, hasBaggage := request.MessageAttributes["baggage"]
	assert.Assert(t, !hasBaggage)
	assert.Equal(t, len(request.MessageAttributes), 1)
	sendSpan := endedSpan(t, recorder, "send orders")
	assert.Equal(t, sendSpan.Parent().TraceID(), parentSpan.SpanContext().TraceID())
}

func TestSendMessageWithTraceValidatesReservedAttributesAndLimit(t *testing.T) {
	var requestCount atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)
		writeSQSResponse(w, `{"MessageId":"message-id"}`)
	}))
	t.Cleanup(server.Close)

	provider, _ := newTraceProvider(t)
	ctx := localSQSContext(context.Background(), server.URL)
	client, err := awssqs.NewClient(ctx, awsconfig.RegionTokyo, awssqs.WithTrace(awssqs.TraceConfig{
		TracerProvider: provider,
		Propagator:     propagation.Baggage{},
	}))
	assert.NilError(t, err)
	queueURL := awssqs.QueueURL(server.URL + "/000000000000/orders")

	_, err = client.SendMessage(ctx, queueURL, "message", sqssend.WithMessageAttributes(
		map[string]types.MessageAttributeValue{
			"traceparent": stringAttribute("caller-value"),
		},
	))
	assert.ErrorIs(t, err, awssqs.ErrReservedMessageAttribute)
	_, err = client.SendMessage(ctx, queueURL, "message", sqssend.WithMessageAttributes(
		map[string]types.MessageAttributeValue{
			"tracestate": stringAttribute("caller-value"),
		},
	))
	assert.ErrorIs(t, err, awssqs.ErrReservedMessageAttribute)

	tooMany := make(map[string]types.MessageAttributeValue, 8)
	for i := 0; i < 8; i++ {
		tooMany[string(rune('a'+i))] = stringAttribute("value")
	}
	_, err = client.SendMessage(ctx, queueURL, "message", sqssend.WithMessageAttributes(tooMany))
	assert.ErrorIs(t, err, awssqs.ErrTooManyMessageAttributes)
	assert.Equal(t, requestCount.Load(), int32(0))
}

func TestSendMessageWithNoopProviderAndParentReturnsError(t *testing.T) {
	var requestCount atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)
		writeSQSResponse(w, `{"MessageId":"message-id"}`)
	}))
	t.Cleanup(server.Close)

	parentProvider, _ := newTraceProvider(t)
	parentCtx, parentSpan := parentProvider.Tracer("parent").Start(context.Background(), "parent")
	t.Cleanup(func() { parentSpan.End() })
	ctx := localSQSContext(parentCtx, server.URL)
	client, err := awssqs.NewClient(ctx, awsconfig.RegionTokyo, awssqs.WithTrace(awssqs.TraceConfig{
		TracerProvider: noop.NewTracerProvider(),
	}))
	assert.NilError(t, err)

	_, err = client.SendMessage(ctx, awssqs.QueueURL(server.URL+"/000000000000/orders"), "message")
	assert.ErrorIs(t, err, awssqs.ErrTraceProviderNotConfigured)
	assert.Equal(t, requestCount.Load(), int32(0))
}

func TestSendMessageWithTraceUsesCustomSpanName(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeSQSResponse(w, `{"MessageId":"message-id"}`)
	}))
	t.Cleanup(server.Close)

	provider, recorder := newTraceProvider(t)
	ctx := localSQSContext(context.Background(), server.URL)
	client, err := awssqs.NewClient(
		ctx,
		awsconfig.RegionTokyo,
		awssqs.WithTrace(awssqs.TraceConfig{
			TracerProvider: provider,
			Propagator:     propagation.Baggage{},
		}),
	)
	assert.NilError(t, err)
	_, err = client.SendMessage(
		ctx,
		awssqs.QueueURL(server.URL+"/000000000000/orders"),
		"message",
		sqssend.WithOperationName("publish"),
	)
	assert.NilError(t, err)
	assert.Equal(t, len(spansNamed(recorder, "publish orders")), 1)

	_, err = client.SendMessage(
		ctx,
		awssqs.QueueURL(server.URL+"/000000000000/invoices"),
		"message",
	)
	assert.NilError(t, err)
	assert.Equal(t, len(spansNamed(recorder, "send invoices")), 1)
}

func TestProcessMessageOperationNameIsPerCallAndRetainsQueueName(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeSQSResponse(w, `{}`)
	}))
	t.Cleanup(server.Close)

	provider, recorder := newTraceProvider(t)
	ctx := localSQSContext(context.Background(), server.URL)
	client, err := awssqs.NewClient(ctx, awsconfig.RegionTokyo, awssqs.WithTrace(awssqs.TraceConfig{
		TracerProvider: provider,
		Propagator:     propagation.Baggage{},
	}))
	assert.NilError(t, err)

	for _, test := range []struct {
		queue    string
		spanName string
		opts     []sqsprocess.ProcessMessageOption
	}{
		{
			queue:    "orders",
			spanName: "consume orders",
			opts:     []sqsprocess.ProcessMessageOption{sqsprocess.WithOperationName("consume")},
		},
		{
			queue:    "invoices",
			spanName: "process invoices",
		},
	} {
		err = client.ProcessMessage(
			ctx,
			awssqs.QueueURL(server.URL+"/000000000000/"+test.queue),
			types.Message{ReceiptHandle: aws.String("receipt")},
			func(context.Context, types.Message) error { return nil },
			test.opts...,
		)
		assert.NilError(t, err)
		assert.Equal(t, len(spansNamed(recorder, test.spanName)), 1)
	}
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
	sourceCtx, sourceSpan := provider.Tracer("source").Start(context.Background(), "source")
	t.Cleanup(func() { sourceSpan.End() })
	sourceCarrier := propagation.MapCarrier{}
	propagation.TraceContext{}.Inject(sourceCtx, sourceCarrier)
	workerCtx, workerSpan := provider.Tracer("worker").Start(context.Background(), "worker")
	t.Cleanup(func() { workerSpan.End() })

	ctx := localSQSContext(workerCtx, server.URL)
	client, err := awssqs.NewClient(ctx, awsconfig.RegionTokyo, awssqs.WithTrace(awssqs.TraceConfig{
		TracerProvider: provider,
		Propagator:     propagation.Baggage{},
	}))
	assert.NilError(t, err)
	message := types.Message{
		ReceiptHandle: aws.String("receipt"),
		MessageAttributes: map[string]types.MessageAttributeValue{
			"traceparent": stringAttribute(sourceCarrier.Get("traceparent")),
		},
	}
	var handlerSpanContext oteltrace.SpanContext
	var messageTraceInfo commontracers.TraceInfo
	var hasMessageTraceInfo bool
	err = client.ProcessMessage(
		ctx,
		awssqs.QueueURL(server.URL+"/000000000000/orders"),
		message,
		func(handlerCtx context.Context, _ types.Message) error {
			handlerSpanContext = oteltrace.SpanContextFromContext(handlerCtx)
			messageTraceInfo, hasMessageTraceInfo = awssqs.ExtractMessageTraceContext(handlerCtx)
			return nil
		},
	)
	assert.NilError(t, err)
	assert.Equal(t, deleteCount.Load(), int32(1))
	assert.Equal(t, endedSpan(t, recorder, "process orders").Name(), "process orders")
	processSpan := endedSpan(t, recorder, "process orders")
	assert.Equal(t, processSpan.Parent().SpanID(), workerSpan.SpanContext().SpanID())
	assert.Equal(t, len(processSpan.Links()), 1)
	assert.Equal(t, processSpan.Links()[0].SpanContext.TraceID(), sourceSpan.SpanContext().TraceID())
	assert.Equal(t, processSpan.Links()[0].SpanContext.SpanID(), sourceSpan.SpanContext().SpanID())
	assert.Assert(t, processSpan.Links()[0].SpanContext.IsRemote())
	assert.Assert(t, endedSpan(t, recorder, "delete orders").Parent().Equal(processSpan.SpanContext()))
	assert.Assert(t, handlerSpanContext.Equal(processSpan.SpanContext()))
	assert.Assert(t, hasMessageTraceInfo)
	assert.Equal(t, messageTraceInfo.GetTraceParent(), sourceCarrier.Get("traceparent"))
	assert.Equal(t, messageTraceInfo.GetTraceState(), sourceCarrier.Get("tracestate"))
	assert.Equal(t, messageTraceInfo.GetTraceID(commontracers.FormatString), sourceSpan.SpanContext().TraceID().String())
	assert.Equal(t, messageTraceInfo.GetSpanID(), sourceSpan.SpanContext().SpanID().String())
	assert.Equal(t, spanAttribute(processSpan, "messaging.system"), "aws_sqs")
	assert.Equal(t, spanAttribute(processSpan, "messaging.operation.name"), "process")
	assert.Equal(t, spanAttribute(processSpan, "messaging.operation.type"), "process")
	assert.Equal(t, spanAttribute(processSpan, "messaging.destination.name"), "orders")
	assert.Equal(t, spanAttribute(endedSpan(t, recorder, "delete orders"), "messaging.system"), "aws_sqs")
	assert.Equal(t, spanAttribute(endedSpan(t, recorder, "delete orders"), "messaging.operation.type"), "settle")
}

func TestExtractMessageTraceContextOutsideProcessMessage(t *testing.T) {
	if _, ok := awssqs.ExtractMessageTraceContext(context.Background()); ok {
		t.Fatal("message trace context is unexpectedly available")
	}
	if _, ok := awssqs.ExtractMessageTraceContext(nil); ok {
		t.Fatal("message trace context is unexpectedly available for nil context")
	}
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
	workerCtx, workerSpan := provider.Tracer("worker").Start(context.Background(), "worker")
	t.Cleanup(func() { workerSpan.End() })
	ctx := localSQSContext(workerCtx, server.URL)
	client, err := awssqs.NewClient(ctx, awsconfig.RegionTokyo, awssqs.WithTrace(awssqs.TraceConfig{
		TracerProvider: provider,
		Propagator:     propagation.Baggage{},
	}))
	assert.NilError(t, err)

	var handlerSpanContext oteltrace.SpanContext
	var hasMessageTraceInfo bool
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
			_, hasMessageTraceInfo = awssqs.ExtractMessageTraceContext(handlerCtx)
			return nil
		},
	)
	assert.NilError(t, err)
	assert.Equal(t, deleteCount.Load(), int32(1))

	processSpan := endedSpan(t, recorder, "process orders")
	assert.Equal(t, processSpan.Parent().SpanID(), workerSpan.SpanContext().SpanID())
	assert.Equal(t, len(processSpan.Links()), 0)
	assert.Assert(t, handlerSpanContext.Equal(processSpan.SpanContext()))
	assert.Assert(t, !hasMessageTraceInfo)
	assert.Equal(t, processSpan.Status().Code.String(), "Error")
}

func TestProcessMessageHandlerErrorDoesNotDelete(t *testing.T) {
	var requestCount atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)
		writeSQSResponse(w, `{}`)
	}))
	t.Cleanup(server.Close)

	provider, recorder := newTraceProvider(t)
	ctx := localSQSContext(context.Background(), server.URL)
	client, err := awssqs.NewClient(ctx, awsconfig.RegionTokyo, awssqs.WithTrace(awssqs.TraceConfig{
		TracerProvider: provider,
		Propagator:     propagation.Baggage{},
	}))
	assert.NilError(t, err)
	handlerErr := errors.New("handler failed")
	err = client.ProcessMessage(
		ctx,
		awssqs.QueueURL(server.URL+"/000000000000/orders"),
		types.Message{ReceiptHandle: aws.String("receipt")},
		func(context.Context, types.Message) error { return handlerErr },
	)
	assert.ErrorIs(t, err, handlerErr)
	assert.Equal(t, requestCount.Load(), int32(0))
	assert.Equal(t, endedSpan(t, recorder, "process orders").Status().Code.String(), "Error")
}

func newTraceProvider(t *testing.T) (*sdktrace.TracerProvider, *tracetest.SpanRecorder) {
	t.Helper()
	recorder := tracetest.NewSpanRecorder()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))
	t.Cleanup(func() { _ = provider.Shutdown(context.Background()) })
	return provider, recorder
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
	_ = json.NewEncoder(w).Encode(json.RawMessage(body))
}

func endedSpan(t *testing.T, recorder *tracetest.SpanRecorder, name string) sdktrace.ReadOnlySpan {
	t.Helper()
	spans := spansNamed(recorder, name)
	assert.Assert(t, len(spans) != 0)
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
