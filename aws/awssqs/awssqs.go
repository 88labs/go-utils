package awssqs

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/gob"

	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"

	"github.com/88labs/go-utils/aws/awsconfig"
	"github.com/88labs/go-utils/aws/awssqs/options/sqsreceive"
	"github.com/88labs/go-utils/aws/awssqs/options/sqssend"
)

// SendMessage
// aws-sdk-go v2 sqs SentMessage
// convert message to json and send to sqs.
// default DelaySeconds=0
//
// Mocks: Using ctxawslocal.WithContext, you can make requests for local mocks.
func SendMessage(
	ctx context.Context, region awsconfig.Region, queueURL QueueURL, message any, opts ...sqssend.SendMessageOption,
) (*sqs.SendMessageOutput, error) {
	sdkClient, err := GetClient(ctx, region)
	if err != nil {
		return nil, err
	}
	return (&Client{client: sdkClient}).SendMessage(ctx, queueURL, message, opts...)
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
	sdkClient, err := GetClient(ctx, region)
	if err != nil {
		return nil, err
	}
	return (&Client{client: sdkClient}).SendMessageGob(ctx, queueURL, message, opts...)
}

// ReceiveMessage
// aws-sdk-go v2 sqs ReceiveMessage
// default MaxNumberOfMessages=1, WaitTimeSeconds=20, VisibilityTimeout=30
//
// Mocks: Using ctxawslocal.WithContext, you can make requests for local mocks.
func ReceiveMessage(
	ctx context.Context, region awsconfig.Region, queueURL QueueURL, opts ...sqsreceive.ReceiveMessageOption,
) (*sqs.ReceiveMessageOutput, error) {
	sdkClient, err := GetClient(ctx, region)
	if err != nil {
		return nil, err
	}
	return (&Client{client: sdkClient}).ReceiveMessage(ctx, queueURL, opts...)
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

// DeleteMessage
// aws-sdk-go v2 sqs DeleteMessage
//
// Mocks: Using ctxawslocal.WithContext, you can make requests for local mocks.
func DeleteMessage(ctx context.Context, region awsconfig.Region, queueURL QueueURL, message types.Message) error {
	sdkClient, err := GetClient(ctx, region)
	if err != nil {
		return err
	}
	return (&Client{client: sdkClient}).DeleteMessage(ctx, queueURL, message)
}
