package awssqs

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/gob"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"path"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/baggage"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	oteltrace "go.opentelemetry.io/otel/trace"

	"github.com/88labs/go-utils/aws/awssqs/options/sqsprocess"
	"github.com/88labs/go-utils/aws/awssqs/options/sqsreceive"
	"github.com/88labs/go-utils/aws/awssqs/options/sqssend"
	"github.com/88labs/go-utils/aws/internal/awstrace"
	"github.com/88labs/go-utils/tracers"
)

const (
	traceInstrumentationName    = "github.com/88labs/go-utils/aws/awssqs"
	traceParentMessageAttribute = "traceparent"
	traceStateMessageAttribute  = "tracestate"
	maxMessageAttributes        = 10
	defaultCreateSpanPrefix     = "create"
	defaultSendSpanPrefix       = "send"
	defaultProcessSpanPrefix    = "process"
	defaultDeleteSpanPrefix     = "delete"

	messagingSystem           = "aws_sqs"
	messagingOperationCreate  = "create"
	messagingOperationSend    = "send"
	messagingOperationProcess = "process"
	messagingOperationSettle  = "settle"
	messagingOperationDelete  = "delete"
)

var (
	// ErrTraceProviderNotConfigured indicates that tracing was enabled but no
	// provider could create a valid span context.
	ErrTraceProviderNotConfigured = errors.New("awssqs: trace provider is not configured")
	// ErrTracePropagatorNotConfigured indicates that the configured propagator
	// did not produce a valid W3C traceparent.
	ErrTracePropagatorNotConfigured = errors.New("awssqs: W3C trace context propagator is not configured")
	// ErrInvalidTraceContext indicates malformed trace context data.
	ErrInvalidTraceContext = errors.New("awssqs: invalid trace context")
	// ErrReservedMessageAttribute indicates that a reserved trace attribute was
	// supplied by the caller.
	ErrReservedMessageAttribute = errors.New("awssqs: reserved message attribute")
	// ErrTooManyMessageAttributes indicates that the SQS message attribute limit
	// would be exceeded.
	ErrTooManyMessageAttributes = errors.New("awssqs: too many message attributes")
	// ErrNilMessageHandler indicates that ProcessMessage received no handler.
	ErrNilMessageHandler = errors.New("awssqs: message handler is nil")
)

type extractedMessageContext struct {
	ctx              context.Context
	spanContext      oteltrace.SpanContext
	traceInfo        tracers.TraceInfo
	hasSourceContext bool
	extractionErr    error
}

// SendMessage converts a message to JSON and sends it to SQS.
// Default DelaySeconds=0.
func (c *Client) SendMessage(
	ctx context.Context, queueURL QueueURL, message any, opts ...sqssend.SendMessageOption,
) (*sqs.SendMessageOutput, error) {
	conf := sqssend.GetConf(opts...)
	sendCtx, span, err := c.startSpan(
		ctx,
		c.sendSpanName(queueURL, conf.OperationName),
		oteltrace.WithSpanKind(oteltrace.SpanKindProducer),
		oteltrace.WithAttributes(messagingSpanAttributes(queueURL, messagingOperationSend, messagingOperationSend, nil, nil)...),
	)
	if err != nil {
		return nil, err
	}
	if span != nil {
		defer span.End()
	}

	jsonb, err := json.Marshal(message)
	if err != nil {
		recordSpanError(span, err)
		return nil, err
	}
	return c.sendMessageBody(sendCtx, queueURL, string(jsonb), conf.DelaySeconds, conf.MessageAttributes, span)
}

// SendMessageGob converts a message to gob encoding and sends it to SQS.
// Default DelaySeconds=0.
func (c *Client) SendMessageGob(
	ctx context.Context, queueURL QueueURL, message any, opts ...sqssend.SendMessageOption,
) (*sqs.SendMessageOutput, error) {
	conf := sqssend.GetConf(opts...)
	sendCtx, span, err := c.startSpan(
		ctx,
		c.sendSpanName(queueURL, conf.OperationName),
		oteltrace.WithSpanKind(oteltrace.SpanKindProducer),
		oteltrace.WithAttributes(messagingSpanAttributes(queueURL, messagingOperationSend, messagingOperationSend, nil, nil)...),
	)
	if err != nil {
		return nil, err
	}
	if span != nil {
		defer span.End()
	}

	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(message); err != nil {
		recordSpanError(span, err)
		return nil, err
	}
	body := base64.StdEncoding.EncodeToString(buf.Bytes())
	return c.sendMessageBody(sendCtx, queueURL, body, conf.DelaySeconds, conf.MessageAttributes, span)
}

