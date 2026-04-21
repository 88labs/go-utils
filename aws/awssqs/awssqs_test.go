package awssqs_test

import (
	"context"
	"fmt"
	"log"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"
	"github.com/go-faker/faker/v4"
	"github.com/stretchr/testify/assert"
	"golang.org/x/sync/errgroup"

	"github.com/88labs/go-utils/aws/awsconfig"
	"github.com/88labs/go-utils/aws/awssqs"
	"github.com/88labs/go-utils/aws/awssqs/options/sqsreceive"
	"github.com/88labs/go-utils/aws/ctxawslocal"
)

const (
	TestQueue  = "http://127.0.0.1:29324/000000000000/test-queue"
	TestQueue2 = "http://127.0.0.1:29324/000000000000/test-2-queue"
	TestRegion = awsconfig.RegionTokyo
)

func Cleanup() {
	ctx := ctxawslocal.WithContext(
		context.Background(),
		ctxawslocal.WithAccessKey("DUMMYACCESSKEYEXAMPLE"),
		ctxawslocal.WithSecretAccessKey("DUMMYSECRETKEYEXAMPLE"),
		ctxawslocal.WithSQSEndpoint("http://127.0.0.1:29324"),
	)
	res, err := awssqs.ReceiveMessage(ctx, TestRegion, TestQueue,
		sqsreceive.WithWaitTimeSeconds(0),
		sqsreceive.WithMaxNumberOfMessages(10))
	if err != nil {
		log.Print(err)
		return
	}
	if len(res.Messages) == 0 {
		return
	}
	for _, m := range res.Messages {
		err := awssqs.DeleteMessage(ctx, TestRegion, TestQueue, m)
		if err != nil {
			log.Print(err)
		}
	}
}

// CleanupQueue2 はテスト後に TestQueue2 に残留したメッセージをすべて削除する。
// TestNewClient_* テストが TestQueue2 に JSON メッセージを送信するため、
// 後続の TestReceiveGobAndDeleteMessage が gob デコードに失敗しないよう必ず呼ぶ。
// メッセージがなくなるまでループして確実に空にする。
func CleanupQueue2() {
	ctx := ctxawslocal.WithContext(
		context.Background(),
		ctxawslocal.WithAccessKey("DUMMYACCESSKEYEXAMPLE"),
		ctxawslocal.WithSecretAccessKey("DUMMYSECRETKEYEXAMPLE"),
		ctxawslocal.WithSQSEndpoint("http://127.0.0.1:29324"),
	)
	for {
		res, err := awssqs.ReceiveMessage(ctx, TestRegion, TestQueue2,
			sqsreceive.WithWaitTimeSeconds(0),
			sqsreceive.WithMaxNumberOfMessages(10))
		if err != nil {
			log.Print(err)
			return
		}
		if len(res.Messages) == 0 {
			return
		}
		for _, m := range res.Messages {
			if err := awssqs.DeleteMessage(ctx, TestRegion, TestQueue2, m); err != nil {
				log.Print(err)
			}
		}
	}
}

