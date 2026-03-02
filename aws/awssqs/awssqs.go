package awssqs

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/gob"
	"encoding/json"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"

	"github.com/88labs/go-utils/aws/awsconfig"
	"github.com/88labs/go-utils/aws/awssqs/options/sqsreceive"
	"github.com/88labs/go-utils/aws/awssqs/options/sqssend"
)

// SendMessage sends a JSON-encoded message to SQS.
// default DelaySeconds=0
func (c *Client) SendMessage(
	ctx context.Context, queueURL QueueURL, message any, opts ...sqssend.SendMessageOption,
) (*sqs.SendMessageOutput, error) {
	conf := sqssend.GetConf(opts...)
	jsonb, err := json.Marshal(message)
	if err != nil {
		return nil, err
	}
	params := &sqs.SendMessageInput{
		MessageBody:       aws.String(string(jsonb)),
		QueueUrl:          queueURL.AWSString(),
		DelaySeconds:      conf.DelaySeconds,
		MessageAttributes: conf.MessageAttributes,
	}
	sqsRes, err := c.raw.SendMessage(ctx, params)
	if err != nil {
		return nil, err
	}
	return sqsRes, nil
}

// SendMessage
// aws-sdk-go v2 sqs SentMessage
// convert message to json and send to sqs.
// default DelaySeconds=0
//
// Mocks: Using ctxawslocal.WithContext, you can make requests for local mocks.
func SendMessage(
	ctx context.Context, region awsconfig.Region, queueURL QueueURL, message any, opts ...sqssend.SendMessageOption,
) (*sqs.SendMessageOutput, error) {
	sqsClient, err := GetClient(ctx, region)
	if err != nil {
		return nil, err
	}
	return (&Client{raw: sqsClient}).SendMessage(ctx, queueURL, message, opts...)
}

// SendMessageGob sends a gob-encoded message to SQS.
// default DelaySeconds=0
func (c *Client) SendMessageGob(
	ctx context.Context, queueURL QueueURL, message any, opts ...sqssend.SendMessageOption,
) (*sqs.SendMessageOutput, error) {
	conf := sqssend.GetConf(opts...)
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	if err := enc.Encode(message); err != nil {
		return nil, err
	}
	b64 := base64.StdEncoding.EncodeToString(buf.Bytes())
	params := &sqs.SendMessageInput{
		MessageBody:       aws.String(b64),
		QueueUrl:          queueURL.AWSString(),
		DelaySeconds:      conf.DelaySeconds,
		MessageAttributes: conf.MessageAttributes,
	}
	sqsRes, err := c.raw.SendMessage(ctx, params)
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
func SendMessageGob(
	ctx context.Context, region awsconfig.Region, queueURL QueueURL, message any, opts ...sqssend.SendMessageOption,
) (*sqs.SendMessageOutput, error) {
	sqsClient, err := GetClient(ctx, region)
	if err != nil {
		return nil, err
	}
	return (&Client{raw: sqsClient}).SendMessageGob(ctx, queueURL, message, opts...)
}

// ReceiveMessage receives messages from SQS.
// default MaxNumberOfMessages=1, WaitTimeSeconds=20, VisibilityTimeout=30
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
	sqsRes, err := c.raw.ReceiveMessage(ctx, params)
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
func ReceiveMessage(
	ctx context.Context, region awsconfig.Region, queueURL QueueURL, opts ...sqsreceive.ReceiveMessageOption,
) (*sqs.ReceiveMessageOutput, error) {
	sqsClient, err := GetClient(ctx, region)
	if err != nil {
		return nil, err
	}
	return (&Client{raw: sqsClient}).ReceiveMessage(ctx, queueURL, opts...)
}

// ReceiveMessageGob on Client receives messages and decodes gob payloads.
// Returns raw SQS output. Callers can decode gob messages themselves.
// default MaxNumberOfMessages=1, WaitTimeSeconds=20, VisibilityTimeout=30
func (c *Client) ReceiveMessageGob(
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
	return c.raw.ReceiveMessage(ctx, params)
}

// ReceiveMessageGob
// aws-sdk-go v2 sqs ReceiveMessage
// Message received from sqs with gob.
// default MaxNumberOfMessages=1, WaitTimeSeconds=20, VisibilityTimeout=30
//
// Mocks: Using ctxawslocal.WithContext, you can make requests for local mocks.
func ReceiveMessageGob[T any](
	ctx context.Context, region awsconfig.Region, queueURL QueueURL, _ T, opts ...sqsreceive.ReceiveMessageOption,
) ([]*T, *sqs.ReceiveMessageOutput, error) {
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

// DeleteMessage deletes a message from SQS.
func (c *Client) DeleteMessage(ctx context.Context, queueURL QueueURL, message types.Message) error {
	params := &sqs.DeleteMessageInput{
		QueueUrl:      queueURL.AWSString(),
		ReceiptHandle: message.ReceiptHandle,
	}
	if _, err := c.raw.DeleteMessage(ctx, params); err != nil {
		return err
	}
	return nil
}

// DeleteMessage
// aws-sdk-go v2 sqs DeleteMessage
//
// Mocks: Using ctxawslocal.WithContext, you can make requests for local mocks.
func DeleteMessage(ctx context.Context, region awsconfig.Region, queueURL QueueURL, message types.Message) error {
	sqsClient, err := GetClient(ctx, region)
	if err != nil {
		return err
	}
	return (&Client{raw: sqsClient}).DeleteMessage(ctx, queueURL, message)
}