func (c *Client) sendMessageBody(
	ctx context.Context,
	queueURL QueueURL,
	body string,
	delaySeconds int32,
	messageAttributes map[string]types.MessageAttributeValue,
	span oteltrace.Span,
) (*sqs.SendMessageOutput, error) {
	attributes, err := c.messageAttributes(ctx, messageAttributes)
	if err != nil {
		recordSpanError(span, err)
		return nil, err
	}
	params := &sqs.SendMessageInput{
		MessageBody:       aws.String(body),
		QueueUrl:          queueURL.AWSString(),
		DelaySeconds:      delaySeconds,
		MessageAttributes: attributes,
	}
	sqsRes, err := c.client.SendMessage(ctx, params)
	if err != nil {
		recordSpanError(span, err)
		return nil, err
	}
	return sqsRes, nil
}

// SendMessageBatch sends a batch of already encoded SQS messages. Each entry
// receives an independent trace context when tracing is enabled. The optional
// batch options apply only to this call; a custom operation name retains the
// queue name in the resulting span name. SQS may return per-entry failures with
// a nil Go error, so callers must inspect the returned Failed entries.
func (c *Client) SendMessageBatch(
	ctx context.Context,
	queueURL QueueURL,
	entries []types.SendMessageBatchRequestEntry,
	opts ...sqssend.SendMessageBatchOption,
) (*sqs.SendMessageBatchOutput, error) {
	conf := sqssend.GetBatchConf(opts...)
	prepared, links, err := c.prepareBatchEntries(ctx, queueURL, entries)
	if err != nil {
		return nil, err
	}

	batchMessageCount := len(entries)
	spanOptions := []oteltrace.SpanStartOption{
		oteltrace.WithSpanKind(oteltrace.SpanKindClient),
		oteltrace.WithAttributes(messagingSpanAttributes(
			queueURL,
			messagingOperationSend,
			messagingOperationSend,
			&batchMessageCount,
			nil,
		)...),
	}
	if len(links) > 0 {
		spanOptions = append(spanOptions, oteltrace.WithLinks(links...))
	}
	sendCtx, span, err := c.startSpan(ctx, c.sendSpanName(queueURL, conf.OperationName), spanOptions...)
	if err != nil {
		return nil, err
	}
	if span != nil {
		defer span.End()
	}

	sqsRes, err := c.client.SendMessageBatch(sendCtx, &sqs.SendMessageBatchInput{
		QueueUrl: queueURL.AWSString(),
		Entries:  prepared,
	})
	if err != nil {
		recordSpanError(span, err)
		return nil, err
	}
	if len(sqsRes.Failed) > 0 {
		recordSpanError(span, batchSendError(sqsRes.Failed))
	}
	return sqsRes, nil
}

func (c *Client) prepareBatchEntries(
	ctx context.Context, queueURL QueueURL, entries []types.SendMessageBatchRequestEntry,
) ([]types.SendMessageBatchRequestEntry, []oteltrace.Link, error) {
	prepared := make([]types.SendMessageBatchRequestEntry, len(entries))
	links := make([]oteltrace.Link, 0, len(entries))
	for i := range entries {
		entry, link, err := c.prepareBatchEntry(ctx, queueURL, entries[i])
		if err != nil {
			return nil, nil, err
		}
		prepared[i] = entry
		if link.SpanContext.IsValid() {
			links = append(links, link)
		}
	}
	return prepared, links, nil
}

