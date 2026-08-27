package awssqs

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/gob"

	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"

	"github.com/88labs/go-utils/aws/awsconfig"
	"github.com/88labs/go-utils/aws/awssqs/options/sqsprocess"
	"github.com/88labs/go-utils/aws/awssqs/options/sqsreceive"
	"github.com/88labs/go-utils/aws/awssqs/options/sqssend"
)

// SendMessage converts a value to JSON and sends it to SQS. When tracing is
// enabled with WithTrace, the W3C propagator and trace provider must be usable
// before the SQS request is sent. Traceparent,
// tracestate consume two reserved SQS message attributes, leaving at most eight
// application attributes. Baggage is propagated only when a custom propagator
// is supplied. The operation-specific options apply only to this call.
//
// Mocks: Using ctxawslocal.WithContext, you can make requests for local mocks.
func SendMessage(
	ctx context.Context, region awsconfig.Region, queueURL QueueURL, message any, opts ...sqssend.SendMessageOption,
) (*sqs.SendMessageOutput, error) {
	sdkClient, err := GetClient(ctx, region)
	if err != nil {
		return nil, err
	}
	return packageClientFromSDK(sdkClient).SendMessage(ctx, queueURL, message, opts...)
}

// SendMessageBatch sends a batch of already encoded SQS messages. With tracing,
// each entry gets a create span and the batch gets a send span linked to those
// creation contexts. SQS may return Failed entries with a nil Go error; the
// returned output is preserved and callers must inspect Failed. Batch options
// apply only to this call.
func SendMessageBatch(
	ctx context.Context,
	region awsconfig.Region,
	queueURL QueueURL,
	entries []types.SendMessageBatchRequestEntry,
	opts ...sqssend.SendMessageBatchOption,
) (*sqs.SendMessageBatchOutput, error) {
	sdkClient, err := GetClient(ctx, region)
	if err != nil {
		return nil, err
	}
	return packageClientFromSDK(sdkClient).SendMessageBatch(ctx, queueURL, entries, opts...)
}

// SendMessageGob converts a value to gob encoding and sends it to SQS. The
// tracing and message-attribute contract is the same as SendMessage.
//
// Mocks: Using ctxawslocal.WithContext, you can make requests for local mocks.
func SendMessageGob(
	ctx context.Context, region awsconfig.Region, queueURL QueueURL, message any, opts ...sqssend.SendMessageOption,
) (*sqs.SendMessageOutput, error) {
	sdkClient, err := GetClient(ctx, region)
	if err != nil {
		return nil, err
	}
	return packageClientFromSDK(sdkClient).SendMessageGob(ctx, queueURL, message, opts...)
}

// ReceiveMessage receives messages from SQS. It remains a thin SDK wrapper;
// use ProcessMessage to apply traced processing and acknowledgement semantics.
//
// Mocks: Using ctxawslocal.WithContext, you can make requests for local mocks.
func ReceiveMessage(
	ctx context.Context, region awsconfig.Region, queueURL QueueURL, opts ...sqsreceive.ReceiveMessageOption,
) (*sqs.ReceiveMessageOutput, error) {
	sdkClient, err := GetClient(ctx, region)
	if err != nil {
		return nil, err
	}
	return packageClientFromSDK(sdkClient).ReceiveMessage(ctx, queueURL, opts...)
}

// ReceiveMessageGob receives messages from SQS and decodes their gob bodies.
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

// DeleteMessage deletes a message from SQS. When tracing is enabled, the
// acknowledgement is represented by a delete span.
//
// Mocks: Using ctxawslocal.WithContext, you can make requests for local mocks.
func DeleteMessage(ctx context.Context, region awsconfig.Region, queueURL QueueURL, message types.Message) error {
	sdkClient, err := GetClient(ctx, region)
	if err != nil {
		return err
	}
	return packageClientFromSDK(sdkClient).DeleteMessage(ctx, queueURL, message)
}

// ProcessMessage processes one received message and deletes it only when the
// handler returns nil. The worker context is the process-span parent and the
// sender context is a span link passed to the handler as a tracers.TraceInfo
// value for log correlation. Invalid incoming trace attributes are recorded on
// the process span but do not stop the handler or acknowledgement. Process
// options apply only to this call. Package-level clients use only the WithTrace
// configuration supplied to GetClient's first initialization; later GetClient
// calls do not reconfigure the singleton.
func ProcessMessage(
	ctx context.Context,
	region awsconfig.Region,
	queueURL QueueURL,
	message types.Message,
	handler MessageHandler,
	opts ...sqsprocess.ProcessMessageOption,
) error {
	sdkClient, err := GetClient(ctx, region)
	if err != nil {
		return err
	}
	return packageClientFromSDK(sdkClient).ProcessMessage(ctx, queueURL, message, handler, opts...)
}
