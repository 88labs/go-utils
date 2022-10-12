package sqssend

import (
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"
)

type SendMessageOption interface {
	Apply(*confSendMessage)
}

type confSendMessage struct {
	// SendMessage
	DelaySeconds      int32
	MessageAttributes map[string]types.MessageAttributeValue
}

type OptionDelaySeconds int32

func (o OptionDelaySeconds) Apply(c *confSendMessage) {
	c.DelaySeconds = int32(o)
}

func WithDelaySeconds(delaySeconds int32) OptionDelaySeconds {
	return OptionDelaySeconds(delaySeconds)
}

type OptionMessageAttributes map[string]types.MessageAttributeValue

func (o OptionMessageAttributes) Apply(c *confSendMessage) {
	c.MessageAttributes = o
}

func WithMessageAttributes(attributes map[string]types.MessageAttributeValue) OptionMessageAttributes {
	return OptionMessageAttributes(attributes)
}

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