// waitForMessages は queueURL に少なくとも wantCount 件の可視メッセージが溜まるまで
// GetQueueAttributes をポーリングする。メッセージを受信・消費しないため、
// 後続のテスト本体の ReceiveMessage に影響しない。
// timeout 内に条件を満たさなければ t.Fatal する。
func waitForMessages(t *testing.T, ctx context.Context, queueURL string, wantCount int, timeout time.Duration) {
	t.Helper()
	const interval = 200 * time.Millisecond
	deadline := time.Now().Add(timeout)

	sqsClient, err := awssqs.GetClient(ctx, TestRegion)
	if !assert.NoError(t, err) {
		return
	}

	for {
		out, err := sqsClient.GetQueueAttributes(ctx, &sqs.GetQueueAttributesInput{
			QueueUrl:       &queueURL,
			AttributeNames: []types.QueueAttributeName{types.QueueAttributeNameApproximateNumberOfMessages},
		})
		if err == nil {
			var count int
			if v, ok := out.Attributes[string(types.QueueAttributeNameApproximateNumberOfMessages)]; ok {
				fmt.Sscanf(v, "%d", &count) //nolint:errcheck
			}
			if count >= wantCount {
				return
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %d message(s) in %s after %s", wantCount, queueURL, timeout)
		}
		time.Sleep(interval)
	}
}

func TestSendMessage(t *testing.T) {
	ctx := ctxawslocal.WithContext(
		context.Background(),
		ctxawslocal.WithAccessKey("DUMMYACCESSKEYEXAMPLE"),
		ctxawslocal.WithSecretAccessKey("DUMMYSECRETKEYEXAMPLE"),
		ctxawslocal.WithSQSEndpoint("http://127.0.0.1:29324"),
	)

	t.Cleanup(Cleanup)
	type TestMessageBody struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}
	t.Run("SendMessage", func(t *testing.T) {
		message := TestMessageBody{
			ID:   1,
			Name: faker.Name(),
		}
		res, err := awssqs.SendMessage(ctx, TestRegion, TestQueue, message)
		if !assert.NoError(t, err) {
			return
		}
		assert.NotEmpty(t, res.MessageId)
	})
	t.Run("SendMessage duplicate message 3", func(t *testing.T) {
		message := TestMessageBody{
			ID:   1,
			Name: "duplicate message",
		}
		uniq := make(map[string]struct{})
		for i := 0; i < 3; i++ {
			res, err := awssqs.SendMessage(ctx, TestRegion, TestQueue, message)
			if !assert.NoError(t, err) {
				return
			}
			t.Log(res.MessageId)
			if i > 0 {
				_, ok := uniq[*res.MessageId]
				assert.False(t, ok)
			}
			uniq[*res.MessageId] = struct{}{}
		}
	})
}

func TestSendMessageGob(t *testing.T) {
	ctx := ctxawslocal.WithContext(
		context.Background(),
		ctxawslocal.WithAccessKey("DUMMYACCESSKEYEXAMPLE"),
		ctxawslocal.WithSecretAccessKey("DUMMYSECRETKEYEXAMPLE"),
		ctxawslocal.WithSQSEndpoint("http://127.0.0.1:29324"),
	)

	t.Cleanup(Cleanup)
	type TestMessageBody struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}
	t.Run("SendMessage", func(t *testing.T) {
		message := TestMessageBody{
			ID:   1,
			Name: faker.Name(),
		}
		res, err := awssqs.SendMessageGob(ctx, TestRegion, TestQueue, message)
		if !assert.NoError(t, err) {
			return
		}
		assert.NotEmpty(t, res.MessageId)
	})
	t.Run("SendMessage duplicate message 3", func(t *testing.T) {
		message := TestMessageBody{
			ID:   1,
			Name: "duplicate message",
		}
		uniq := make(map[string]struct{})
		for i := 0; i < 3; i++ {
			res, err := awssqs.SendMessageGob(ctx, TestRegion, TestQueue, message)
			if !assert.NoError(t, err) {
				return
			}
			t.Log(res.MessageId)
			if i > 0 {
				_, ok := uniq[*res.MessageId]
				assert.False(t, ok)
			}
			uniq[*res.MessageId] = struct{}{}
		}
	})
}

