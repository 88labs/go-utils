package awss3_test

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/bxcodec/faker/v3"
	"github.com/stretchr/testify/assert"

	"github.com/88labs/go-utils/aws/awsconfig"
	"github.com/88labs/go-utils/aws/awss3"
	"github.com/88labs/go-utils/aws/awss3/options/s3donwload"
	"github.com/88labs/go-utils/aws/ctxawslocal"
	"github.com/88labs/go-utils/ulid"
)

const (
	TestBucket = "test"
	TestRegion = awsconfig.RegionTokyo
)

func TestHeadObject(t *testing.T) {
	ctx := ctxawslocal.WithContext(
		context.Background(),
		ctxawslocal.WithAccessKey("DUMMYACCESSKEYEXAMPLE"),
		ctxawslocal.WithSecretAccessKey("DUMMYSECRETKEYEXAMPLE"),
	)
	s3Client, err := awss3.GetClient(ctx, TestRegion)
	assert.NoError(t, err)

	createFixture := func() awss3.Key {
		key := fmt.Sprintf("awstest/%s.txt", ulid.MustNew())
		uploader := manager.NewUploader(s3Client)
		input := s3.PutObjectInput{
			Body:    strings.NewReader("test"),
			Bucket:  aws.String(TestBucket),
			Key:     aws.String(key),
			Expires: aws.Time(time.Now().Add(10 * time.Minute)),
		}
		if _, err := uploader.Upload(ctx, &input); err != nil {
			assert.NoError(t, err)
		}
		return awss3.Key(key)
	}

	t.Run("exists object", func(t *testing.T) {
		key := createFixture()
		_, err := awss3.HeadObject(ctx, TestRegion, TestBucket, key)
		assert.NoError(t, err)
	})
	t.Run("not exists object", func(t *testing.T) {
		_, err := awss3.HeadObject(ctx, TestRegion, TestBucket, "NOT_FOUND")
		assert.Error(t, err)
	})
}

func TestGetObject(t *testing.T) {
	ctx := ctxawslocal.WithContext(
		context.Background(),
		ctxawslocal.WithAccessKey("DUMMYACCESSKEYEXAMPLE"),
		ctxawslocal.WithSecretAccessKey("DUMMYSECRETKEYEXAMPLE"),
	)
	s3Client, err := awss3.GetClient(ctx, TestRegion)
	assert.NoError(t, err)

	createFixture := func() awss3.Key {
		key := fmt.Sprintf("awstest/%s.txt", ulid.MustNew())
		uploader := manager.NewUploader(s3Client)
		input := s3.PutObjectInput{
			Body:    strings.NewReader("test"),
			Bucket:  aws.String(TestBucket),
			Key:     aws.String(key),
			Expires: aws.Time(time.Now().Add(10 * time.Minute)),
		}
		if _, err := uploader.Upload(ctx, &input); err != nil {
			assert.NoError(t, err)
		}
		return awss3.Key(key)
	}

	t.Run("GetObject", func(t *testing.T) {
		key := createFixture()
		var buf bytes.Buffer
		err := awss3.GetObject(ctx, TestRegion, TestBucket, key, &buf)
		assert.NoError(t, err)
		assert.Equal(t, "test", string(buf.Bytes()))
	})
}

