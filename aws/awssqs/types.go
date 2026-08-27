package awssqs

import (
	"context"

	"github.com/88labs/go-utils/tracers"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"
)

type QueueURL string

func (q QueueURL) String() string {
	return string(q)
}

func (q QueueURL) AWSString() *string {
	return aws.String(string(q))
}

// MessageHandler processes one received SQS message. The context contains the
// worker process span, and messageTraceInfo contains the sender's W3C trace
// context when it is valid. A zero TraceInfo indicates that the message trace
// context is unavailable. A nil error acknowledges the message after the
// handler returns.
type MessageHandler func(context.Context, types.Message, tracers.TraceInfo) error
