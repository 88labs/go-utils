package cdc

import (
	"context"
	"fmt"
	"log/slog"
	"runtime/debug"
	"strconv"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"
)

// SQSAPI is the subset of *sqs.Client used (easy to fake in tests).
type SQSAPI interface {
	ReceiveMessage(ctx context.Context, in *sqs.ReceiveMessageInput, opts ...func(*sqs.Options)) (*sqs.ReceiveMessageOutput, error)
	DeleteMessage(ctx context.Context, in *sqs.DeleteMessageInput, opts ...func(*sqs.Options)) (*sqs.DeleteMessageOutput, error)
	ChangeMessageVisibility(ctx context.Context, in *sqs.ChangeMessageVisibilityInput, opts ...func(*sqs.Options)) (*sqs.ChangeMessageVisibilityOutput, error)
}

type Config struct {
	QueueURL string

	MaxMessages       int32         // per poll, 1..10 (default 10)
	WaitTimeSeconds   int32         // long poll (default 20)
	VisibilityTimeout int32         // 0 = queue default
	Concurrency       int           // parallel handlers (default 10)
	HandlerTimeout    time.Duration // 0 = none
	ReceiveBackoff    time.Duration // after a receive error (default 2s)

	// RetryDelay, if set, picks the visibility timeout for a failed message
	// based on its receive count (exponential backoff).
	RetryDelay func(receiveCount int) time.Duration

	// OnError is called for handler/receive errors. Default: slog.Error.
	OnError func(ctx context.Context, err error, meta Meta)
}

type Consumer struct {
	client SQSAPI
	router *Router
	cfg    Config
}

func NewConsumer(client SQSAPI, router *Router, cfg Config) *Consumer {
	if cfg.MaxMessages <= 0 || cfg.MaxMessages > 10 {
		cfg.MaxMessages = 10
	}
	if cfg.WaitTimeSeconds <= 0 {
		cfg.WaitTimeSeconds = 20
	}
	if cfg.Concurrency <= 0 {
		cfg.Concurrency = 10
	}
	if cfg.ReceiveBackoff <= 0 {
		cfg.ReceiveBackoff = 2 * time.Second
	}
	if cfg.OnError == nil {
		cfg.OnError = func(_ context.Context, err error, m Meta) {
			slog.Error("cdc consumer error", "err", err, "sqs_message_id", m.SQSMessageID, "receive_count", m.ReceiveCount)
		}
	}
	// The consumer runs on a private copy; see Router.
	return &Consumer{client: client, router: router.freeze(), cfg: cfg}
}

// Run polls until ctx is cancelled, then waits for in-flight messages to finish.
func (c *Consumer) Run(ctx context.Context) error {
	sem := make(chan struct{}, c.cfg.Concurrency)
	var wg sync.WaitGroup
	defer wg.Wait()

	for ctx.Err() == nil {
		out, err := c.client.ReceiveMessage(ctx, &sqs.ReceiveMessageInput{
			QueueUrl:            aws.String(c.cfg.QueueURL),
			MaxNumberOfMessages: c.cfg.MaxMessages,
			WaitTimeSeconds:     c.cfg.WaitTimeSeconds,
			VisibilityTimeout:   c.cfg.VisibilityTimeout,
			MessageSystemAttributeNames: []types.MessageSystemAttributeName{
				types.MessageSystemAttributeNameApproximateReceiveCount,
			},
		})
		if err != nil {
			if ctx.Err() != nil {
				break
			}
			c.cfg.OnError(ctx, fmt.Errorf("receive: %w", err), Meta{})
			select {
			case <-time.After(c.cfg.ReceiveBackoff):
			case <-ctx.Done():
			}
			continue
		}
		for _, m := range out.Messages {
			sem <- struct{}{}
			wg.Add(1)
			go func(m types.Message) {
				defer wg.Done()
				defer func() { <-sem }()
				c.process(ctx, m)
			}(m)
		}
	}
	return nil
}

func (c *Consumer) process(ctx context.Context, m types.Message) {
	rc, _ := strconv.Atoi(m.Attributes[string(types.MessageSystemAttributeNameApproximateReceiveCount)])
	meta := Meta{SQSMessageID: aws.ToString(m.MessageId), ReceiveCount: rc}

	// Let in-flight work finish on shutdown; ack/nack must still reach SQS.
	hctx := context.WithoutCancel(ctx)
	if c.cfg.HandlerTimeout > 0 {
		var cancel context.CancelFunc
		hctx, cancel = context.WithTimeout(hctx, c.cfg.HandlerTimeout)
		defer cancel()
	}

	err := c.safeDispatch(hctx, aws.ToString(m.Body), meta)
	if err != nil {
		c.cfg.OnError(hctx, err, meta)
		if !IsPermanent(err) {
			if c.cfg.RetryDelay != nil {
				_, _ = c.client.ChangeMessageVisibility(context.WithoutCancel(ctx), &sqs.ChangeMessageVisibilityInput{
					QueueUrl:          aws.String(c.cfg.QueueURL),
					ReceiptHandle:     m.ReceiptHandle,
					VisibilityTimeout: int32(c.cfg.RetryDelay(rc).Seconds()),
				})
			}
			return // stays in queue -> redelivery -> DLQ after maxReceiveCount
		}
	}
	if _, derr := c.client.DeleteMessage(context.WithoutCancel(ctx), &sqs.DeleteMessageInput{
		QueueUrl:      aws.String(c.cfg.QueueURL),
		ReceiptHandle: m.ReceiptHandle,
	}); derr != nil {
		c.cfg.OnError(ctx, fmt.Errorf("delete: %w", derr), meta)
	}
}

func (c *Consumer) safeDispatch(ctx context.Context, body string, meta Meta) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic in handler: %v\n%s", r, debug.Stack())
		}
	}()
	return c.router.dispatch(ctx, body, meta)
}