func (c *Client) prepareBatchEntry(
	ctx context.Context,
	queueURL QueueURL,
	entry types.SendMessageBatchRequestEntry,
) (types.SendMessageBatchRequestEntry, oteltrace.Link, error) {
	prepared := entry
	if !c.config.traceEnabled {
		attributes, err := c.messageAttributes(ctx, entry.MessageAttributes)
		if err != nil {
			return types.SendMessageBatchRequestEntry{}, oteltrace.Link{}, err
		}
		prepared.MessageAttributes = attributes
		return prepared, oteltrace.Link{}, nil
	}

	entryCtx, entrySpan, err := c.startSpan(
		ctx,
		c.createSpanName(queueURL),
		oteltrace.WithSpanKind(oteltrace.SpanKindProducer),
		oteltrace.WithAttributes(messagingSpanAttributes(
			queueURL,
			messagingOperationCreate,
			messagingOperationCreate,
			nil,
			nil,
		)...),
	)
	if err != nil {
		return types.SendMessageBatchRequestEntry{}, oteltrace.Link{}, err
	}
	attributes, err := c.messageAttributes(entryCtx, entry.MessageAttributes)
	if err != nil {
		recordSpanError(entrySpan, err)
		entrySpan.End()
		return types.SendMessageBatchRequestEntry{}, oteltrace.Link{}, err
	}
	link := oteltrace.Link{SpanContext: entrySpan.SpanContext()}
	entrySpan.End()
	prepared.MessageAttributes = attributes
	return prepared, link, nil
}

// ReceiveMessage receives messages from SQS.
// Default MaxNumberOfMessages=1, WaitTimeSeconds=20, VisibilityTimeout=30.
func (c *Client) ReceiveMessage(
	ctx context.Context, queueURL QueueURL, opts ...sqsreceive.ReceiveMessageOption,
) (*sqs.ReceiveMessageOutput, error) {
	conf := sqsreceive.GetConf(opts...)
	params := &sqs.ReceiveMessageInput{
		QueueUrl:              queueURL.AWSString(),
		MaxNumberOfMessages:   conf.MaxNumberOfMessages,
		WaitTimeSeconds:       conf.WaitTimeSeconds,
		VisibilityTimeout:     conf.VisibilityTimeout,
		MessageAttributeNames: []string{"All"},
	}
	sqsRes, err := c.client.ReceiveMessage(ctx, params)
	if err != nil {
		return nil, err
	}
	return sqsRes, nil
}

// ProcessMessage extracts the sender trace context, processes one message, and
// deletes it only when the handler returns nil. The worker context remains the
// process span's parent; the sender context is represented as a span link and
// is passed to the handler as a tracers.TraceInfo value. When the configured
// propagator includes Baggage, the extracted Baggage is added to the handler
// context without changing its active span. An invalid incoming trace context
// is recorded on the process span and does not prevent the handler or
// acknowledgement from running. Optional process options apply only to this
// call, and retain the queue name in a custom span name.
func (c *Client) ProcessMessage(
	ctx context.Context,
	queueURL QueueURL,
	message types.Message,
	handler MessageHandler,
	opts ...sqsprocess.ProcessMessageOption,
) error {
	conf := sqsprocess.GetConf(opts...)
	if handler == nil {
		return ErrNilMessageHandler
	}
	if !c.config.traceEnabled {
		if err := handler(ctx, message, tracers.TraceInfo{}); err != nil {
			return err
		}
		return c.DeleteMessage(ctx, queueURL, message)
	}

	messageContext := c.extractMessageContext(message)
	spanOptions := []oteltrace.SpanStartOption{
		oteltrace.WithSpanKind(oteltrace.SpanKindConsumer),
		oteltrace.WithAttributes(messagingSpanAttributes(
			queueURL,
			messagingOperationProcess,
			messagingOperationProcess,
			nil,
			message.MessageId,
		)...),
	}
	if messageContext.extractionErr == nil && messageContext.hasSourceContext {
		spanOptions = append(spanOptions, oteltrace.WithLinks(oteltrace.Link{
			SpanContext: messageContext.spanContext,
		}))
	}
	processCtx, span, err := c.startSpan(ctx, c.processSpanName(queueURL, conf.OperationName), spanOptions...)
	if err != nil {
		return err
	}
	defer span.End()
	if messageContext.extractionErr != nil {
		recordSpanError(span, messageContext.extractionErr)
	}
	processCtx = withMessageBaggage(processCtx, messageContext.ctx)
	if err := handler(processCtx, message, messageContext.traceInfo); err != nil {
		recordSpanError(span, err)
		return err
	}
	if err := c.DeleteMessage(processCtx, queueURL, message); err != nil {
		recordSpanError(span, err)
		return err
	}
	return nil
}

