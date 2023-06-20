package awss3_test

import (
	"bytes"
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/88labs/go-utils/aws/awss3/options/global/s3dialer"

	"github.com/88labs/go-utils/aws/awss3/options/s3list"

	"github.com/88labs/go-utils/aws/awss3/options/s3head"

	"github.com/88labs/go-utils/utf8bom"

	"github.com/aws/aws-sdk-go-v2/service/s3/types"

	"github.com/88labs/go-utils/aws/awss3/options/s3selectcsv"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/bxcodec/faker/v3"
	"github.com/stretchr/testify/assert"

	"github.com/88labs/go-utils/aws/awsconfig"
	"github.com/88labs/go-utils/aws/awss3"
	"github.com/88labs/go-utils/aws/awss3/options/s3download"
	"github.com/88labs/go-utils/aws/awss3/options/s3presigned"
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

	t.Run("exists object", func(t *testing.T) {
		key := createFixture(100)
		res, err := awss3.HeadObject(ctx, TestRegion, TestBucket, key)
		assert.NoError(t, err)
		assert.Equal(t, int64(100), res.ContentLength)
	})
	t.Run("not exists object", func(t *testing.T) {
		_, err := awss3.HeadObject(ctx, TestRegion, TestBucket, "NOT_FOUND")
		if assert.Error(t, err) {
			assert.ErrorIs(t, awss3.ErrNotFound, err)
		}
	})
	t.Run("exists object use Waiter", func(t *testing.T) {
		key := createFixture(100)
		res, err := awss3.HeadObject(ctx, TestRegion, TestBucket, key,
			s3head.WithTimeout(5*time.Second),
		)
		assert.NoError(t, err)
		assert.Equal(t, int64(100), res.ContentLength)
	})
	t.Run("not exists object use Waiter", func(t *testing.T) {
		_, err := awss3.HeadObject(ctx, TestRegion, TestBucket, "NOT_FOUND",
			s3head.WithTimeout(5*time.Second),
		)
		if assert.Error(t, err) {
			assert.ErrorIs(t, awss3.ErrNotFound, errors.Unwrap(err))
		}
	})
}