func TestDownloadFiles(t *testing.T) {
	ctx := ctxawslocal.WithContext(
		context.Background(),
		ctxawslocal.WithAccessKey("DUMMYACCESSKEYEXAMPLE"),
		ctxawslocal.WithSecretAccessKey("DUMMYSECRETKEYEXAMPLE"),
	)
	s3Client, err := awss3.GetClient(ctx, TestRegion)
	assert.NoError(t, err)

	keys := make(awss3.Keys, 3)
	for i := 0; i < 3; i++ {
		key := fmt.Sprintf("awstest/%s.txt", ulid.MustNew())
		uploader := manager.NewUploader(s3Client)
		input := s3.PutObjectInput{
			Body:    strings.NewReader("test"),
			Bucket:  aws.String(TestBucket),
			Key:     aws.String(key),
			Expires: aws.Time(time.Now().Add(10 * time.Minute)),
		}
		if _, err := uploader.Upload(ctx, &input); err != nil {
			assert.NoError(t, err)
			return
		}
		keys[i] = awss3.Key(key)
	}

	t.Run("no option", func(t *testing.T) {
		filePaths, finish, err := awss3.DownloadFiles(ctx, TestRegion, TestBucket, keys)
		assert.NoError(t, err)
		if assert.Len(t, filePaths, len(keys)) {
			for i, v := range filePaths {
				assert.Equal(t, filepath.Base(keys[i].String()), filepath.Base(v))
				fileBody, err := os.ReadFile(v)
				assert.NoError(t, err)
				assert.Equal(t, "test", string(fileBody))
			}
		}
		err = finish()
		assert.NoError(t, err)
	})
	t.Run("FileNameReplacer:not duplicate", func(t *testing.T) {
		filePaths, finish, err := awss3.DownloadFiles(ctx, TestRegion, TestBucket, keys,
			s3donwload.WithFileNameReplacerFunc(func(S3Key, baseFileName string) string {
				return "add_" + baseFileName
			}),
		)
		assert.NoError(t, err)
		if assert.Len(t, filePaths, len(keys)) {
			for i, v := range filePaths {
				assert.Equal(t, "add_"+filepath.Base(keys[i].String()), filepath.Base(v))
				fileBody, err := os.ReadFile(v)
				assert.NoError(t, err)
				assert.Equal(t, "test", string(fileBody))
			}
		}
		err = finish()
		assert.NoError(t, err)
	})
	t.Run("FileNameReplacer:duplicate", func(t *testing.T) {
		filePaths, finish, err := awss3.DownloadFiles(ctx, TestRegion, TestBucket, keys,
			s3donwload.WithFileNameReplacerFunc(func(S3Key, baseFileName string) string {
				return "fixname.txt"
			}),
		)
		assert.NoError(t, err)
		if assert.Len(t, filePaths, len(keys)) {
			for i, v := range filePaths {
				if i == 0 {
					assert.Equal(t, "fixname.txt", filepath.Base(v))
				} else {
					assert.Equal(t, fmt.Sprintf("fixname_%d.txt", i+1), filepath.Base(v))
				}
				fileBody, err := os.ReadFile(v)
				assert.NoError(t, err)
				assert.Equal(t, "test", string(fileBody))
			}
		}
		err = finish()
		assert.NoError(t, err)
	})
}

func TestPutObject(t *testing.T) {
	ctx := ctxawslocal.WithContext(
		context.Background(),
		ctxawslocal.WithAccessKey("DUMMYACCESSKEYEXAMPLE"),
		ctxawslocal.WithSecretAccessKey("DUMMYSECRETKEYEXAMPLE"),
	)

	t.Run("PutObject", func(t *testing.T) {
		key := fmt.Sprintf("awstest/%s.txt", ulid.MustNew())
		body := faker.Sentence()
		_, err := awss3.PutObject(ctx, TestRegion, TestBucket, awss3.Key(key), strings.NewReader(body))
		assert.NoError(t, err)
		filePaths, finish, err := awss3.DownloadFiles(ctx, TestRegion, TestBucket, awss3.NewKeys(key))
		assert.NoError(t, err)
		assert.Len(t, filePaths, 1)
		fileBody, err := os.ReadFile(filePaths[0])
		assert.NoError(t, err)
		assert.Equal(t, body, string(fileBody))
		err = finish()
		assert.NoError(t, err)
	})
}

func TestUploadManager(t *testing.T) {
	ctx := ctxawslocal.WithContext(
		context.Background(),
		ctxawslocal.WithAccessKey("DUMMYACCESSKEYEXAMPLE"),
		ctxawslocal.WithSecretAccessKey("DUMMYSECRETKEYEXAMPLE"),
	)

	t.Run("UploadManager", func(t *testing.T) {
		key := fmt.Sprintf("awstest/%s.txt", ulid.MustNew())
		body := faker.Sentence()
		_, err := awss3.UploadManager(ctx, TestRegion, TestBucket, awss3.Key(key), strings.NewReader(body))
		assert.NoError(t, err)
		filePaths, finish, err := awss3.DownloadFiles(ctx, TestRegion, TestBucket, awss3.NewKeys(key))
		assert.NoError(t, err)
		assert.Len(t, filePaths, 1)
		fileBody, err := os.ReadFile(filePaths[0])
		assert.NoError(t, err)
		assert.Equal(t, body, string(fileBody))
		err = finish()
		assert.NoError(t, err)
	})
}