func TestReceiveAndDeleteMessage(t *testing.T) {
	ctx := ctxawslocal.WithContext(
		context.Background(),
		ctxawslocal.WithAccessKey("DUMMYACCESSKEYEXAMPLE"),
		ctxawslocal.WithSecretAccessKey("DUMMYSECRETKEYEXAMPLE"),
		ctxawslocal.WithSQSEndpoint("http://127.0.0.1:29324"),
	)

	t.Cleanup(Cleanup)
	type TestMessageBody struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}
	var count int
	createFixture := func(t *testing.T, ctx context.Context) {
		t.Helper()
		count++
		message := TestMessageBody{
			ID:   count,
			Name: faker.Name(),
		}
		_, err := awssqs.SendMessage(ctx, TestRegion, TestQueue, message)
		assert.NoError(t, err)
		waitForMessages(t, ctx, TestQueue, 1, 10*time.Second)
	}

	t.Run("Retrieve And DeleteMessage:1 message", func(t *testing.T) {
		createFixture(t, ctx)
		res, err := awssqs.ReceiveMessage(ctx, TestRegion, TestQueue, sqsreceive.WithWaitTimeSeconds(0))
		if !assert.NoError(t, err) {
			return
		}
		if assert.Greater(t, len(res.Messages), 0) {
			for _, m := range res.Messages {
				err := awssqs.DeleteMessage(ctx, TestRegion, TestQueue, m)
				assert.NoError(t, err)
			}
		}
	})

	t.Run("Do not get the same queue during VisibilityTimeout", func(t *testing.T) {
		createFixture(t, ctx)
		createFixture(t, ctx)
		var (
			eg                     errgroup.Group
			messageID1, messageID2 string
		)
		eg.Go(func() error {
			res, err := awssqs.ReceiveMessage(ctx, TestRegion, TestQueue,
				sqsreceive.WithWaitTimeSeconds(0),
				sqsreceive.WithVisibilityTimeout(5),
			)
			if err != nil {
				return err
			}
			for _, v := range res.Messages {
				messageID1 = *v.MessageId
			}
			return nil
		})
		eg.Go(func() error {
			res, err := awssqs.ReceiveMessage(ctx, TestRegion, TestQueue,
				sqsreceive.WithWaitTimeSeconds(0),
				sqsreceive.WithVisibilityTimeout(5),
			)
			if err != nil {
				return err
			}
			for _, v := range res.Messages {
				messageID2 = *v.MessageId
			}
			return nil
		})
		assert.NoError(t, eg.Wait())
		assert.NotEqual(t, messageID1, messageID2)
	})
}

// TestNewClient_returnsWorkingClient verifies that NewClient constructs a client
// that can successfully send and receive a message on SQS.
func TestNewClient_returnsWorkingClient(t *testing.T) {
	ctx := ctxawslocal.WithContext(
		context.Background(),
		ctxawslocal.WithAccessKey("DUMMYACCESSKEYEXAMPLE"),
		ctxawslocal.WithSecretAccessKey("DUMMYSECRETKEYEXAMPLE"),
		ctxawslocal.WithSQSEndpoint("http://127.0.0.1:29324"),
	)
	t.Cleanup(CleanupQueue2)

	client, err := awssqs.NewClient(ctx, TestRegion)
	if !assert.NoError(t, err) {
		return
	}

	type Msg struct {
		Value string `json:"value"`
	}
	msg := Msg{Value: faker.Name()}
	res, err := client.SendMessage(ctx, TestQueue2, msg)
	if !assert.NoError(t, err) {
		return
	}
	if !assert.NotNil(t, res) {
		return
	}
	assert.NotEmpty(t, res.MessageId)

	waitForMessages(t, ctx, TestQueue2, 1, 10*time.Second)

	recvRes, err := client.ReceiveMessage(ctx, TestQueue2, sqsreceive.WithWaitTimeSeconds(0))
	if !assert.NoError(t, err) {
		return
	}
	if !assert.NotNil(t, recvRes) {
		return
	}
	if !assert.Greater(t, len(recvRes.Messages), 0) {
		return
	}
	err = client.DeleteMessage(ctx, TestQueue2, recvRes.Messages[0])
	assert.NoError(t, err)
}

// TestNewClient_SQSClient_exposesUnderlyingSDKClient verifies that the underlying
// *sqs.Client can be retrieved for advanced usage.
func TestNewClient_SQSClient_exposesUnderlyingSDKClient(t *testing.T) {
	ctx := ctxawslocal.WithContext(
		context.Background(),
		ctxawslocal.WithAccessKey("DUMMYACCESSKEYEXAMPLE"),
		ctxawslocal.WithSecretAccessKey("DUMMYSECRETKEYEXAMPLE"),
		ctxawslocal.WithSQSEndpoint("http://127.0.0.1:29324"),
	)

	client, err := awssqs.NewClient(ctx, TestRegion)
	assert.NoError(t, err)
	assert.NotNil(t, client.SQSClient())
}