func TestListObjects(t *testing.T) {
	ctx := ctxawslocal.WithContext(
		context.Background(),
		ctxawslocal.WithS3Endpoint("http://127.0.0.1:29000"), // use Minio
		ctxawslocal.WithAccessKey("DUMMYACCESSKEYEXAMPLE"),
		ctxawslocal.WithSecretAccessKey("DUMMYSECRETKEYEXAMPLE"),
	)
	s3Client, err := awss3.GetClient(ctx, TestRegion)
	assert.NoError(t, err)

	createFixture := func(prefix string) awss3.Key {
		key := fmt.Sprintf("%s/awstest/%s.txt", prefix, ulid.MustNew())
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

	t.Run("ListObjects", func(t *testing.T) {
		key1 := createFixture("hoge")
		key2 := createFixture("hoge")
		key3 := createFixture("hoge")
		res, err := awss3.ListObjects(ctx, TestRegion, TestBucket)
		assert.NoError(t, err)
		if _, ok := res.Find(key1); !ok {
			t.Errorf("%s not found", key1)
		}
		if _, ok := res.Find(key2); !ok {
			t.Errorf("%s not found", key2)
		}
		if _, ok := res.Find(key3); !ok {
			t.Errorf("%s not found", key3)
		}
	})

	t.Run("ListObjects OptionsPrefix", func(t *testing.T) {
		key1 := createFixture("hoge")
		key2 := createFixture("hoge")
		key3 := createFixture("fuga")
		res, err := awss3.ListObjects(ctx, TestRegion, TestBucket, s3list.WithPrefix("hoge"))
		assert.NoError(t, err)
		if _, ok := res.Find(key1); !ok {
			t.Errorf("%s not found", key1)
		}
		if _, ok := res.Find(key2); !ok {
			t.Errorf("%s not found", key2)
		}
		if _, ok := res.Find(key3); ok {
			t.Errorf("%s found", key3)
		}
	})

	t.Run("ListObjects 1001 objects", func(t *testing.T) {
		keys := make([]awss3.Key, 1001)
		for i := 0; i < 1001; i++ {
			keys[i] = createFixture("piyo")
		}
		res, err := awss3.ListObjects(ctx, TestRegion, TestBucket)
		assert.NoError(t, err)
		for _, key := range keys {
			if _, ok := res.Find(key); !ok {
				t.Errorf("%s not found", key)
			}
		}
	})
}

func TestGetObject(t *testing.T) {
	ctx := ctxawslocal.WithContext(
		context.Background(),
		ctxawslocal.WithS3Endpoint("http://127.0.0.1:29000"), // use Minio
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

	t.Run("GetObjectWriter", func(t *testing.T) {
		key := createFixture()
		var buf bytes.Buffer
		err := awss3.GetObjectWriter(ctx, TestRegion, TestBucket, key, &buf)
		assert.NoError(t, err)
		assert.Equal(t, "test", string(buf.Bytes()))
	})

	t.Run("GetObjectWriter NotFound", func(t *testing.T) {
		var buf bytes.Buffer
		err := awss3.GetObjectWriter(ctx, TestRegion, TestBucket, "NOT_FOUND", &buf)
		if assert.Error(t, err) {
			assert.ErrorIs(t, awss3.ErrNotFound, err)
		}
	})
}

func TestDeleteObject(t *testing.T) {
	ctx := ctxawslocal.WithContext(
		context.Background(),
		ctxawslocal.WithS3Endpoint("http://127.0.0.1:29000"), // use Minio
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

	t.Run("DeleteObject", func(t *testing.T) {
		key := createFixture()
		_, err := awss3.DeleteObject(ctx, TestRegion, TestBucket, key)
		assert.NoError(t, err)
		_, err = awss3.HeadObject(ctx, TestRegion, TestBucket, key)
		assert.Error(t, err)
	})

	t.Run("DeleteObject NotFound", func(t *testing.T) {
		_, err := awss3.DeleteObject(ctx, TestRegion, TestBucket, "NOT_FOUND")
		if assert.Error(t, err) {
			assert.ErrorIs(t, awss3.ErrNotFound, err)
		}
	})
}

func TestDownloadFiles(t *testing.T) {
	ctx := ctxawslocal.WithContext(
		context.Background(),
		ctxawslocal.WithS3Endpoint("http://127.0.0.1:29000"), // use Minio
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
		filePaths, err := awss3.DownloadFiles(ctx, TestRegion, TestBucket, keys, t.TempDir())
		assert.NoError(t, err)
		if assert.Len(t, filePaths, len(keys)) {
			for i, v := range filePaths {
				assert.Equal(t, filepath.Base(keys[i].String()), filepath.Base(v))
				fileBody, err := os.ReadFile(v)
				assert.NoError(t, err)
				assert.Equal(t, "test", string(fileBody))
			}
		}
	})
	t.Run("FileNameReplacer:not duplicate", func(t *testing.T) {
		filePaths, err := awss3.DownloadFiles(ctx, TestRegion, TestBucket, keys, t.TempDir(),
			s3download.WithFileNameReplacerFunc(func(S3Key, baseFileName string) string {
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
	})
	t.Run("FileNameReplacer:duplicate", func(t *testing.T) {
		filePaths, err := awss3.DownloadFiles(ctx, TestRegion, TestBucket, keys, t.TempDir(),
			s3download.WithFileNameReplacerFunc(func(S3Key, baseFileName string) string {
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
	})
}

func TestPutObject(t *testing.T) {
	ctx := ctxawslocal.WithContext(
		context.Background(),
		ctxawslocal.WithS3Endpoint("http://127.0.0.1:29000"), // use Minio
		ctxawslocal.WithAccessKey("DUMMYACCESSKEYEXAMPLE"),
		ctxawslocal.WithSecretAccessKey("DUMMYSECRETKEYEXAMPLE"),
	)

	t.Run("PutObject", func(t *testing.T) {
		key := fmt.Sprintf("awstest/%s.txt", ulid.MustNew())
		body := faker.Sentence()
		_, err := awss3.PutObject(ctx, TestRegion, TestBucket, awss3.Key(key), strings.NewReader(body))
		assert.NoError(t, err)
		filePaths, err := awss3.DownloadFiles(ctx, TestRegion, TestBucket, awss3.NewKeys(key), t.TempDir())
		assert.NoError(t, err)
		assert.Len(t, filePaths, 1)
		fileBody, err := os.ReadFile(filePaths[0])
		assert.NoError(t, err)
		assert.Equal(t, body, string(fileBody))
	})
}

func TestUploadManager(t *testing.T) {
	ctx := ctxawslocal.WithContext(
		context.Background(),
		ctxawslocal.WithS3Endpoint("http://127.0.0.1:29000"), // use Minio
		ctxawslocal.WithAccessKey("DUMMYACCESSKEYEXAMPLE"),
		ctxawslocal.WithSecretAccessKey("DUMMYSECRETKEYEXAMPLE"),
	)

	t.Run("UploadManager", func(t *testing.T) {
		key := fmt.Sprintf("awstest/%s.txt", ulid.MustNew())
		body := faker.Sentence()
		_, err := awss3.UploadManager(ctx, TestRegion, TestBucket, awss3.Key(key), strings.NewReader(body))
		assert.NoError(t, err)
		filePaths, err := awss3.DownloadFiles(ctx, TestRegion, TestBucket, awss3.NewKeys(key), t.TempDir())
		assert.NoError(t, err)
		assert.Len(t, filePaths, 1)
		fileBody, err := os.ReadFile(filePaths[0])
		assert.NoError(t, err)
		assert.Equal(t, body, string(fileBody))
		assert.NoError(t, err)
	})
}

func TestPresign(t *testing.T) {
	ctx := ctxawslocal.WithContext(
		context.Background(),
		ctxawslocal.WithS3Endpoint("http://127.0.0.1:29000"), // use Minio
		ctxawslocal.WithAccessKey("DUMMYACCESSKEYEXAMPLE"),
		ctxawslocal.WithSecretAccessKey("DUMMYSECRETKEYEXAMPLE"),
	)
	uploadText := func() awss3.Key {
		s3Client, err := awss3.GetClient(ctx, TestRegion)
		if err != nil {
			t.Fatal(err)
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
			return ""
		}
		return awss3.Key(key)
	}
	uploadPDF := func() awss3.Key {
		s3Client, err := awss3.GetClient(ctx, TestRegion)
		if err != nil {
			t.Fatal(err)
		}
		key := fmt.Sprintf("awstest/%s.pdf", ulid.MustNew())
		uploader := manager.NewUploader(s3Client)
		input := s3.PutObjectInput{
			Body:    strings.NewReader("test"),
			Bucket:  aws.String(TestBucket),
			Key:     aws.String(key),
			Expires: aws.Time(time.Now().Add(10 * time.Minute)),
		}
		if _, err := uploader.Upload(ctx, &input); err != nil {
			assert.NoError(t, err)
			return ""
		}
		return awss3.Key(key)
	}

	t.Run("Presign", func(t *testing.T) {
		key := uploadText()
		presign, err := awss3.Presign(ctx, TestRegion, TestBucket, key)
		assert.NoError(t, err)
		assert.NotEmpty(t, presign)
	})
	t.Run("Presign PDF", func(t *testing.T) {
		key := uploadPDF()
		presign, err := awss3.Presign(ctx, TestRegion, TestBucket, key)
		assert.NoError(t, err)
		assert.NotEmpty(t, presign)
	})
	t.Run("Presign NotFound", func(t *testing.T) {
		_, err := awss3.Presign(ctx, TestRegion, TestBucket, "NOT_FOUND")
		if assert.Error(t, err) {
			assert.ErrorIs(t, awss3.ErrNotFound, err)
		}
	})
}

func TestResponseContentDisposition(t *testing.T) {
	const fileName = ",あいうえお　牡蠣喰家来 サシスセソ@+$_-^|+{}"
	t.Run("success", func(t *testing.T) {
		actual := awss3.ResponseContentDisposition(s3presigned.ContentDispositionTypeAttachment, fileName)
		assert.NotEmpty(t, actual)
	})
}

func TestCopy(t *testing.T) {
	createFixture := func(ctx context.Context) awss3.Key {
		s3Client, err := awss3.GetClient(ctx, TestRegion)
		if err != nil {
			t.Fatal(err)
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
			ctxawslocal.WithS3Endpoint("http://127.0.0.1:29000"), // use Minio
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
			ctxawslocal.WithS3Endpoint("http://127.0.0.1:29000"), // use Minio
			ctxawslocal.WithAccessKey("DUMMYACCESSKEYEXAMPLE"),
			ctxawslocal.WithSecretAccessKey("DUMMYSECRETKEYEXAMPLE"),
		)
		key := createFixture(ctx)
		assert.NoError(t, awss3.Copy(ctx, TestRegion, TestBucket, key, key))
	})

	t.Run("Copy:NotFound", func(t *testing.T) {
		ctx := ctxawslocal.WithContext(
			context.Background(),
			ctxawslocal.WithS3Endpoint("http://127.0.0.1:29000"), // use Minio
			ctxawslocal.WithAccessKey("DUMMYACCESSKEYEXAMPLE"),
			ctxawslocal.WithSecretAccessKey("DUMMYSECRETKEYEXAMPLE"),
		)
		err := awss3.Copy(ctx, TestRegion, TestBucket, "NOT_FOUND", "NOT_FOUND")
		if assert.Error(t, err) {
			assert.ErrorIs(t, awss3.ErrNotFound, err)
		}
	})
}

func TestSelectCSVAll(t *testing.T) {
	type TestCSV string
	const (
		TestCSVHeader TestCSV = `id,name,detail
1,hoge,あ髙社🍣
2,fuga,い髙社🍣
3,piyo,う髙社🍣
`
		TestCSVNoHeader TestCSV = `1,hoge,あ髙社🍣
2,fuga,い髙社🍣
3,piyo,う髙社🍣
`
		TestCSVWithLineFeedLF_LF     TestCSV = "id,name,detail\n1,hoge,\"あ髙\n社🍣\"\n2,fuga,\"い髙\n社🍣\"\n3,piyo,\"う髙\n社🍣\""
		TestCSVWithLineFeedLF_CRLF   TestCSV = "id,name,detail\n1,hoge,\"あ髙\r\n社🍣\"\n2,fuga,\"い髙\r\n社🍣\"\n3,piyo,\"う髙\r\n社🍣\""
		TestCSVWithLineFeedCRLF_LF   TestCSV = "id,name,detail\r\n1,hoge,\"あ髙\n社🍣\"\r\n2,fuga,\"い髙\n社🍣\"\r\n3,piyo,\"う髙\n社🍣\""
		TestCSVWithLineFeedCRLF_CRLF TestCSV = "id,name,detail\r\n1,hoge,\"あ髙\r\n社🍣\"\r\n2,fuga,\"い髙\r\n社🍣\"\r\n3,piyo,\"う髙\r\n社🍣\""
	)
	var (
		WantCSV = [][]string{
			{"id", "name", "detail"},
			{"1", "hoge", "あ髙社🍣"},
			{"2", "fuga", "い髙社🍣"},
			{"3", "piyo", "う髙社🍣"},
		}
		WantNoHeaderCSV = [][]string{
			{"1", "hoge", "あ髙社🍣"},
			{"2", "fuga", "い髙社🍣"},
			{"3", "piyo", "う髙社🍣"},
		}
		WantCSVWithLineFeedLF = [][]string{
			{"id", "name", "detail"},
			{"1", "hoge", "あ髙\n社🍣"},
			{"2", "fuga", "い髙\n社🍣"},
			{"3", "piyo", "う髙\n社🍣"},
		}
	)

	createFixture := func(ctx context.Context, body TestCSV) awss3.Key {
		s3Client, err := awss3.GetClient(ctx, TestRegion)
		if err != nil {
			t.Fatal(err)
		}
		key := fmt.Sprintf("awstest/%s.txt", ulid.MustNew())
		uploader := manager.NewUploader(s3Client)
		input := s3.PutObjectInput{
			Body:    strings.NewReader(string(body)),
			Bucket:  aws.String(TestBucket),
			Key:     aws.String(key),
			Expires: aws.Time(time.Now().Add(10 * time.Minute)),
		}
		if _, err := uploader.Upload(ctx, &input); err != nil {
			t.Fatal(err)
		}
		waiter := s3.NewObjectExistsWaiter(s3Client)
		if err := waiter.Wait(ctx,
			&s3.HeadObjectInput{Bucket: aws.String(TestBucket), Key: aws.String(key)},
			time.Second,
		); err != nil {
			t.Fatal(err)
		}
		return awss3.Key(key)
	}

	t.Run("CSV With Header", func(t *testing.T) {
		ctx := ctxawslocal.WithContext(
			context.Background(),
			ctxawslocal.WithS3Endpoint("http://127.0.0.1:29000"), // use Minio
			ctxawslocal.WithAccessKey("DUMMYACCESSKEYEXAMPLE"),
			ctxawslocal.WithSecretAccessKey("DUMMYSECRETKEYEXAMPLE"),
		)
		src := TestCSVHeader
		key := createFixture(ctx, src)
		var buf bytes.Buffer
		err := awss3.SelectCSVAll(ctx, TestRegion, TestBucket, key, awss3.SelectCSVAllQuery, &buf)
		if !assert.NoError(t, err) {
			return
		}
		r := csv.NewReader(&buf)
		records, err := r.ReadAll()
		assert.NoError(t, err)
		assert.Equal(t, WantCSV, records)
	})
	t.Run("CSV With Header", func(t *testing.T) {
		ctx := ctxawslocal.WithContext(
			context.Background(),
			ctxawslocal.WithS3Endpoint("http://127.0.0.1:29000"), // use Minio
			ctxawslocal.WithAccessKey("DUMMYACCESSKEYEXAMPLE"),
			ctxawslocal.WithSecretAccessKey("DUMMYSECRETKEYEXAMPLE"),
		)
		src := TestCSVHeader
		key := createFixture(ctx, src)
		var buf bytes.Buffer
		err := awss3.SelectCSVAll(ctx, TestRegion, TestBucket, key, awss3.SelectCSVAllQuery, &buf)
		if !assert.NoError(t, err) {
			return
		}
		r := csv.NewReader(&buf)
		records, err := r.ReadAll()
		assert.NoError(t, err)
		assert.Equal(t, WantCSV, records)
	})
	t.Run("CSV With LineFeed File:LF, Field:LF", func(t *testing.T) {
		ctx := ctxawslocal.WithContext(
			context.Background(),
			ctxawslocal.WithS3Endpoint("http://127.0.0.1:29000"), // use Minio
			ctxawslocal.WithAccessKey("DUMMYACCESSKEYEXAMPLE"),
			ctxawslocal.WithSecretAccessKey("DUMMYSECRETKEYEXAMPLE"),
		)
		src := TestCSVWithLineFeedLF_LF
		key := createFixture(ctx, src)
		var buf bytes.Buffer
		err := awss3.SelectCSVAll(ctx, TestRegion, TestBucket, key, awss3.SelectCSVAllQuery, &buf,
			s3selectcsv.WithCSVInput(types.CSVInput{AllowQuotedRecordDelimiter: true}),
		)
		if !assert.NoError(t, err) {
			return
		}
		r := csv.NewReader(&buf)
		records, err := r.ReadAll()
		assert.NoError(t, err)
		assert.Equal(t, WantCSVWithLineFeedLF, records)
	})
	t.Run("CSV With LineFeed File:CRLF, Field:LF", func(t *testing.T) {
		ctx := ctxawslocal.WithContext(
			context.Background(),
			ctxawslocal.WithS3Endpoint("http://127.0.0.1:29000"), // use Minio
			ctxawslocal.WithAccessKey("DUMMYACCESSKEYEXAMPLE"),
			ctxawslocal.WithSecretAccessKey("DUMMYSECRETKEYEXAMPLE"),
		)
		src := TestCSVWithLineFeedCRLF_LF
		key := createFixture(ctx, src)
		var buf bytes.Buffer
		err := awss3.SelectCSVAll(ctx, TestRegion, TestBucket, key, awss3.SelectCSVAllQuery, &buf,
			s3selectcsv.WithCSVInput(types.CSVInput{AllowQuotedRecordDelimiter: true}),
		)
		if !assert.NoError(t, err) {
			return
		}
		r := csv.NewReader(&buf)
		records, err := r.ReadAll()
		assert.NoError(t, err)
		assert.Equal(t, WantCSVWithLineFeedLF, records)
	})
	t.Run("CSV With LineFeed File:LF, Field:CRLF", func(t *testing.T) {
		ctx := ctxawslocal.WithContext(
			context.Background(),
			ctxawslocal.WithS3Endpoint("http://127.0.0.1:29000"), // use Minio
			ctxawslocal.WithAccessKey("DUMMYACCESSKEYEXAMPLE"),
			ctxawslocal.WithSecretAccessKey("DUMMYSECRETKEYEXAMPLE"),
		)
		src := TestCSVWithLineFeedLF_CRLF
		key := createFixture(ctx, src)
		var buf bytes.Buffer
		err := awss3.SelectCSVAll(ctx, TestRegion, TestBucket, key, awss3.SelectCSVAllQuery, &buf,
			s3selectcsv.WithCSVInput(types.CSVInput{AllowQuotedRecordDelimiter: true}),
		)
		if !assert.NoError(t, err) {
			return
		}
		r := csv.NewReader(&buf)
		records, err := r.ReadAll()
		assert.NoError(t, err)
		assert.Equal(t, WantCSVWithLineFeedLF, records)
	})
	t.Run("CSV With LineFeed File:CRLF, Field:CRLF", func(t *testing.T) {
		ctx := ctxawslocal.WithContext(
			context.Background(),
			ctxawslocal.WithS3Endpoint("http://127.0.0.1:29000"), // use Minio
			ctxawslocal.WithAccessKey("DUMMYACCESSKEYEXAMPLE"),
			ctxawslocal.WithSecretAccessKey("DUMMYSECRETKEYEXAMPLE"),
		)
		src := TestCSVWithLineFeedCRLF_CRLF
		key := createFixture(ctx, src)
		var buf bytes.Buffer
		err := awss3.SelectCSVAll(ctx, TestRegion, TestBucket, key, awss3.SelectCSVAllQuery, &buf,
			s3selectcsv.WithCSVInput(types.CSVInput{AllowQuotedRecordDelimiter: true}),
		)

		if !assert.NoError(t, err) {
			return
		}
		r := csv.NewReader(&buf)
		records, err := r.ReadAll()
		assert.NoError(t, err)
		assert.Equal(t, WantCSVWithLineFeedLF, records)
	})
	t.Run("CSV No Header", func(t *testing.T) {
		ctx := ctxawslocal.WithContext(
			context.Background(),
			ctxawslocal.WithS3Endpoint("http://127.0.0.1:29000"), // use Minio
			ctxawslocal.WithAccessKey("DUMMYACCESSKEYEXAMPLE"),
			ctxawslocal.WithSecretAccessKey("DUMMYSECRETKEYEXAMPLE"),
		)
		src := TestCSVNoHeader
		key := createFixture(ctx, src)
		var buf bytes.Buffer
		err := awss3.SelectCSVAll(ctx, TestRegion, TestBucket, key, awss3.SelectCSVAllQuery, &buf,
			s3selectcsv.WithCSVInput(types.CSVInput{FileHeaderInfo: types.FileHeaderInfoNone}),
		)
		if !assert.NoError(t, err) {
			return
		}
		r := csv.NewReader(&buf)
		records, err := r.ReadAll()
		assert.NoError(t, err)
		assert.Equal(t, WantNoHeaderCSV, records)
	})
	t.Run("CSV With UTF-8 BOM", func(t *testing.T) {
		ctx := ctxawslocal.WithContext(
			context.Background(),
			ctxawslocal.WithS3Endpoint("http://127.0.0.1:29000"), // use Minio
			ctxawslocal.WithAccessKey("DUMMYACCESSKEYEXAMPLE"),
			ctxawslocal.WithSecretAccessKey("DUMMYSECRETKEYEXAMPLE"),
		)
		src := TestCSV(utf8bom.AddBOM([]byte(TestCSVHeader)))
		key := createFixture(ctx, src)
		var buf bytes.Buffer
		err := awss3.SelectCSVAll(ctx, TestRegion, TestBucket, key, awss3.SelectCSVAllQuery, &buf)
		if !assert.NoError(t, err) {
			return
		}
		r := csv.NewReader(&buf)
		records, err := r.ReadAll()
		assert.NoError(t, err)
		assert.Equal(t, WantCSV, records)
	})
	t.Run("CSV 300000 records", func(t *testing.T) {
		ctx := ctxawslocal.WithContext(
			context.Background(),
			ctxawslocal.WithS3Endpoint("http://127.0.0.1:29000"), // use Minio
			ctxawslocal.WithAccessKey("DUMMYACCESSKEYEXAMPLE"),
			ctxawslocal.WithSecretAccessKey("DUMMYSECRETKEYEXAMPLE"),
		)
		src := TestCSV(strings.Repeat(string(TestCSVNoHeader), 100000))
		key := createFixture(ctx, src)

		var buf bytes.Buffer
		err := awss3.SelectCSVAll(ctx, TestRegion, TestBucket, key, awss3.SelectCSVAllQuery, &buf,
			s3selectcsv.WithCSVInput(types.CSVInput{FileHeaderInfo: types.FileHeaderInfoNone}),
		)
		if !assert.NoError(t, err) {
			return
		}
		r := csv.NewReader(&buf)
		records, err := r.ReadAll()
		assert.NoError(t, err)
		assert.Equal(t, 300000, len(records))
	})
}

func TestSelectCSVHeaders(t *testing.T) {
	type TestCSV string
	const (
		TestCSVHeader TestCSV = `id,name,detail
1,hoge,あ髙社🍣
2,fuga,い髙社🍣
3,piyo,う髙社🍣
`
	)
	var (
		WantCSVHeaders = []string{"id", "name", "detail"}
	)

	createFixture := func(ctx context.Context, body TestCSV) awss3.Key {
		s3Client, err := awss3.GetClient(ctx, TestRegion)
		if err != nil {
			t.Fatal(err)
		}
		key := fmt.Sprintf("awstest/%s.txt", ulid.MustNew())
		uploader := manager.NewUploader(s3Client)
		input := s3.PutObjectInput{
			Body:    strings.NewReader(string(body)),
			Bucket:  aws.String(TestBucket),
			Key:     aws.String(key),
			Expires: aws.Time(time.Now().Add(10 * time.Minute)),
		}
		if _, err := uploader.Upload(ctx, &input); err != nil {
			t.Fatal(err)
		}
		waiter := s3.NewObjectExistsWaiter(s3Client)
		if err := waiter.Wait(ctx,
			&s3.HeadObjectInput{Bucket: aws.String(TestBucket), Key: aws.String(key)},
			time.Second,
		); err != nil {
			t.Fatal(err)
		}
		return awss3.Key(key)
	}

	t.Run("CSV With Header", func(t *testing.T) {
		ctx := ctxawslocal.WithContext(
			context.Background(),
			ctxawslocal.WithS3Endpoint("http://127.0.0.1:29000"), // use Minio
			ctxawslocal.WithAccessKey("DUMMYACCESSKEYEXAMPLE"),
			ctxawslocal.WithSecretAccessKey("DUMMYSECRETKEYEXAMPLE"),
		)
		src := TestCSVHeader
		key := createFixture(ctx, src)
		got, err := awss3.SelectCSVHeaders(ctx, TestRegion, TestBucket, key)
		if !assert.NoError(t, err) {
			return
		}
		assert.Equal(t, WantCSVHeaders, got)
	})
	t.Run("Empty CSV", func(t *testing.T) {
		ctx := ctxawslocal.WithContext(
			context.Background(),
			ctxawslocal.WithS3Endpoint("http://127.0.0.1:29000"), // use Minio
			ctxawslocal.WithAccessKey("DUMMYACCESSKEYEXAMPLE"),
			ctxawslocal.WithSecretAccessKey("DUMMYSECRETKEYEXAMPLE"),
		)
		key := createFixture(ctx, "")
		_, err := awss3.SelectCSVHeaders(ctx, TestRegion, TestBucket, key)
		assert.Error(t, err)
	})
}

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
		assert.Equal(t, int64(100), res.ContentLength)
	})
}