// DeleteMessage deletes a message from SQS.
func (c *Client) DeleteMessage(ctx context.Context, queueURL QueueURL, message types.Message) error {
	deleteCtx, span, err := c.startSpan(
		ctx,
		c.deleteSpanName(queueURL),
		oteltrace.WithSpanKind(oteltrace.SpanKindClient),
		oteltrace.WithAttributes(messagingSpanAttributes(
			queueURL,
			messagingOperationDelete,
			messagingOperationSettle,
			nil,
			message.MessageId,
		)...),
	)
	if err != nil {
		return err
	}
	if span != nil {
		defer span.End()
	}

	_, err = c.client.DeleteMessage(deleteCtx, &sqs.DeleteMessageInput{
		QueueUrl:      queueURL.AWSString(),
		ReceiptHandle: message.ReceiptHandle,
	})
	if err != nil {
		recordSpanError(span, err)
		return err
	}
	return nil
}

func (c *Client) startSpan(
	ctx context.Context, name string, options ...oteltrace.SpanStartOption,
) (context.Context, oteltrace.Span, error) {
	if !c.config.traceEnabled {
		return ctx, nil, nil
	}

	ctx = awstrace.ContextWithDatadogParent(ctx)
	parentSpanContext := oteltrace.SpanContextFromContext(ctx)
	provider := c.config.traceProvider
	if provider == nil {
		return nil, nil, ErrTraceProviderNotConfigured
	}
	tracer := provider.Tracer(traceInstrumentationName)
	spanCtx, span := tracer.Start(ctx, name, options...)
	createdSpanContext := span.SpanContext()
	if !createdSpanContext.IsValid() {
		err := fmt.Errorf("%w: tracer returned an invalid span context", ErrTraceProviderNotConfigured)
		recordSpanError(span, err)
		span.End()
		return nil, nil, err
	}
	if parentSpanContext.IsValid() && createdSpanContext.Equal(parentSpanContext) {
		err := fmt.Errorf("%w: tracer returned the parent span context", ErrTraceProviderNotConfigured)
		recordSpanError(span, err)
		span.End()
		return nil, nil, err
	}
	return spanCtx, span, nil
}

func (c *Client) sendSpanName(queueURL QueueURL, operationName string) string {
	if operationName == "" {
		operationName = defaultSendSpanPrefix
	}
	return operationName + " " + queueName(queueURL)
}

func (c *Client) createSpanName(queueURL QueueURL) string {
	return defaultCreateSpanPrefix + " " + queueName(queueURL)
}

func (c *Client) processSpanName(queueURL QueueURL, operationName string) string {
	if operationName == "" {
		operationName = defaultProcessSpanPrefix
	}
	return operationName + " " + queueName(queueURL)
}

func (c *Client) deleteSpanName(queueURL QueueURL) string {
	return defaultDeleteSpanPrefix + " " + queueName(queueURL)
}

func (c *Client) messageAttributes(
	ctx context.Context, input map[string]types.MessageAttributeValue,
) (map[string]types.MessageAttributeValue, error) {
	if !c.config.traceEnabled {
		if len(input) > maxMessageAttributes {
			return nil, fmt.Errorf("%w: got %d attributes, maximum %d", ErrTooManyMessageAttributes,
				len(input), maxMessageAttributes)
		}
		return cloneMessageAttributes(input), nil
	}
	propagator := c.config.tracePropagator
	if propagator == nil {
		return nil, ErrTracePropagatorNotConfigured
	}
	reservedAttributes := reservedMessageAttributes(propagator)
	reservedAttributeCount := len(reservedAttributes)
	if len(input)+reservedAttributeCount > maxMessageAttributes {
		return nil, fmt.Errorf("%w: got %d attributes plus %d reserved trace attributes, maximum %d",
			ErrTooManyMessageAttributes, len(input), reservedAttributeCount, maxMessageAttributes)
	}
	for key := range reservedAttributes {
		if _, ok := input[key]; ok {
			return nil, fmt.Errorf("%w: %s", ErrReservedMessageAttribute, key)
		}
	}

	carrier := propagation.MapCarrier{}
	propagator.Inject(ctx, carrier)
	if err := validateInjectedTraceContext(ctx, carrier); err != nil {
		return nil, err
	}
	if len(input)+len(carrier) > maxMessageAttributes {
		return nil, fmt.Errorf("%w: got %d attributes plus %d injected trace attributes, maximum %d",
			ErrTooManyMessageAttributes, len(input), len(carrier), maxMessageAttributes)
	}

	attributes := cloneMessageAttributes(input)
	if attributes == nil {
		attributes = make(map[string]types.MessageAttributeValue, len(carrier))
	}
	for key, value := range carrier {
		attributes[key] = stringMessageAttribute(value)
	}
	return attributes, nil
}