// TestNewClient_isIndependentFromSingleton verifies that a Client created via
// NewClient can receive a message that was sent through the package-level singleton.
func TestNewClient_isIndependentFromSingleton(t *testing.T) {
	ctx := ctxawslocal.WithContext(
		context.Background(),
		ctxawslocal.WithAccessKey("DUMMYACCESSKEYEXAMPLE"),
		ctxawslocal.WithSecretAccessKey("DUMMYSECRETKEYEXAMPLE"),
		ctxawslocal.WithSQSEndpoint("http://127.0.0.1:29324"),
	)
	t.Cleanup(CleanupQueue2)

	type Msg struct{ Value string }

	// パッケージレベルのシングルトン経由で送信する。
	_, err := awssqs.SendMessage(ctx, TestRegion, TestQueue2, Msg{Value: faker.Name()})
	assert.NoError(t, err)

	waitForMessages(t, ctx, TestQueue2, 1, 10*time.Second)

	// 別途生成した Client 経由で受信 — 状態を共有せずに成功する必要がある。
	client, err := awssqs.NewClient(ctx, TestRegion)
	assert.NoError(t, err)

	recvRes, err := client.ReceiveMessage(ctx, TestQueue2, sqsreceive.WithWaitTimeSeconds(0))
	assert.NoError(t, err)
	if !assert.Greater(t, len(recvRes.Messages), 0) {
		return
	}
	err = client.DeleteMessage(ctx, TestQueue2, recvRes.Messages[0])
	assert.NoError(t, err)
}
func TestReceiveGobAndDeleteMessage(t *testing.T) {
	ctx := ctxawslocal.WithContext(
		context.Background(),
		ctxawslocal.WithAccessKey("DUMMYACCESSKEYEXAMPLE"),
		ctxawslocal.WithSecretAccessKey("DUMMYSECRETKEYEXAMPLE"),
		ctxawslocal.WithSQSEndpoint("http://127.0.0.1:29324"),
	)

	t.Cleanup(Cleanup)
	type TestMessageBody struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}
	var count int
	createFixture := func(t *testing.T, ctx context.Context) {
		t.Helper()
		count++
		message := TestMessageBody{
			ID:   count,
			Name: faker.Name(),
		}
		_, err := awssqs.SendMessageGob(ctx, TestRegion, TestQueue2, message)
		assert.NoError(t, err)
		waitForMessages(t, ctx, TestQueue2, 1, 10*time.Second)
	}

	t.Run("Retrieve And DeleteMessage:1 message", func(t *testing.T) {
		createFixture(t, ctx)
		items, res, err := awssqs.ReceiveMessageGob(ctx, TestRegion, TestQueue2, TestMessageBody{},
			sqsreceive.WithWaitTimeSeconds(0))
		if !assert.NoError(t, err) {
			return
		}
		if assert.Greater(t, len(items), 0) {
			for _, v := range items {
				assert.NotEmpty(t, v.ID)
				assert.NotEmpty(t, v.Name)
			}
		}
		if assert.Greater(t, len(res.Messages), 0) {
			for _, m := range res.Messages {
				err := awssqs.DeleteMessage(ctx, TestRegion, TestQueue2, m)
				assert.NoError(t, err)
			}
		}
	})

	t.Run("Do not get the same queue during VisibilityTimeout", func(t *testing.T) {
		createFixture(t, ctx)
		createFixture(t, ctx)
		var (
			eg                     errgroup.Group
			messageID1, messageID2 string
		)
		eg.Go(func() error {
			_, res, err := awssqs.ReceiveMessageGob(ctx, TestRegion, TestQueue2, TestMessageBody{},
				sqsreceive.WithWaitTimeSeconds(0),
				sqsreceive.WithVisibilityTimeout(5),
			)
			if err != nil {
				return err
			}
			for _, v := range res.Messages {
				messageID1 = *v.MessageId
			}
			return nil
		})
		eg.Go(func() error {
			_, res, err := awssqs.ReceiveMessageGob(ctx, TestRegion, TestQueue2, TestMessageBody{},
				sqsreceive.WithWaitTimeSeconds(0),
				sqsreceive.WithVisibilityTimeout(5),
			)
			if err != nil {
				return err
			}
			for _, v := range res.Messages {
				messageID2 = *v.MessageId
			}
			return nil
		})
		assert.NoError(t, eg.Wait())
		assert.NotEqual(t, messageID1, messageID2)
	})
}
