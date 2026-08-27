package sqssend

import "github.com/aws/aws-sdk-go-v2/service/sqs/types"

// SendMessageOption configures one SendMessage or SendMessageGob call.
type SendMessageOption interface {
	Apply(*confSendMessage)
}

// SendMessageBatchOption configures one SendMessageBatch call.
type SendMessageBatchOption interface {
	applyBatch(*confSendMessageBatch)
}

type confSendMessage struct {
	// SendMessage
	DelaySeconds      int32
	MessageAttributes map[string]types.MessageAttributeValue
	OperationName     string
}

type confSendMessageBatch struct {
	OperationName string
}

type OptionDelaySeconds int32

func (o OptionDelaySeconds) Apply(c *confSendMessage) {
	c.DelaySeconds = int32(o)
}

func WithDelaySeconds(delaySeconds int32) OptionDelaySeconds {
	return OptionDelaySeconds(delaySeconds)
}

// OptionOperationName changes the operation portion of the send span name.
// The queue name is always appended by the SQS client.
type OptionOperationName string

// Apply configures the operation portion of a send span name.
func (o OptionOperationName) Apply(c *confSendMessage) {
	c.OperationName = string(o)
}

// applyBatch configures the operation portion of a batch send span name.
func (o OptionOperationName) applyBatch(c *confSendMessageBatch) {
	c.OperationName = string(o)
}

// WithOperationName changes the operation portion of the send span name for
// this API call. The default is "send" and the final name includes the queue,
// for example "publish orders".
func WithOperationName(operationName string) OptionOperationName {
	return OptionOperationName(operationName)
}

// OptionMessageAttributes supplies application-defined SQS message
// attributes. With the standard tracing configuration, traceparent and
// tracestate are reserved, leaving at most eight application-defined
// attributes. Baggage is reserved only when a custom propagator includes it;
// propagation.Baggage{} reserves one additional attribute and leaves at most
// seven application-defined attributes. Trace attributes use the SQS String
// data type, and invalid values or a final total above ten attributes are
// rejected before the request is sent.
type OptionMessageAttributes map[string]types.MessageAttributeValue

func (o OptionMessageAttributes) Apply(c *confSendMessage) {
	c.MessageAttributes = o
}

func WithMessageAttributes(attributes map[string]types.MessageAttributeValue) OptionMessageAttributes {
	return OptionMessageAttributes(attributes)
}

// GetConf resolves options for one SendMessage or SendMessageGob call.
func GetConf(opts ...SendMessageOption) confSendMessage {
	// default options
	c := confSendMessage{
		DelaySeconds: 0,
	}
	for _, opt := range opts {
		opt.Apply(&c)
	}
	return c
}

// GetBatchConf resolves options for one SendMessageBatch call.
func GetBatchConf(opts ...SendMessageBatchOption) confSendMessageBatch {
	var c confSendMessageBatch
	for _, opt := range opts {
		if opt != nil {
			opt.applyBatch(&c)
		}
	}
	return c
}