func messagingSpanAttributes(
	queueURL QueueURL,
	operationName, operationType string,
	batchMessageCount *int,
	messageID *string,
) []attribute.KeyValue {
	attributes := []attribute.KeyValue{
		attribute.String("messaging.system", messagingSystem),
		attribute.String("messaging.operation.name", operationName),
		attribute.String("messaging.operation.type", operationType),
		attribute.String("messaging.destination.name", queueName(queueURL)),
		attribute.String("aws.sqs.queue.url", queueURL.String()),
	}
	if batchMessageCount != nil {
		attributes = append(attributes, attribute.Int("messaging.batch.message_count", *batchMessageCount))
	}
	if messageID != nil && *messageID != "" {
		attributes = append(attributes, attribute.String("messaging.message.id", *messageID))
	}
	return attributes
}

func validateInjectedTraceContext(ctx context.Context, carrier propagation.MapCarrier) error {
	traceParent := carrier.Get(traceParentMessageAttribute)
	traceState := carrier.Get(traceStateMessageAttribute)
	if traceParent == "" {
		return fmt.Errorf("%w: propagator did not inject traceparent", ErrTracePropagatorNotConfigured)
	}
	traceCarrier := propagation.MapCarrier{traceParentMessageAttribute: traceParent}
	if traceState != "" {
		traceCarrier[traceStateMessageAttribute] = traceState
	}
	extracted := propagation.TraceContext{}.Extract(context.Background(), traceCarrier)
	extractedSpanContext := oteltrace.SpanContextFromContext(extracted)
	currentSpanContext := oteltrace.SpanContextFromContext(ctx)
	if !extractedSpanContext.IsValid() || !currentSpanContext.IsValid() {
		return fmt.Errorf("%w: propagator injected an invalid W3C context", ErrInvalidTraceContext)
	}
	if extractedSpanContext.TraceID() != currentSpanContext.TraceID() ||
		extractedSpanContext.SpanID() != currentSpanContext.SpanID() ||
		extractedSpanContext.TraceFlags() != currentSpanContext.TraceFlags() {
		return fmt.Errorf("%w: propagator context does not match the send span", ErrInvalidTraceContext)
	}
	if traceState != "" && extractedSpanContext.TraceState().String() != traceState {
		return fmt.Errorf("%w: propagator injected invalid tracestate", ErrInvalidTraceContext)
	}
	return nil
}

