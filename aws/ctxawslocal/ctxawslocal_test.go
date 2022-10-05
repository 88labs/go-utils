package ctxawslocal_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/88labs/go-utils/aws/ctxawslocal"
)

func TestIsLocal(t *testing.T) {
	t.Run("context.Value not exists", func(t *testing.T) {
		ctx := context.Background()
		assert.False(t, ctxawslocal.IsLocal(ctx))
	})
	t.Run("context.Value exists", func(t *testing.T) {
		ctx := ctxawslocal.WithContext(context.Background())
		assert.True(t, ctxawslocal.IsLocal(ctx))
	})
}

func TestGetConf(t *testing.T) {
	t.Run("context.Value not exists", func(t *testing.T) {
		ctx := context.Background()
		_, ok := ctxawslocal.GetConf(ctx)
		assert.False(t, ok)
	})
	t.Run("unspecified config", func(t *testing.T) {
		ctx := ctxawslocal.WithContext(context.Background())
		c, ok := ctxawslocal.GetConf(ctx)
		assert.True(t, ok)
		assert.Equal(t, &ctxawslocal.ConfMock{
			AccessKey:       "test",             //localstack default AccessKey
			SecretAccessKey: "test",             // localstack default SecretAccessKey
			Endpoint:        "http://127.0.0.1", // localhost
			S3Port:          4566,               // localstack default s3 port
		}, c)
	})
	t.Run("set config", func(t *testing.T) {
		ctx := ctxawslocal.WithContext(context.Background(),
			ctxawslocal.WithAccessKey("DUMMYACCESSKEYEXAMPLE"),
			ctxawslocal.WithSecretAccessKey("DUMMYACCESSKEYEXAMPLE"),
			ctxawslocal.WithEndpoint("http://localhost"),
			ctxawslocal.WithS3Port(14572),
		)
		c, ok := ctxawslocal.GetConf(ctx)
		assert.True(t, ok)
		assert.Equal(t, &ctxawslocal.ConfMock{
			AccessKey:       "DUMMYACCESSKEYEXAMPLE",
			SecretAccessKey: "DUMMYACCESSKEYEXAMPLE",
			Endpoint:        "http://localhost",
			S3Port:          14572,
		}, c)
	})
}
