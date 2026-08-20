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
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	oteltrace "go.opentelemetry.io/otel/trace"

	"github.com/88labs/go-utils/aws/awssqs/options/sqsreceive"
	"github.com/88labs/go-utils/aws/awssqs/options/sqssend"
	"github.com/88labs/go-utils/aws/internal/awstrace"
)

const (
	traceInstrumentationName    = "github.com/88labs/go-utils/aws/awssqs"
	traceParentMessageAttribute = "traceparent"
	traceStateMessageAttribute  = "tracestate"
	maxMessageAttributes        = 10
	traceMessageAttributeCount  = 2
	defaultSendSpanPrefix       = "send"
	defaultProcessSpanPrefix    = "process"
	defaultDeleteSpanPrefix     = "delete"
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

// SendMessage converts a message to JSON and sends it to SQS.
// Default DelaySeconds=0.
func (c *Client) SendMessage(
	ctx context.Context, queueURL QueueURL, message any, opts ...sqssend.SendMessageOption,
) (*sqs.SendMessageOutput, error) {
	conf := sqssend.GetConf(opts...)
	sendCtx, span, err := c.startSpan(ctx, c.sendSpanName(queueURL), oteltrace.WithSpanKind(oteltrace.SpanKindProducer))
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
	sendCtx, span, err := c.startSpan(ctx, c.sendSpanName(queueURL), oteltrace.WithSpanKind(oteltrace.SpanKindProducer))
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
// receives an independent trace context when tracing is enabled.
func (c *Client) SendMessageBatch(
	ctx context.Context, queueURL QueueURL, entries []types.SendMessageBatchRequestEntry,
) (*sqs.SendMessageBatchOutput, error) {
	sendCtx, span, err := c.startSpan(ctx, c.sendSpanName(queueURL), oteltrace.WithSpanKind(oteltrace.SpanKindProducer))
	if err != nil {
		return nil, err
	}
	if span != nil {
		defer span.End()
	}

	prepared := make([]types.SendMessageBatchRequestEntry, len(entries))
	for i := range entries {
		prepared[i], err = c.prepareBatchEntry(sendCtx, queueURL, entries[i], span)
		if err != nil {
			return nil, err
		}
	}

	sqsRes, err := c.client.SendMessageBatch(sendCtx, &sqs.SendMessageBatchInput{
		QueueUrl: queueURL.AWSString(),
		Entries:  prepared,
	})
	if err != nil {
		recordSpanError(span, err)
		return nil, err
	}
	return sqsRes, nil
}

func (c *Client) prepareBatchEntry(
	ctx context.Context,
	queueURL QueueURL,
	entry types.SendMessageBatchRequestEntry,
	batchSpan oteltrace.Span,
) (types.SendMessageBatchRequestEntry, error) {
	prepared := entry
	if !c.config.traceEnabled {
		attributes, err := c.messageAttributes(ctx, entry.MessageAttributes)
		if err != nil {
			recordSpanError(batchSpan, err)
			return types.SendMessageBatchRequestEntry{}, err
		}
		prepared.MessageAttributes = attributes
		return prepared, nil
	}

	entryCtx, entrySpan, err := c.startSpan(
		ctx,
		c.sendSpanName(queueURL),
		oteltrace.WithSpanKind(oteltrace.SpanKindProducer),
	)
	if err != nil {
		recordSpanError(batchSpan, err)
		return types.SendMessageBatchRequestEntry{}, err
	}
	attributes, err := c.messageAttributes(entryCtx, entry.MessageAttributes)
	if err != nil {
		recordSpanError(entrySpan, err)
		entrySpan.End()
		recordSpanError(batchSpan, err)
		return types.SendMessageBatchRequestEntry{}, err
	}
	entrySpan.End()
	prepared.MessageAttributes = attributes
	return prepared, nil
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
// deletes it only when handler returns nil. The worker context remains the
// process span's parent; the sender context is represented as a span link.
func (c *Client) ProcessMessage(
	ctx context.Context, queueURL QueueURL, message types.Message, handler MessageHandler,
) error {
	if handler == nil {
		return ErrNilMessageHandler
	}
	if !c.config.traceEnabled {
		if err := handler(ctx, message); err != nil {
			return err
		}
		return c.DeleteMessage(ctx, queueURL, message)
	}

	sourceSpanContext, hasSourceContext, err := extractMessageSpanContext(message)
	if err != nil {
		return err
	}
	spanOptions := []oteltrace.SpanStartOption{
		oteltrace.WithSpanKind(oteltrace.SpanKindConsumer),
	}
	if hasSourceContext {
		spanOptions = append(spanOptions, oteltrace.WithLinks(oteltrace.Link{
			SpanContext: sourceSpanContext,
		}))
	}
	processCtx, span, err := c.startSpan(ctx, c.processSpanName(queueURL), spanOptions...)
	if err != nil {
		return err
	}
	defer span.End()

	if err := handler(processCtx, message); err != nil {
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
	provider := c.config.traceProvider
	if provider == nil {
		provider = otel.GetTracerProvider()
	}
	if provider == nil {
		return nil, nil, ErrTraceProviderNotConfigured
	}
	tracer := provider.Tracer(traceInstrumentationName)
	if tracer == nil {
		return nil, nil, ErrTraceProviderNotConfigured
	}
	spanCtx, span := tracer.Start(ctx, name, options...)
	if span == nil {
		return nil, nil, ErrTraceProviderNotConfigured
	}
	if !oteltrace.SpanContextFromContext(spanCtx).IsValid() && span.SpanContext().IsValid() {
		spanCtx = oteltrace.ContextWithSpan(spanCtx, span)
	}
	if !oteltrace.SpanContextFromContext(spanCtx).IsValid() {
		err := fmt.Errorf("%w: tracer returned an invalid span context", ErrTraceProviderNotConfigured)
		recordSpanError(span, err)
		span.End()
		return nil, nil, err
	}
	return spanCtx, span, nil
}

func (c *Client) sendSpanName(queueURL QueueURL) string {
	if c.config.sendSpanName != "" {
		return c.config.sendSpanName
	}
	return defaultSendSpanPrefix + " " + queueName(queueURL)
}

func (c *Client) processSpanName(queueURL QueueURL) string {
	if c.config.processSpanName != "" {
		return c.config.processSpanName
	}
	return defaultProcessSpanPrefix + " " + queueName(queueURL)
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
	if _, ok := input[traceParentMessageAttribute]; ok {
		return nil, fmt.Errorf("%w: %s", ErrReservedMessageAttribute, traceParentMessageAttribute)
	}
	if _, ok := input[traceStateMessageAttribute]; ok {
		return nil, fmt.Errorf("%w: %s", ErrReservedMessageAttribute, traceStateMessageAttribute)
	}
	if len(input)+traceMessageAttributeCount > maxMessageAttributes {
		return nil, fmt.Errorf("%w: got %d attributes plus %d trace attributes, maximum %d",
			ErrTooManyMessageAttributes, len(input), traceMessageAttributeCount, maxMessageAttributes)
	}

	propagator := otel.GetTextMapPropagator()
	if propagator == nil {
		return nil, ErrTracePropagatorNotConfigured
	}
	carrier := propagation.MapCarrier{}
	propagator.Inject(ctx, carrier)
	traceParent := carrier.Get(traceParentMessageAttribute)
	traceState := carrier.Get(traceStateMessageAttribute)
	if err := validateInjectedTraceContext(ctx, traceParent, traceState); err != nil {
		return nil, err
	}

	attributes := cloneMessageAttributes(input)
	if attributes == nil {
		attributes = make(map[string]types.MessageAttributeValue, traceMessageAttributeCount)
	}
	attributes[traceParentMessageAttribute] = stringMessageAttribute(traceParent)
	attributes[traceStateMessageAttribute] = stringMessageAttribute(traceState)
	return attributes, nil
}

func validateInjectedTraceContext(ctx context.Context, traceParent, traceState string) error {
	if traceParent == "" {
		return fmt.Errorf("%w: propagator did not inject traceparent", ErrTracePropagatorNotConfigured)
	}
	carrier := propagation.MapCarrier{traceParentMessageAttribute: traceParent}
	if traceState != "" {
		carrier[traceStateMessageAttribute] = traceState
	}
	extracted := propagation.TraceContext{}.Extract(context.Background(), carrier)
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

func extractMessageSpanContext(message types.Message) (oteltrace.SpanContext, bool, error) {
	traceParentAttribute, hasTraceParent := message.MessageAttributes[traceParentMessageAttribute]
	traceStateAttribute, hasTraceState := message.MessageAttributes[traceStateMessageAttribute]
	if !hasTraceParent && !hasTraceState {
		return oteltrace.SpanContext{}, false, nil
	}
	if !hasTraceParent || !isStringMessageAttribute(traceParentAttribute) {
		return oteltrace.SpanContext{}, false,
			fmt.Errorf("%w: invalid %s message attribute", ErrInvalidTraceContext, traceParentMessageAttribute)
	}
	if hasTraceState && !isStringMessageAttribute(traceStateAttribute) {
		return oteltrace.SpanContext{}, false,
			fmt.Errorf("%w: invalid %s message attribute", ErrInvalidTraceContext, traceStateMessageAttribute)
	}

	carrier := propagation.MapCarrier{
		traceParentMessageAttribute: *traceParentAttribute.StringValue,
	}
	if hasTraceState && *traceStateAttribute.StringValue != "" {
		carrier[traceStateMessageAttribute] = *traceStateAttribute.StringValue
	}
	propagator := otel.GetTextMapPropagator()
	if propagator == nil {
		return oteltrace.SpanContext{}, false, ErrTracePropagatorNotConfigured
	}
	extracted := propagator.Extract(context.Background(), carrier)
	spanContext := oteltrace.SpanContextFromContext(extracted)
	if !spanContext.IsValid() {
		return oteltrace.SpanContext{}, false,
			fmt.Errorf("%w: propagator could not extract message context", ErrInvalidTraceContext)
	}
	return spanContext, true, nil
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

func recordSpanError(span oteltrace.Span, err error) {
	if span == nil || err == nil {
		return
	}
	span.RecordError(err)
	span.SetStatus(codes.Error, err.Error())
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
