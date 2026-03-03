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

	"github.com/88labs/go-utils/aws/awssqs/options/sqsreceive"
	"github.com/88labs/go-utils/aws/awssqs/options/sqssend"
)

// SendMessage converts a message to JSON and sends it to SQS.
// Default DelaySeconds=0.
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
	sqsRes, err := c.client.SendMessage(ctx, params)
	if err != nil {
		return nil, err
	}
	return sqsRes, nil
}

// SendMessageGob converts a message to gob encoding and sends it to SQS.
// Default DelaySeconds=0.
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
	sqsRes, err := c.client.SendMessage(ctx, params)
	if err != nil {
		return nil, err
	}
	return sqsRes, nil
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

// DeleteMessage deletes a message from SQS.
func (c *Client) DeleteMessage(ctx context.Context, queueURL QueueURL, message types.Message) error {
	params := &sqs.DeleteMessageInput{
		QueueUrl:      queueURL.AWSString(),
		ReceiptHandle: message.ReceiptHandle,
	}
	if _, err := c.client.DeleteMessage(ctx, params); err != nil {
		return err
	}
	return nil
}