func (c *Client) extractMessageContext(message types.Message) extractedMessageContext {
	messageContext := extractedMessageContext{ctx: context.Background()}
	propagator := c.config.tracePropagator
	if propagator == nil {
		messageContext.extractionErr = ErrTracePropagatorNotConfigured
		return messageContext
	}

	traceParentAttribute, hasTraceParent := message.MessageAttributes[traceParentMessageAttribute]
	traceStateAttribute, hasTraceState := message.MessageAttributes[traceStateMessageAttribute]
	var traceParent, traceState string
	var traceErr error
	if hasTraceParent {
		if !isStringMessageAttribute(traceParentAttribute) {
			traceErr = fmt.Errorf("%w: invalid %s message attribute", ErrInvalidTraceContext, traceParentMessageAttribute)
		} else {
			traceParent = *traceParentAttribute.StringValue
		}
	} else if hasTraceState {
		traceErr = fmt.Errorf("%w: invalid %s message attribute", ErrInvalidTraceContext, traceParentMessageAttribute)
	}
	if traceErr == nil && hasTraceState {
		if !isStringMessageAttribute(traceStateAttribute) {
			traceErr = fmt.Errorf("%w: invalid %s message attribute", ErrInvalidTraceContext, traceStateMessageAttribute)
		} else {
			traceState = *traceStateAttribute.StringValue
			if traceState != "" {
				if _, err := oteltrace.ParseTraceState(traceState); err != nil {
					traceErr = fmt.Errorf("%w: invalid message tracestate", ErrInvalidTraceContext)
				}
			}
		}
	}

	carrier := propagation.MapCarrier{}
	for key, attribute := range message.MessageAttributes {
		if !isStringMessageAttribute(attribute) {
			continue
		}
		carrier[key] = *attribute.StringValue
	}
	messageContext.ctx = propagator.Extract(context.Background(), carrier)
	if !hasTraceParent && !hasTraceState {
		return messageContext
	}
	if traceErr != nil {
		messageContext.extractionErr = traceErr
		return messageContext
	}

	if messageContext.ctx == nil {
		messageContext.extractionErr = fmt.Errorf("%w: propagator returned a nil context", ErrInvalidTraceContext)
		return messageContext
	}
	extractedSpanContext := oteltrace.SpanContextFromContext(messageContext.ctx)
	if !extractedSpanContext.IsValid() {
		messageContext.extractionErr = fmt.Errorf("%w: propagator could not extract message context", ErrInvalidTraceContext)
		return messageContext
	}
	traceInfo := tracers.NewTraceInfoFromTraceParent(traceParent, traceState)
	if !traceInfo.IsValid() {
		messageContext.extractionErr = fmt.Errorf("%w: could not create message trace info", ErrInvalidTraceContext)
		return messageContext
	}
	messageContext.spanContext = extractedSpanContext
	messageContext.traceInfo = traceInfo
	messageContext.hasSourceContext = true
	return messageContext
}

func withMessageBaggage(processCtx, messageCtx context.Context) context.Context {
	if messageCtx == nil {
		return processCtx
	}
	messageBaggage := baggage.FromContext(messageCtx)
	if messageBaggage.Len() == 0 {
		return processCtx
	}
	return baggage.ContextWithBaggage(processCtx, messageBaggage)
}

func isStringMessageAttribute(attribute types.MessageAttributeValue) bool {
	return attribute.DataType != nil && *attribute.DataType == "String" && attribute.StringValue != nil
}

func stringMessageAttribute(value string) types.MessageAttributeValue {
	return types.MessageAttributeValue{
		DataType:    aws.String("String"),
		StringValue: aws.String(value),
	}
}

func cloneMessageAttributes(input map[string]types.MessageAttributeValue) map[string]types.MessageAttributeValue {
	if input == nil {
		return nil
	}
	output := make(map[string]types.MessageAttributeValue, len(input))
	for key, value := range input {
		output[key] = value
	}
	return output
}

func reservedMessageAttributes(propagator propagation.TextMapPropagator) map[string]struct{} {
	reserved := make(map[string]struct{})
	for _, field := range propagator.Fields() {
		reserved[field] = struct{}{}
	}
	return reserved
}

func recordSpanError(span oteltrace.Span, err error) {
	if span == nil || err == nil {
		return
	}
	span.RecordError(err)
	span.SetStatus(codes.Error, err.Error())
}

func batchSendError(failed []types.BatchResultErrorEntry) error {
	details := make([]string, 0, len(failed))
	for _, entry := range failed {
		details = append(details, fmt.Sprintf(
			"id=%s code=%s message=%s",
			stringValue(entry.Id),
			stringValue(entry.Code),
			stringValue(entry.Message),
		))
	}
	return fmt.Errorf("awssqs: %d batch message(s) failed: %s", len(failed), strings.Join(details, "; "))
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func queueName(queueURL QueueURL) string {
	raw := strings.TrimSpace(queueURL.String())
	if raw == "" {
		return "unknown"
	}
	if parsed, err := url.Parse(raw); err == nil {
		if name := path.Base(strings.Trim(parsed.Path, "/")); name != "." && name != "/" && name != "" {
			if decoded, err := url.PathUnescape(name); err == nil && decoded != "" {
				return decoded
			}
			return name
		}
	}
	name := path.Base(strings.Trim(raw, "/"))
	if name == "." || name == "/" || name == "" {
		return "unknown"
	}
	return name
}
