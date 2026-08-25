package awssqs

import (
	"context"

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

// MessageHandler processes one received SQS message. A nil error acknowledges
// the message after the handler returns.
type MessageHandler func(context.Context, types.Message) error
