package global_test

import (
	"bytes"
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/88labs/go-utils/ulid"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/stretchr/testify/assert"

	"github.com/88labs/go-utils/aws/awsconfig"
	"github.com/88labs/go-utils/aws/awss3"
	"github.com/88labs/go-utils/aws/awss3/options/global/s3dialer"
	"github.com/88labs/go-utils/aws/ctxawslocal"
)

const (
	TestBucket = "test"
	TestRegion = awsconfig.RegionTokyo
)

func TestGlobalOptionWithHeadObject(t *testing.T) {
	ctx := ctxawslocal.WithContext(
		context.Background(),
		ctxawslocal.WithS3Endpoint("http://127.0.0.1:29000"), // use Minio
		ctxawslocal.WithAccessKey("DUMMYACCESSKEYEXAMPLE"),
		ctxawslocal.WithSecretAccessKey("DUMMYSECRETKEYEXAMPLE"),
	)
	s3Client, err := awss3.GetClient(ctx, TestRegion)
	assert.NoError(t, err)

	createFixture := func(fileSize int) awss3.Key {
		key := fmt.Sprintf("awstest/%s.txt", ulid.MustNew())
		uploader := manager.NewUploader(s3Client)
		input := s3.PutObjectInput{
			Body:    bytes.NewReader(bytes.Repeat([]byte{1}, fileSize)),
			Bucket:  aws.String(TestBucket),
			Key:     aws.String(key),
			Expires: aws.Time(time.Now().Add(10 * time.Minute)),
		}
		if _, err := uploader.Upload(ctx, &input); err != nil {
			assert.NoError(t, err)
		}
		return awss3.Key(key)
	}

	t.Run("If the option is specified", func(t *testing.T) {
		key := createFixture(100)
		dialer := s3dialer.NewConfGlobalDialer()
		dialer.WithTimeout(time.Second)
		dialer.WithKeepAlive(2 * time.Second)
		dialer.WithDeadline(time.Now().Add(time.Second))
		awss3.GlobalDialer = dialer
		res, err := awss3.HeadObject(ctx, TestRegion, TestBucket, key)
		assert.NoError(t, err)
		assert.Equal(t, aws.Int64(100), res.ContentLength)
	})
}
