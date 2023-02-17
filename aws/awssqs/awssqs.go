package awssqs

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/gob"
	"encoding/json"

	"github.com/88labs/go-utils/aws/awsconfig"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"

	"github.com/88labs/go-utils/aws/awssqs/options/sqsreceive"
	"github.com/88labs/go-utils/aws/awssqs/options/sqssend"
)

type QueueURL string

func (q QueueURL) String() string {
	return string(q)
}

func (q QueueURL) AWSString() *string {
	return aws.String(string(q))
}

// SendMessage
// aws-sdk-go v2 sqs SentMessage
// convert message to json and send to sqs.
// default DelaySeconds=0
//
// Mocks: Using ctxawslocal.WithContext, you can make requests for local mocks.
func SendMessage(ctx context.Context, region awsconfig.Region, queueURL QueueURL, message any, opts ...sqssend.SendMessageOption) (*sqs.SendMessageOutput, error) {
	c := sqssend.GetConf(opts...)
	client, err := GetClient(ctx, region)
	if err != nil {
		return nil, err
	}
	jsonb, err := json.Marshal(message)
	if err != nil {
		return nil, err
	}
	params := &sqs.SendMessageInput{
		MessageBody:       aws.String(string(jsonb)),
		QueueUrl:          queueURL.AWSString(),
		DelaySeconds:      c.DelaySeconds,
		MessageAttributes: c.MessageAttributes,
	}
	sqsRes, err := client.SendMessage(ctx, params)
	if err != nil {
		return nil, err
	}
	return sqsRes, nil
}

// SendMessageGob
// aws-sdk-go v2 sqs SentMessage
// convert message to gob and send to sqs.
// default DelaySeconds=0
//
// Mocks: Using ctxawslocal.WithContext, you can make requests for local mocks.
func SendMessageGob(ctx context.Context, region awsconfig.Region, queueURL QueueURL, message any, opts ...sqssend.SendMessageOption) (*sqs.SendMessageOutput, error) {
	c := sqssend.GetConf(opts...)
	client, err := GetClient(ctx, region)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	if err := enc.Encode(message); err != nil {
		return nil, err
	}
	b64 := base64.StdEncoding.EncodeToString(buf.Bytes())
	params := &sqs.SendMessageInput{
		MessageBody:       aws.String(b64),
		QueueUrl:          queueURL.AWSString(),
		DelaySeconds:      c.DelaySeconds,
		MessageAttributes: c.MessageAttributes,
	}
	sqsRes, err := client.SendMessage(ctx, params)
	if err != nil {
		return nil, err
	}
	return sqsRes, nil
}

// ReceiveMessage
// aws-sdk-go v2 sqs ReceiveMessage
// default MaxNumberOfMessages=1, WaitTimeSeconds=20, VisibilityTimeout=30
//
// Mocks: Using ctxawslocal.WithContext, you can make requests for local mocks.
func ReceiveMessage(ctx context.Context, region awsconfig.Region, queueURL QueueURL, opts ...sqsreceive.ReceiveMessageOption) (*sqs.ReceiveMessageOutput, error) {
	c := sqsreceive.GetConf(opts...)
	client, err := GetClient(ctx, region)
	if err != nil {
		return nil, err
	}

	params := &sqs.ReceiveMessageInput{
		QueueUrl:              queueURL.AWSString(),
		MaxNumberOfMessages:   c.MaxNumberOfMessages,
		WaitTimeSeconds:       c.WaitTimeSeconds,
		VisibilityTimeout:     c.VisibilityTimeout,
		MessageAttributeNames: []string{"All"},
	}
	sqsRes, err := client.ReceiveMessage(ctx, params)
	if err != nil {
		return nil, err
	}
	return sqsRes, nil
}

// ReceiveMessageGob
// aws-sdk-go v2 sqs ReceiveMessage
// Message received from sqs with gob.
// default MaxNumberOfMessages=1, WaitTimeSeconds=20, VisibilityTimeout=30
//
// Mocks: Using ctxawslocal.WithContext, you can make requests for local mocks.
func ReceiveMessageGob[T any](ctx context.Context, region awsconfig.Region, queueURL QueueURL, _ T, opts ...sqsreceive.ReceiveMessageOption) ([]*T, *sqs.ReceiveMessageOutput, error) {
	c := sqsreceive.GetConf(opts...)
	client, err := GetClient(ctx, region)
	if err != nil {
		return nil, nil, err
	}

	params := &sqs.ReceiveMessageInput{
		QueueUrl:              queueURL.AWSString(),
		MaxNumberOfMessages:   c.MaxNumberOfMessages,
		WaitTimeSeconds:       c.WaitTimeSeconds,
		VisibilityTimeout:     c.VisibilityTimeout,
		MessageAttributeNames: []string{"All"},
	}
	sqsRes, err := client.ReceiveMessage(ctx, params)
	if err != nil {
		return nil, nil, err
	}

	items := make([]*T, 0, len(sqsRes.Messages))
	for i := range sqsRes.Messages {
		msg := &sqsRes.Messages[i]
		if msg.Body == nil {
			continue
		}
		b, err := base64.StdEncoding.DecodeString(*msg.Body)
		if err != nil {
			return nil, nil, err
		}
		buf := bytes.NewBuffer(b)
		var item T
		if err := gob.NewDecoder(buf).Decode(&item); err != nil {
			return nil, nil, err
		}
		items = append(items, &item)
	}

	return items, sqsRes, nil
}

// DeleteMessage
// aws-sdk-go v2 sqs DeleteMessage
//
// Mocks: Using ctxawslocal.WithContext, you can make requests for local mocks.
func DeleteMessage(ctx context.Context, region awsconfig.Region, queueURL QueueURL, message types.Message) error {
	client, err := GetClient(ctx, region)
	if err != nil {
		return err
	}
	params := &sqs.DeleteMessageInput{
		QueueUrl:      queueURL.AWSString(),
		ReceiptHandle: message.ReceiptHandle,
	}
	if _, err := client.DeleteMessage(ctx, params); err != nil {
		return err
	}
	return nil
}