func TestPresign(t *testing.T) {
	ctx := ctxawslocal.WithContext(
		context.Background(),
		ctxawslocal.WithAccessKey("DUMMYACCESSKEYEXAMPLE"),
		ctxawslocal.WithSecretAccessKey("DUMMYSECRETKEYEXAMPLE"),
	)
	s3Client, err := awss3.GetClient(ctx, TestRegion)
	if err != nil {
		t.Error(err)
		return
	}
	key := fmt.Sprintf("awstest/%s.txt", ulid.MustNew())
	uploader := manager.NewUploader(s3Client)
	input := s3.PutObjectInput{
		Body:    strings.NewReader("test"),
		Bucket:  aws.String(TestBucket),
		Key:     aws.String(key),
		Expires: aws.Time(time.Now().Add(10 * time.Minute)),
	}
	if _, err := uploader.Upload(ctx, &input); err != nil {
		assert.NoError(t, err)
		return
	}

	t.Run("Presign", func(t *testing.T) {
		presign, err := awss3.Presign(ctx, TestRegion, TestBucket, awss3.Key(key))
		assert.NoError(t, err)
		assert.NotEmpty(t, presign)
	})
}

func TestResponseContentDisposition(t *testing.T) {
	const fileName = ",あいうえお　牡蠣喰家来 サシスセソ@+$_-^|+{}"
	t.Run("success", func(t *testing.T) {
		actual := awss3.ResponseContentDisposition(fileName)
		assert.NotEmpty(t, actual)
	})
}

func TestCopy(t *testing.T) {
	createFixture := func(ctx context.Context) awss3.Key {
		s3Client, err := awss3.GetClient(ctx, TestRegion)
		if err != nil {
			t.Error(err)
			t.FailNow()
		}
		key := fmt.Sprintf("awstest/%s.txt", ulid.MustNew())
		uploader := manager.NewUploader(s3Client)
		input := s3.PutObjectInput{
			Body:    strings.NewReader("test"),
			Bucket:  aws.String(TestBucket),
			Key:     aws.String(key),
			Expires: aws.Time(time.Now().Add(10 * time.Minute)),
		}
		if _, err := uploader.Upload(ctx, &input); err != nil {
			assert.NoError(t, err)
			t.FailNow()
		}
		waiter := s3.NewObjectExistsWaiter(s3Client)
		if err := waiter.Wait(ctx,
			&s3.HeadObjectInput{Bucket: aws.String(TestBucket), Key: aws.String(key)},
			time.Second,
		); err != nil {
			assert.NoError(t, err)
			t.FailNow()
		}
		return awss3.Key(key)
	}

	t.Run("Copy:Same Bucket and Other Key", func(t *testing.T) {
		ctx := ctxawslocal.WithContext(
			context.Background(),
			ctxawslocal.WithAccessKey("DUMMYACCESSKEYEXAMPLE"),
			ctxawslocal.WithSecretAccessKey("DUMMYSECRETKEYEXAMPLE"),
		)
		key := createFixture(ctx)
		key2 := awss3.Key(fmt.Sprintf("awstest/%s.txt", ulid.MustNew()))
		assert.NoError(t, awss3.Copy(ctx, TestRegion, TestBucket, key, key2))
	})
	t.Run("Copy:Same Item", func(t *testing.T) {
		ctx := ctxawslocal.WithContext(
			context.Background(),
			ctxawslocal.WithAccessKey("DUMMYACCESSKEYEXAMPLE"),
			ctxawslocal.WithSecretAccessKey("DUMMYSECRETKEYEXAMPLE"),
		)
		key := createFixture(ctx)
		assert.NoError(t, awss3.Copy(ctx, TestRegion, TestBucket, key, key))
	})
}
