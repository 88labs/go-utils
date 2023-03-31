package awssqs_test

import (
	"context"
	"log"
	"testing"
	"time"

	"github.com/88labs/go-utils/aws/awsconfig"

	"github.com/bxcodec/faker/v3"
	"github.com/stretchr/testify/assert"
	"golang.org/x/sync/errgroup"

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
		count++
		message := TestMessageBody{
			ID:   count,
			Name: faker.Name(),
		}
		_, err := awssqs.SendMessage(ctx, TestRegion, TestQueue, message)
		assert.NoError(t, err)
		time.Sleep(5 * time.Second)
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
		count++
		message := TestMessageBody{
			ID:   count,
			Name: faker.Name(),
		}
		_, err := awssqs.SendMessageGob(ctx, TestRegion, TestQueue2, message)
		assert.NoError(t, err)
		time.Sleep(5 * time.Second)
	}

	t.Run("Retrieve And DeleteMessage:1 message", func(t *testing.T) {
		createFixture(t, ctx)
		items, res, err := awssqs.ReceiveMessageGob(ctx, TestRegion, TestQueue2, TestMessageBody{}, sqsreceive.WithWaitTimeSeconds(0))
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
