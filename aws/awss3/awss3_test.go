package awss3_test

import (
	"bytes"
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/s3/transfermanager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/go-faker/faker/v4"
	"gotest.tools/v3/assert"

	"github.com/88labs/go-utils/ulid"
	"github.com/88labs/go-utils/utf8bom"

	"github.com/88labs/go-utils/aws/awsconfig"
	"github.com/88labs/go-utils/aws/awss3"
	"github.com/88labs/go-utils/aws/awss3/options/s3download"
	"github.com/88labs/go-utils/aws/awss3/options/s3head"
	"github.com/88labs/go-utils/aws/awss3/options/s3list"
	"github.com/88labs/go-utils/aws/awss3/options/s3presigned"
	"github.com/88labs/go-utils/aws/awss3/options/s3selectcsv"
	"github.com/88labs/go-utils/aws/ctxawslocal"
)

const (
	NonExistentBucket = "non-existent-bucket"
	TestBucket        = "test"
	TestRegion        = awsconfig.RegionTokyo
)

func TestHeadObject(t *testing.T) {
	t.Parallel()
	ctx := ctxawslocal.WithContext(
		context.Background(),
		ctxawslocal.WithS3Endpoint("http://127.0.0.1:29000"), // use Minio
		ctxawslocal.WithAccessKey("DUMMYACCESSKEYEXAMPLE"),
		ctxawslocal.WithSecretAccessKey("DUMMYSECRETKEYEXAMPLE"),
	)
	s3Client, err := awss3.GetClient(ctx, TestRegion)
	assert.NilError(t, err)

	createFixture := func(fileSize int) awss3.Key {
		key := fmt.Sprintf("awstest/%s.txt", ulid.MustNew())
		uploader := transfermanager.New(s3Client)
		input := &transfermanager.UploadObjectInput{
			Body:    bytes.NewReader(bytes.Repeat([]byte{1}, fileSize)),
			Bucket:  aws.String(TestBucket),
			Key:     aws.String(key),
			Expires: aws.Time(time.Now().Add(10 * time.Minute)),
		}
		if _, err := uploader.UploadObject(ctx, input); err != nil {
			assert.NilError(t, err)
		}
		return awss3.Key(key)
	}

	t.Run("exists object", func(t *testing.T) {
		t.Parallel()
		key := createFixture(100)
		res, err := awss3.HeadObject(ctx, TestRegion, TestBucket, key)
		assert.NilError(t, err)
		assert.DeepEqual(t, aws.Int64(100), res.ContentLength)
	})
	t.Run("not exists object", func(t *testing.T) {
		t.Parallel()
		_, err := awss3.HeadObject(ctx, TestRegion, TestBucket, "NOT_FOUND")
		assert.Assert(t, err != nil)
		assert.ErrorIs(t, err, awss3.ErrNotFound)
	})
	t.Run("exists object use Waiter", func(t *testing.T) {
		t.Parallel()
		key := createFixture(100)
		res, err := awss3.HeadObject(ctx, TestRegion, TestBucket, key,
			s3head.WithTimeout(5*time.Second),
		)
		assert.NilError(t, err)
		assert.DeepEqual(t, aws.Int64(100), res.ContentLength)
	})
	t.Run("not exists object use Waiter", func(t *testing.T) {
		t.Parallel()
		_, err := awss3.HeadObject(ctx, TestRegion, TestBucket, "NOT_FOUND",
			s3head.WithTimeout(5*time.Second),
		)
		assert.Assert(t, err != nil)
		assert.ErrorIs(t, errors.Unwrap(err), awss3.ErrNotFound)
	})
}

func TestListObjects(t *testing.T) {
	t.Parallel()
	ctx := ctxawslocal.WithContext(
		context.Background(),
		ctxawslocal.WithS3Endpoint("http://127.0.0.1:29000"), // use Minio
		ctxawslocal.WithAccessKey("DUMMYACCESSKEYEXAMPLE"),
		ctxawslocal.WithSecretAccessKey("DUMMYSECRETKEYEXAMPLE"),
	)
	s3Client, err := awss3.GetClient(ctx, TestRegion)
	assert.NilError(t, err)

	createFixture := func(prefix string) awss3.Key {
		key := fmt.Sprintf("%s/awstest/%s.txt", prefix, ulid.MustNew())
		uploader := transfermanager.New(s3Client)
		input := &transfermanager.UploadObjectInput{
			Body:    strings.NewReader("test"),
			Bucket:  aws.String(TestBucket),
			Key:     aws.String(key),
			Expires: aws.Time(time.Now().Add(10 * time.Minute)),
		}
		if _, err := uploader.UploadObject(ctx, input); err != nil {
			assert.NilError(t, err)
		}
		return awss3.Key(key)
	}

	t.Run("ListObjects", func(t *testing.T) {
		t.Parallel()
		key1 := createFixture("hoge")
		key2 := createFixture("hoge")
		key3 := createFixture("hoge")
		res, err := awss3.ListObjects(ctx, TestRegion, TestBucket)
		assert.NilError(t, err)
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
		t.Parallel()
		key1 := createFixture("hoge")
		key2 := createFixture("hoge")
		key3 := createFixture("fuga")
		res, err := awss3.ListObjects(ctx, TestRegion, TestBucket, s3list.WithPrefix("hoge"))
		assert.NilError(t, err)
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
		t.Parallel()
		keys := make([]awss3.Key, 1001)
		for i := 0; i < 1001; i++ {
			keys[i] = createFixture("piyo")
		}
		res, err := awss3.ListObjects(ctx, TestRegion, TestBucket)
		assert.NilError(t, err)
		for _, key := range keys {
			if _, ok := res.Find(key); !ok {
				t.Errorf("%s not found", key)
			}
		}
	})
}

func TestGetObject(t *testing.T) {
	t.Parallel()
	ctx := ctxawslocal.WithContext(
		context.Background(),
		ctxawslocal.WithS3Endpoint("http://127.0.0.1:29000"), // use Minio
		ctxawslocal.WithAccessKey("DUMMYACCESSKEYEXAMPLE"),
		ctxawslocal.WithSecretAccessKey("DUMMYSECRETKEYEXAMPLE"),
	)
	s3Client, err := awss3.GetClient(ctx, TestRegion)
	assert.NilError(t, err)

	createFixture := func() awss3.Key {
		key := fmt.Sprintf("awstest/%s.txt", ulid.MustNew())
		uploader := transfermanager.New(s3Client)
		input := &transfermanager.UploadObjectInput{
			Body:    strings.NewReader("test"),
			Bucket:  aws.String(TestBucket),
			Key:     aws.String(key),
			Expires: aws.Time(time.Now().Add(10 * time.Minute)),
		}
		if _, err := uploader.UploadObject(ctx, input); err != nil {
			assert.NilError(t, err)
		}
		return awss3.Key(key)
	}

	t.Run("GetObjectWriter", func(t *testing.T) {
		t.Parallel()
		key := createFixture()
		var buf bytes.Buffer
		err := awss3.GetObjectWriter(ctx, TestRegion, TestBucket, key, &buf)
		assert.NilError(t, err)
		assert.Equal(t, "test", buf.String())
	})

	t.Run("GetObjectWriter NotFound", func(t *testing.T) {
		t.Parallel()
		var buf bytes.Buffer
		err := awss3.GetObjectWriter(ctx, TestRegion, TestBucket, "NOT_FOUND", &buf)
		assert.Assert(t, err != nil)
		assert.ErrorIs(t, err, awss3.ErrNotFound)
	})
}

func TestDeleteObject(t *testing.T) {
	t.Parallel()
	ctx := ctxawslocal.WithContext(
		context.Background(),
		ctxawslocal.WithS3Endpoint("http://127.0.0.1:29000"), // use Minio
		ctxawslocal.WithAccessKey("DUMMYACCESSKEYEXAMPLE"),
		ctxawslocal.WithSecretAccessKey("DUMMYSECRETKEYEXAMPLE"),
	)
	s3Client, err := awss3.GetClient(ctx, TestRegion)
	assert.NilError(t, err)

	createFixture := func() awss3.Key {
		key := fmt.Sprintf("awstest/%s.txt", ulid.MustNew())
		uploader := transfermanager.New(s3Client)
		input := &transfermanager.UploadObjectInput{
			Body:    strings.NewReader("test"),
			Bucket:  aws.String(TestBucket),
			Key:     aws.String(key),
			Expires: aws.Time(time.Now().Add(10 * time.Minute)),
		}
		if _, err := uploader.UploadObject(ctx, input); err != nil {
			assert.NilError(t, err)
		}
		return awss3.Key(key)
	}

	t.Run("DeleteObject", func(t *testing.T) {
		t.Parallel()
		key := createFixture()
		_, err := awss3.DeleteObject(ctx, TestRegion, TestBucket, key)
		assert.NilError(t, err)
		_, err = awss3.HeadObject(ctx, TestRegion, TestBucket, key)
		assert.Assert(t, err != nil)
	})

	t.Run("DeleteObject NotFound", func(t *testing.T) {
		t.Parallel()
		_, err := awss3.DeleteObject(ctx, TestRegion, TestBucket, "NOT_FOUND")
		assert.Assert(t, err != nil)
		assert.ErrorIs(t, err, awss3.ErrNotFound)
	})
}

func TestDownloadFiles(t *testing.T) {
	t.Parallel()
	ctx := ctxawslocal.WithContext(
		context.Background(),
		ctxawslocal.WithS3Endpoint("http://127.0.0.1:29000"), // use Minio
		ctxawslocal.WithAccessKey("DUMMYACCESSKEYEXAMPLE"),
		ctxawslocal.WithSecretAccessKey("DUMMYSECRETKEYEXAMPLE"),
	)
	s3Client, err := awss3.GetClient(ctx, TestRegion)
	assert.NilError(t, err)

	getBodyText := func(idx int) string {
		bodyText := fmt.Sprintf("%d-%s", idx, strings.Repeat("test", 10000))
		return fmt.Sprintf(bodyText, idx)
	}
	keys := make(awss3.Keys, 100)
	for i := 0; i < 100; i++ {
		key := fmt.Sprintf("awstest/%s.txt", ulid.MustNew())
		uploader := transfermanager.New(s3Client)
		input := &transfermanager.UploadObjectInput{
			Body:    strings.NewReader(getBodyText(i)),
			Bucket:  aws.String(TestBucket),
			Key:     aws.String(key),
			Expires: aws.Time(time.Now().Add(10 * time.Minute)),
		}
		if _, err := uploader.UploadObject(ctx, input); err != nil {
			assert.NilError(t, err)
			return
		}
		keys[i] = awss3.Key(key)
	}

	t.Run("no option", func(t *testing.T) {
		t.Parallel()
		filePaths, err := awss3.DownloadFiles(ctx, TestRegion, TestBucket, keys, t.TempDir())
		assert.NilError(t, err)
		assert.Equal(t, len(keys), len(filePaths))
		for i, v := range filePaths {
			assert.Equal(t, filepath.Base(keys[i].String()), filepath.Base(v))
			fileBody, err := os.ReadFile(v)
			assert.NilError(t, err)
			assert.Equal(t, getBodyText(i), string(fileBody))
		}
	})
	t.Run("FileNameReplacer:not duplicate", func(t *testing.T) {
		t.Parallel()
		filePaths, err := awss3.DownloadFiles(ctx, TestRegion, TestBucket, keys, t.TempDir(),
			s3download.WithFileNameReplacerFunc(func(S3Key, baseFileName string) string {
				return "add_" + baseFileName
			}),
		)
		assert.NilError(t, err)
		assert.Equal(t, len(keys), len(filePaths))
		for i, v := range filePaths {
			assert.Equal(t, "add_"+filepath.Base(keys[i].String()), filepath.Base(v))
			fileBody, err := os.ReadFile(v)
			assert.NilError(t, err)
			assert.Equal(t, getBodyText(i), string(fileBody))
		}
	})
	t.Run("FileNameReplacer:duplicate", func(t *testing.T) {
		t.Parallel()
		filePaths, err := awss3.DownloadFiles(ctx, TestRegion, TestBucket, keys, t.TempDir(),
			s3download.WithFileNameReplacerFunc(func(S3Key, baseFileName string) string {
				return "fixname.txt"
			}),
		)
		assert.NilError(t, err)
		assert.Equal(t, len(keys), len(filePaths))
		for i, v := range filePaths {
			if i == 0 {
				assert.Equal(t, "fixname.txt", filepath.Base(v))
			} else {
				assert.Equal(t, fmt.Sprintf("fixname_%d.txt", i+1), filepath.Base(v))
			}
			fileBody, err := os.ReadFile(v)
			assert.NilError(t, err)
			assert.Equal(t, getBodyText(i), string(fileBody))
		}
	})
	t.Run("Error:BucketNotFound", func(t *testing.T) {
		t.Parallel()
		_, err := awss3.DownloadFiles(ctx, TestRegion, "NOT_FOUND", keys, t.TempDir())
		assert.Assert(t, err != nil)
	})
	t.Run("Error:KeyNotFound", func(t *testing.T) {
		t.Parallel()
		dummyKeys := awss3.Keys{"dummy", "dummy", "dummy"}
		_, err := awss3.DownloadFiles(ctx, TestRegion, TestBucket, dummyKeys, t.TempDir())
		assert.Assert(t, err != nil)
	})
}

func TestDownloadFilesParallel(t *testing.T) {
	t.Parallel()
	ctx := ctxawslocal.WithContext(
		context.Background(),
		ctxawslocal.WithS3Endpoint("http://127.0.0.1:29000"), // use Minio
		ctxawslocal.WithAccessKey("DUMMYACCESSKEYEXAMPLE"),
		ctxawslocal.WithSecretAccessKey("DUMMYSECRETKEYEXAMPLE"),
	)
	s3Client, err := awss3.GetClient(ctx, TestRegion)
	assert.NilError(t, err)

	getBodyText := func(idx int) string {
		bodyText := fmt.Sprintf("%d-%s", idx, strings.Repeat("test", 10000))
		return fmt.Sprintf(bodyText, idx)
	}
	keys := make(awss3.Keys, 100)
	for i := 0; i < 100; i++ {
		key := fmt.Sprintf("awstest/%s.txt", ulid.MustNew())
		uploader := transfermanager.New(s3Client)
		input := &transfermanager.UploadObjectInput{
			Body:    strings.NewReader(getBodyText(i)),
			Bucket:  aws.String(TestBucket),
			Key:     aws.String(key),
			Expires: aws.Time(time.Now().Add(10 * time.Minute)),
		}
		if _, err := uploader.UploadObject(ctx, input); err != nil {
			assert.NilError(t, err)
			return
		}
		keys[i] = awss3.Key(key)
	}

	t.Run("no option", func(t *testing.T) {
		t.Parallel()
		filePaths, err := awss3.DownloadFilesParallel(ctx, TestRegion, TestBucket, keys, t.TempDir())
		assert.NilError(t, err)
		assert.Equal(t, len(keys), len(filePaths))
		for i, v := range filePaths {
			assert.Equal(t, filepath.Base(keys[i].String()), filepath.Base(v))
			fileBody, err := os.ReadFile(v)
			assert.NilError(t, err)
			assert.Equal(t, getBodyText(i), string(fileBody))
		}
	})
	t.Run("FileNameReplacer:not duplicate", func(t *testing.T) {
		t.Parallel()
		filePaths, err := awss3.DownloadFilesParallel(ctx, TestRegion, TestBucket, keys, t.TempDir(),
			s3download.WithFileNameReplacerFunc(func(S3Key, baseFileName string) string {
				return "add_" + baseFileName
			}),
		)
		assert.NilError(t, err)
		assert.Equal(t, len(keys), len(filePaths))
		for i, v := range filePaths {
			assert.Equal(t, "add_"+filepath.Base(keys[i].String()), filepath.Base(v))
			fileBody, err := os.ReadFile(v)
			assert.NilError(t, err)
			assert.Equal(t, getBodyText(i), string(fileBody))
		}
	})
	t.Run("FileNameReplacer:duplicate", func(t *testing.T) {
		t.Parallel()
		filePaths, err := awss3.DownloadFilesParallel(ctx, TestRegion, TestBucket, keys, t.TempDir(),
			s3download.WithFileNameReplacerFunc(func(S3Key, baseFileName string) string {
				return "fixname.txt"
			}),
		)
		assert.NilError(t, err)
		assert.Equal(t, len(keys), len(filePaths))
		for i, v := range filePaths {
			if i == 0 {
				assert.Equal(t, "fixname.txt", filepath.Base(v))
			} else {
				assert.Equal(t, fmt.Sprintf("fixname_%d.txt", i+1), filepath.Base(v))
			}
			fileBody, err := os.ReadFile(v)
			assert.NilError(t, err)
			assert.Equal(t, getBodyText(i), string(fileBody))
		}
	})
	t.Run("Error:BucketNotFound", func(t *testing.T) {
		t.Parallel()
		_, err := awss3.DownloadFilesParallel(ctx, TestRegion, "NOT_FOUND", keys, t.TempDir())
		assert.Assert(t, err != nil)
	})
	t.Run("Error:KeyNotFound", func(t *testing.T) {
		t.Parallel()
		dummyKeys := awss3.Keys{"dummy", "dummy", "dummy"}
		_, err := awss3.DownloadFilesParallel(ctx, TestRegion, TestBucket, dummyKeys, t.TempDir())
		assert.Assert(t, err != nil)
	})
}

func TestPutObject(t *testing.T) {
	t.Parallel()
	ctx := ctxawslocal.WithContext(
		context.Background(),
		ctxawslocal.WithS3Endpoint("http://127.0.0.1:29000"), // use Minio
		ctxawslocal.WithAccessKey("DUMMYACCESSKEYEXAMPLE"),
		ctxawslocal.WithSecretAccessKey("DUMMYSECRETKEYEXAMPLE"),
	)

	t.Run("PutObject", func(t *testing.T) {
		t.Parallel()
		key := fmt.Sprintf("awstest/%s.txt", ulid.MustNew())
		body := faker.Sentence()
		_, err := awss3.PutObject(ctx, TestRegion, TestBucket, awss3.Key(key), strings.NewReader(body))
		assert.NilError(t, err)
		filePaths, err := awss3.DownloadFiles(ctx, TestRegion, TestBucket, awss3.NewKeys(key), t.TempDir())
		assert.NilError(t, err)
		assert.Equal(t, 1, len(filePaths))
		fileBody, err := os.ReadFile(filePaths[0])
		assert.NilError(t, err)
		assert.Equal(t, body, string(fileBody))
	})
}

func TestUploadManager(t *testing.T) {
	t.Parallel()
	ctx := ctxawslocal.WithContext(
		context.Background(),
		ctxawslocal.WithS3Endpoint("http://127.0.0.1:29000"), // use Minio
		ctxawslocal.WithAccessKey("DUMMYACCESSKEYEXAMPLE"),
		ctxawslocal.WithSecretAccessKey("DUMMYSECRETKEYEXAMPLE"),
	)

	t.Run("UploadManager", func(t *testing.T) {
		t.Parallel()
		key := fmt.Sprintf("awstest/%s.txt", ulid.MustNew())
		body := faker.Sentence()
		_, err := awss3.UploadManager(ctx, TestRegion, TestBucket, awss3.Key(key), strings.NewReader(body))
		assert.NilError(t, err)
		filePaths, err := awss3.DownloadFiles(ctx, TestRegion, TestBucket, awss3.NewKeys(key), t.TempDir())
		assert.NilError(t, err)
		assert.Equal(t, 1, len(filePaths))
		fileBody, err := os.ReadFile(filePaths[0])
		assert.NilError(t, err)
		assert.Equal(t, body, string(fileBody))
	})
}

func TestPresign(t *testing.T) {
	t.Parallel()
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
		uploader := transfermanager.New(s3Client)
		input := &transfermanager.UploadObjectInput{
			Body:    strings.NewReader("test"),
			Bucket:  aws.String(TestBucket),
			Key:     aws.String(key),
			Expires: aws.Time(time.Now().Add(10 * time.Minute)),
		}
		if _, err := uploader.UploadObject(ctx, input); err != nil {
			assert.NilError(t, err)
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
		uploader := transfermanager.New(s3Client)
		input := &transfermanager.UploadObjectInput{
			Body:    strings.NewReader("test"),
			Bucket:  aws.String(TestBucket),
			Key:     aws.String(key),
			Expires: aws.Time(time.Now().Add(10 * time.Minute)),
		}
		if _, err := uploader.UploadObject(ctx, input); err != nil {
			assert.NilError(t, err)
			return ""
		}
		return awss3.Key(key)
	}

	t.Run("Presign", func(t *testing.T) {
		t.Parallel()
		key := uploadText()
		presign, err := awss3.Presign(ctx, TestRegion, TestBucket, key)
		assert.NilError(t, err)
		assert.Assert(t, presign != "")
	})
	t.Run("Presign PDF", func(t *testing.T) {
		t.Parallel()
		key := uploadPDF()
		presign, err := awss3.Presign(ctx, TestRegion, TestBucket, key)
		assert.NilError(t, err)
		assert.Assert(t, presign != "")
	})
	t.Run("Presign NotFound", func(t *testing.T) {
		t.Parallel()
		_, err := awss3.Presign(ctx, TestRegion, TestBucket, "NOT_FOUND")
		assert.Assert(t, err != nil)
		assert.ErrorIs(t, err, awss3.ErrNotFound)
	})
}

func TestResponseContentDisposition(t *testing.T) {
	t.Parallel()
	const fileName = ",あいうえお　牡蠣喰家来 サシスセソ@+$_-^|+{}"
	t.Run("success", func(t *testing.T) {
		t.Parallel()
		actual := awss3.ResponseContentDisposition(s3presigned.ContentDispositionTypeAttachment, fileName)
		assert.Assert(t, actual != "")
	})
}

func TestCopy(t *testing.T) {
	t.Parallel()
	createFixture := func(ctx context.Context) awss3.Key {
		s3Client, err := awss3.GetClient(ctx, TestRegion)
		if err != nil {
			t.Fatal(err)
		}
		key := fmt.Sprintf("awstest/%s.txt", ulid.MustNew())
		uploader := transfermanager.New(s3Client)
		input := &transfermanager.UploadObjectInput{
			Body:    strings.NewReader("test"),
			Bucket:  aws.String(TestBucket),
			Key:     aws.String(key),
			Expires: aws.Time(time.Now().Add(10 * time.Minute)),
		}
		if _, err := uploader.UploadObject(ctx, input); err != nil {
			assert.NilError(t, err)
			t.FailNow()
		}
		waiter := s3.NewObjectExistsWaiter(s3Client)
		if err := waiter.Wait(ctx,
			&s3.HeadObjectInput{Bucket: aws.String(TestBucket), Key: aws.String(key)},
			time.Second,
		); err != nil {
			assert.NilError(t, err)
			t.FailNow()
		}
		return awss3.Key(key)
	}

	t.Run("Copy:Same Bucket and Other Key", func(t *testing.T) {
		t.Parallel()
		ctx := ctxawslocal.WithContext(
			context.Background(),
			ctxawslocal.WithS3Endpoint("http://127.0.0.1:29000"), // use Minio
			ctxawslocal.WithAccessKey("DUMMYACCESSKEYEXAMPLE"),
			ctxawslocal.WithSecretAccessKey("DUMMYSECRETKEYEXAMPLE"),
		)
		key := createFixture(ctx)
		key2 := awss3.Key(fmt.Sprintf("awstest/%s.txt", ulid.MustNew()))
		assert.NilError(t, awss3.Copy(ctx, TestRegion, TestBucket, key, key2))
	})
	t.Run("Copy:Same Item", func(t *testing.T) {
		t.Parallel()
		ctx := ctxawslocal.WithContext(
			context.Background(),
			ctxawslocal.WithS3Endpoint("http://127.0.0.1:29000"), // use Minio
			ctxawslocal.WithAccessKey("DUMMYACCESSKEYEXAMPLE"),
			ctxawslocal.WithSecretAccessKey("DUMMYSECRETKEYEXAMPLE"),
		)
		key := createFixture(ctx)
		assert.NilError(t, awss3.Copy(ctx, TestRegion, TestBucket, key, key))
	})

	t.Run("Copy:NotFound", func(t *testing.T) {
		t.Parallel()
		ctx := ctxawslocal.WithContext(
			context.Background(),
			ctxawslocal.WithS3Endpoint("http://127.0.0.1:29000"), // use Minio
			ctxawslocal.WithAccessKey("DUMMYACCESSKEYEXAMPLE"),
			ctxawslocal.WithSecretAccessKey("DUMMYSECRETKEYEXAMPLE"),
		)
		err := awss3.Copy(ctx, TestRegion, TestBucket, "NOT_FOUND", "NOT_FOUND")
		assert.Assert(t, err != nil)
		assert.ErrorIs(t, err, awss3.ErrNotFound)
	})
}

func TestSelectCSVAll(t *testing.T) {
	t.Parallel()
	type TestCSV string
	const (
		TestCSVHeader TestCSV = `id,name,detail
1,hoge,あ髙社🍣
2,fuga,い髙社🍣
3,piyo,う髙社🍣
`
		TestCSVNoHeader TestCSV = `1,hoge,あ髙社🍣
2,fuga,い髙社🍣
3,piyo,う髙社🍣
`
		TestCSVWithLineFeedLF_LF     TestCSV = "id,name,detail\n1,hoge,\"あ髙\n社🍣\"\n2,fuga,\"い髙\n社🍣\"\n3,piyo,\"う髙\n社🍣\""
		TestCSVWithLineFeedLF_CRLF   TestCSV = "id,name,detail\n1,hoge,\"あ髙\r\n社🍣\"\n2,fuga,\"い髙\r\n社🍣\"\n3,piyo,\"う髙\r\n社🍣\""
		TestCSVWithLineFeedCRLF_LF   TestCSV = "id,name,detail\r\n1,hoge,\"あ髙\n社🍣\"\r\n2,fuga,\"い髙\n社🍣\"\r\n3,piyo,\"う髙\n社🍣\""
		TestCSVWithLineFeedCRLF_CRLF TestCSV = "id,name,detail\r\n1,hoge,\"あ髙\r\n社🍣\"\r\n2,fuga,\"い髙\r\n社🍣\"\r\n3,piyo,\"う髙\r\n社🍣\""
	)
	var (
		WantCSV = [][]string{
			{"id", "name", "detail"},
			{"1", "hoge", "あ髙社🍣"},
			{"2", "fuga", "い髙社🍣"},
			{"3", "piyo", "う髙社🍣"},
		}
		WantNoHeaderCSV = [][]string{
			{"1", "hoge", "あ髙社🍣"},
			{"2", "fuga", "い髙社🍣"},
			{"3", "piyo", "う髙社🍣"},
		}
		WantCSVWithLineFeedLF = [][]string{
			{"id", "name", "detail"},
			{"1", "hoge", "あ髙\n社🍣"},
			{"2", "fuga", "い髙\n社🍣"},
			{"3", "piyo", "う髙\n社🍣"},
		}
	)

	createFixture := func(ctx context.Context, body TestCSV) awss3.Key {
		s3Client, err := awss3.GetClient(ctx, TestRegion)
		if err != nil {
			t.Fatal(err)
		}
		key := fmt.Sprintf("awstest/%s.txt", ulid.MustNew())
		uploader := transfermanager.New(s3Client)
		input := &transfermanager.UploadObjectInput{
			Body:    strings.NewReader(string(body)),
			Bucket:  aws.String(TestBucket),
			Key:     aws.String(key),
			Expires: aws.Time(time.Now().Add(10 * time.Minute)),
		}
		if _, err := uploader.UploadObject(ctx, input); err != nil {
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
		t.Parallel()
		ctx := ctxawslocal.WithContext(
			context.Background(),
			ctxawslocal.WithS3Endpoint("http://127.0.0.1:29000"), // use Minio
			ctxawslocal.WithAccessKey("DUMMYACCESSKEYEXAMPLE"),
			ctxawslocal.WithSecretAccessKey("DUMMYSECRETKEYEXAMPLE"),
		)
		key := createFixture(ctx, TestCSVHeader)
		var buf bytes.Buffer
		assert.NilError(t, awss3.SelectCSVAll(ctx, TestRegion, TestBucket, key, awss3.SelectCSVAllQuery, &buf))
		records, err := csv.NewReader(&buf).ReadAll()
		assert.NilError(t, err)
		assert.DeepEqual(t, WantCSV, records)
	})
	t.Run("CSV With Header", func(t *testing.T) {
		t.Parallel()
		ctx := ctxawslocal.WithContext(
			context.Background(),
			ctxawslocal.WithS3Endpoint("http://127.0.0.1:29000"), // use Minio
			ctxawslocal.WithAccessKey("DUMMYACCESSKEYEXAMPLE"),
			ctxawslocal.WithSecretAccessKey("DUMMYSECRETKEYEXAMPLE"),
		)
		src := TestCSVHeader
		key := createFixture(ctx, src)
		var buf bytes.Buffer
		assert.NilError(t, awss3.SelectCSVAll(ctx, TestRegion, TestBucket, key, awss3.SelectCSVAllQuery, &buf))
		records, err := csv.NewReader(&buf).ReadAll()
		assert.NilError(t, err)
		assert.DeepEqual(t, WantCSV, records)
	})
	t.Run("CSV With LineFeed File:LF, Field:LF", func(t *testing.T) {
		t.Parallel()
		ctx := ctxawslocal.WithContext(
			context.Background(),
			ctxawslocal.WithS3Endpoint("http://127.0.0.1:29000"), // use Minio
			ctxawslocal.WithAccessKey("DUMMYACCESSKEYEXAMPLE"),
			ctxawslocal.WithSecretAccessKey("DUMMYSECRETKEYEXAMPLE"),
		)
		src := TestCSVWithLineFeedLF_LF
		key := createFixture(ctx, src)
		var buf bytes.Buffer
		assert.NilError(t, awss3.SelectCSVAll(ctx, TestRegion, TestBucket, key, awss3.SelectCSVAllQuery, &buf,
			s3selectcsv.WithCSVInput(types.CSVInput{AllowQuotedRecordDelimiter: aws.Bool(true)}),
		))
		records, err := csv.NewReader(&buf).ReadAll()
		assert.NilError(t, err)
		assert.DeepEqual(t, WantCSVWithLineFeedLF, records)
	})
	t.Run("CSV With LineFeed File:CRLF, Field:LF", func(t *testing.T) {
		t.Parallel()
		ctx := ctxawslocal.WithContext(
			context.Background(),
			ctxawslocal.WithS3Endpoint("http://127.0.0.1:29000"), // use Minio
			ctxawslocal.WithAccessKey("DUMMYACCESSKEYEXAMPLE"),
			ctxawslocal.WithSecretAccessKey("DUMMYSECRETKEYEXAMPLE"),
		)
		src := TestCSVWithLineFeedCRLF_LF
		key := createFixture(ctx, src)
		var buf bytes.Buffer
		assert.NilError(t, awss3.SelectCSVAll(ctx, TestRegion, TestBucket, key, awss3.SelectCSVAllQuery, &buf,
			s3selectcsv.WithCSVInput(types.CSVInput{AllowQuotedRecordDelimiter: aws.Bool(true)}),
		))
		records, err := csv.NewReader(&buf).ReadAll()
		assert.NilError(t, err)
		assert.DeepEqual(t, WantCSVWithLineFeedLF, records)
	})
	t.Run("CSV With LineFeed File:LF, Field:CRLF", func(t *testing.T) {
		t.Parallel()
		ctx := ctxawslocal.WithContext(
			context.Background(),
			ctxawslocal.WithS3Endpoint("http://127.0.0.1:29000"), // use Minio
			ctxawslocal.WithAccessKey("DUMMYACCESSKEYEXAMPLE"),
			ctxawslocal.WithSecretAccessKey("DUMMYSECRETKEYEXAMPLE"),
		)
		src := TestCSVWithLineFeedLF_CRLF
		key := createFixture(ctx, src)
		var buf bytes.Buffer
		assert.NilError(t, awss3.SelectCSVAll(ctx, TestRegion, TestBucket, key, awss3.SelectCSVAllQuery, &buf,
			s3selectcsv.WithCSVInput(types.CSVInput{AllowQuotedRecordDelimiter: aws.Bool(true)}),
		))
		records, err := csv.NewReader(&buf).ReadAll()
		assert.NilError(t, err)
		assert.DeepEqual(t, WantCSVWithLineFeedLF, records)
	})
	t.Run("CSV With LineFeed File:CRLF, Field:CRLF", func(t *testing.T) {
		t.Parallel()
		ctx := ctxawslocal.WithContext(
			context.Background(),
			ctxawslocal.WithS3Endpoint("http://127.0.0.1:29000"), // use Minio
			ctxawslocal.WithAccessKey("DUMMYACCESSKEYEXAMPLE"),
			ctxawslocal.WithSecretAccessKey("DUMMYSECRETKEYEXAMPLE"),
		)
		src := TestCSVWithLineFeedCRLF_CRLF
		key := createFixture(ctx, src)
		var buf bytes.Buffer
		assert.NilError(t, awss3.SelectCSVAll(ctx, TestRegion, TestBucket, key, awss3.SelectCSVAllQuery, &buf,
			s3selectcsv.WithCSVInput(types.CSVInput{AllowQuotedRecordDelimiter: aws.Bool(true)}),
		))

		records, err := csv.NewReader(&buf).ReadAll()
		assert.NilError(t, err)
		assert.DeepEqual(t, WantCSVWithLineFeedLF, records)
	})
	t.Run("CSV No Header", func(t *testing.T) {
		t.Parallel()
		ctx := ctxawslocal.WithContext(
			context.Background(),
			ctxawslocal.WithS3Endpoint("http://127.0.0.1:29000"), // use Minio
			ctxawslocal.WithAccessKey("DUMMYACCESSKEYEXAMPLE"),
			ctxawslocal.WithSecretAccessKey("DUMMYSECRETKEYEXAMPLE"),
		)
		src := TestCSVNoHeader
		key := createFixture(ctx, src)
		var buf bytes.Buffer
		assert.NilError(t, awss3.SelectCSVAll(ctx, TestRegion, TestBucket, key, awss3.SelectCSVAllQuery, &buf,
			s3selectcsv.WithCSVInput(types.CSVInput{FileHeaderInfo: types.FileHeaderInfoNone}),
		))
		records, err := csv.NewReader(&buf).ReadAll()
		assert.NilError(t, err)
		assert.DeepEqual(t, WantNoHeaderCSV, records)
	})
	t.Run("CSV With UTF-8 BOM", func(t *testing.T) {
		t.Parallel()
		ctx := ctxawslocal.WithContext(
			context.Background(),
			ctxawslocal.WithS3Endpoint("http://127.0.0.1:29000"), // use Minio
			ctxawslocal.WithAccessKey("DUMMYACCESSKEYEXAMPLE"),
			ctxawslocal.WithSecretAccessKey("DUMMYSECRETKEYEXAMPLE"),
		)
		src := TestCSV(utf8bom.AddBOM([]byte(TestCSVHeader)))
		key := createFixture(ctx, src)
		var buf bytes.Buffer
		assert.NilError(t, awss3.SelectCSVAll(ctx, TestRegion, TestBucket, key, awss3.SelectCSVAllQuery, &buf))
		records, err := csv.NewReader(&buf).ReadAll()
		assert.NilError(t, err)
		assert.DeepEqual(t, WantCSV, records)
	})
	t.Run("CSV 300000 records", func(t *testing.T) {
		t.Parallel()
		ctx := ctxawslocal.WithContext(
			context.Background(),
			ctxawslocal.WithS3Endpoint("http://127.0.0.1:29000"), // use Minio
			ctxawslocal.WithAccessKey("DUMMYACCESSKEYEXAMPLE"),
			ctxawslocal.WithSecretAccessKey("DUMMYSECRETKEYEXAMPLE"),
		)
		src := TestCSV(strings.Repeat(string(TestCSVNoHeader), 100000))
		key := createFixture(ctx, src)

		var buf bytes.Buffer
		assert.NilError(t, awss3.SelectCSVAll(ctx, TestRegion, TestBucket, key, awss3.SelectCSVAllQuery, &buf,
			s3selectcsv.WithCSVInput(types.CSVInput{FileHeaderInfo: types.FileHeaderInfoNone}),
		))
		records, err := csv.NewReader(&buf).ReadAll()
		assert.NilError(t, err)
		assert.Equal(t, 300000, len(records))
	})
}

func TestSelectCSVHeaders(t *testing.T) {
	t.Parallel()
	type TestCSV string
	const (
		TestCSVHeader TestCSV = `id,name,detail
1,hoge,あ髙社🍣
2,fuga,い髙社🍣
3,piyo,う髙社🍣
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
		uploader := transfermanager.New(s3Client)
		input := &transfermanager.UploadObjectInput{
			Body:    strings.NewReader(string(body)),
			Bucket:  aws.String(TestBucket),
			Key:     aws.String(key),
			Expires: aws.Time(time.Now().Add(10 * time.Minute)),
		}
		if _, err := uploader.UploadObject(ctx, input); err != nil {
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
		t.Parallel()
		ctx := ctxawslocal.WithContext(
			context.Background(),
			ctxawslocal.WithS3Endpoint("http://127.0.0.1:29000"), // use Minio
			ctxawslocal.WithAccessKey("DUMMYACCESSKEYEXAMPLE"),
			ctxawslocal.WithSecretAccessKey("DUMMYSECRETKEYEXAMPLE"),
		)
		key := createFixture(ctx, TestCSVHeader)
		got, err := awss3.SelectCSVHeaders(ctx, TestRegion, TestBucket, key)
		assert.NilError(t, err)
		assert.DeepEqual(t, WantCSVHeaders, got)
	})
	t.Run("Empty CSV", func(t *testing.T) {
		t.Parallel()
		ctx := ctxawslocal.WithContext(
			context.Background(),
			ctxawslocal.WithS3Endpoint("http://127.0.0.1:29000"), // use Minio
			ctxawslocal.WithAccessKey("DUMMYACCESSKEYEXAMPLE"),
			ctxawslocal.WithSecretAccessKey("DUMMYSECRETKEYEXAMPLE"),
		)
		key := createFixture(ctx, "")
		_, err := awss3.SelectCSVHeaders(ctx, TestRegion, TestBucket, key)
		assert.Assert(t, err != nil)
	})
}

func TestPresignPutObject(t *testing.T) {
	t.Parallel()
	ctx := ctxawslocal.WithContext(
		context.Background(),
		ctxawslocal.WithS3Endpoint("http://127.0.0.1:29000"), // use Minio
		ctxawslocal.WithAccessKey("DUMMYACCESSKEYEXAMPLE"),
		ctxawslocal.WithSecretAccessKey("DUMMYSECRETKEYEXAMPLE"),
	)

	uploadTxtByPresignedPutObjectURL := func(presignedURL string) error {
		content := []byte("Hello World")
		req, err := http.NewRequest(http.MethodPut, presignedURL, bytes.NewReader(content))
		if err != nil {
			return err
		}

		req.Header.Set("Content-Type", "text/plain")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return err
		}

		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("failed to upload file, status code: %d", resp.StatusCode)
		}

		return nil
	}

	confirmedUploadedObject := func(ctx context.Context, key awss3.Key) error {
		_, err := awss3.HeadObject(ctx, TestRegion, TestBucket, key)
		return err
	}

	t.Run("Presign", func(t *testing.T) {
		t.Parallel()
		key := awss3.Key("test_presign_put_object_01.txt")
		pURL, err := awss3.PresignPutObject(ctx, TestRegion, TestBucket, key)
		assert.NilError(t, err)
		assert.Assert(t, pURL != "")

		err = uploadTxtByPresignedPutObjectURL(pURL)
		assert.NilError(t, err)
		err = confirmedUploadedObject(ctx, key)
		assert.NilError(t, err)
	})
}

func TestCreateMultipartUpload(t *testing.T) {
	t.Parallel()
	ctx := ctxawslocal.WithContext(
		context.Background(),
		ctxawslocal.WithS3Endpoint("http://127.0.0.1:29000"), // use Minio
		ctxawslocal.WithAccessKey("DUMMYACCESSKEYEXAMPLE"),
		ctxawslocal.WithSecretAccessKey("DUMMYSECRETKEYEXAMPLE"),
	)

	t.Run("Create multipart upload with existing bucket", func(t *testing.T) {
		t.Parallel()
		key := awss3.Key("test_create_multipart_upload_file_a.txt")
		uploadId, err := awss3.CreateMultipartUpload(ctx, TestRegion, TestBucket, key)
		assert.NilError(t, err)
		assert.Assert(t, uploadId != "")

		defer awss3.AbortMultipartUpload(ctx, TestRegion, TestBucket, key, uploadId) //nolint:errcheck
	})

	t.Run("Create multipart upload with non-existing bucket", func(t *testing.T) {
		t.Parallel()
		key := awss3.Key("test_create_multipart_upload_file_b.txt")
		uploadId, err := awss3.CreateMultipartUpload(ctx, TestRegion, NonExistentBucket, key)
		assert.Assert(t, err != nil)
		assert.Equal(t, "", uploadId)
	})
}

func TestAbortMultipartUpload(t *testing.T) {
	t.Parallel()
	ctx := ctxawslocal.WithContext(
		context.Background(),
		ctxawslocal.WithS3Endpoint("http://127.0.0.1:29000"), // use Minio
		ctxawslocal.WithAccessKey("DUMMYACCESSKEYEXAMPLE"),
		ctxawslocal.WithSecretAccessKey("DUMMYSECRETKEYEXAMPLE"),
	)

	t.Run("Abort multipart upload with existing uploadId", func(t *testing.T) {
		t.Parallel()
		key := awss3.Key("test_abort_multipart_upload_file_a.txt")
		uploadId, err := awss3.CreateMultipartUpload(ctx, TestRegion, TestBucket, key)
		assert.NilError(t, err)
		assert.Assert(t, uploadId != "")
		assert.NilError(t, awss3.AbortMultipartUpload(ctx, TestRegion, TestBucket, key, uploadId))
	})

	t.Run("Abort multipart upload with non-existing uploadId", func(t *testing.T) {
		t.Parallel()
		key := awss3.Key("test_abort_multipart_upload_file_b.txt")
		err := awss3.AbortMultipartUpload(ctx, TestRegion, TestBucket, key, "non-existing-upload-id")
		assert.Assert(t, err != nil)
	})
}

func TestUploadPart(t *testing.T) {
	t.Parallel()
	ctx := ctxawslocal.WithContext(
		context.Background(),
		ctxawslocal.WithS3Endpoint("http://127.0.0.1:29000"), // use Minio
		ctxawslocal.WithAccessKey("DUMMYACCESSKEYEXAMPLE"),
		ctxawslocal.WithSecretAccessKey("DUMMYSECRETKEYEXAMPLE"),
	)

	t.Run("Upload part with existing uploadId", func(t *testing.T) {
		t.Parallel()
		key := awss3.Key("test_upload_part_file_a.txt")
		uploadId, err := awss3.CreateMultipartUpload(ctx, TestRegion, TestBucket, key)
		assert.NilError(t, err)
		assert.Assert(t, uploadId != "")
		defer awss3.AbortMultipartUpload(ctx, TestRegion, TestBucket, key, uploadId) //nolint:errcheck

		resp, err := awss3.UploadPart(ctx, TestRegion, TestBucket, key, uploadId, 1, strings.NewReader("Hello World"))
		assert.NilError(t, err)
		assert.Assert(t, resp != nil)
	})
	t.Run("Upload part with non-existing uploadId", func(t *testing.T) {
		t.Parallel()
		key := awss3.Key("test_upload_part_file_b.txt")
		resp, err := awss3.UploadPart(ctx, TestRegion, TestBucket, key, "non-existing-upload-id", 1,
			strings.NewReader("Hello World"))
		assert.Assert(t, err != nil)
		assert.Assert(t, resp == nil)
	})
}

func TestCompleteMultipartUpload(t *testing.T) {
	t.Parallel()
	ctx := ctxawslocal.WithContext(
		context.Background(),
		ctxawslocal.WithS3Endpoint("http://127.0.0.1:29000"), // use Minio
		ctxawslocal.WithAccessKey("DUMMYACCESSKEYEXAMPLE"),
		ctxawslocal.WithSecretAccessKey("DUMMYSECRETKEYEXAMPLE"),
	)

	t.Run("Complete multipart upload with enough parts", func(t *testing.T) {
		t.Parallel()
		key := awss3.Key("test_complete_multipart_upload_file_a.txt")
		uploadId, err := awss3.CreateMultipartUpload(ctx, TestRegion, TestBucket, key)
		assert.NilError(t, err)
		assert.Assert(t, uploadId != "")
		defer awss3.AbortMultipartUpload(ctx, TestRegion, TestBucket, key, uploadId) //nolint:errcheck

		partResp, err := awss3.UploadPart(ctx, TestRegion, TestBucket, key, uploadId, 1,
			strings.NewReader("Hello World"))
		assert.NilError(t, err)
		assert.Assert(t, partResp != nil)

		_, err = awss3.CompleteMultipartUpload(ctx, TestRegion, TestBucket, key, uploadId, []types.CompletedPart{
			{ETag: partResp.ETag, PartNumber: aws.Int32(1)},
		})
		assert.NilError(t, err)
	})

	t.Run("Complete multipart upload without parts", func(t *testing.T) {
		t.Parallel()
		key := awss3.Key("test_complete_multipart_upload_file_b.txt")
		uploadId, err := awss3.CreateMultipartUpload(ctx, TestRegion, TestBucket, key)
		assert.NilError(t, err)
		assert.Assert(t, uploadId != "")
		defer awss3.AbortMultipartUpload(ctx, TestRegion, TestBucket, key, uploadId) //nolint:errcheck

		_, err = awss3.CompleteMultipartUpload(ctx, TestRegion, TestBucket, key, uploadId, []types.CompletedPart{})
		assert.Assert(t, err != nil)
	})

	t.Run("Complete multipart upload with incorrect parts", func(t *testing.T) {
		t.Parallel()
		key := awss3.Key("test_complete_multipart_upload_file_c.txt")
		uploadId, err := awss3.CreateMultipartUpload(ctx, TestRegion, TestBucket, key)
		assert.NilError(t, err)
		assert.Assert(t, uploadId != "")
		defer awss3.AbortMultipartUpload(ctx, TestRegion, TestBucket, key, uploadId) //nolint:errcheck

		partResp, err := awss3.UploadPart(ctx, TestRegion, TestBucket, key, uploadId, 1,
			strings.NewReader("Hello World"))
		assert.NilError(t, err)
		assert.Assert(t, partResp != nil)

		_, err = awss3.CompleteMultipartUpload(ctx, TestRegion, TestBucket, key, uploadId, []types.CompletedPart{
			{ETag: partResp.ETag, PartNumber: aws.Int32(2)}, // incorrect part number
		})
		assert.Assert(t, err != nil)
	})

	t.Run("Full flow: Create, Abort multipart upload", func(t *testing.T) {
		t.Parallel()
		key := awss3.Key("test_full_flow_abort_multipart_upload_file.txt")
		uploadId, err := awss3.CreateMultipartUpload(ctx, TestRegion, TestBucket, key)
		assert.NilError(t, err)
		assert.Assert(t, uploadId != "")

		partResp, err := awss3.UploadPart(ctx, TestRegion, TestBucket, key, uploadId, 1,
			bytes.NewReader([]byte("This is a test file for multipart upload.")))
		assert.NilError(t, err)
		assert.Assert(t, partResp != nil)

		completeResp, err := awss3.CompleteMultipartUpload(ctx, TestRegion, TestBucket, key, uploadId,
			[]types.CompletedPart{{ETag: partResp.ETag, PartNumber: aws.Int32(1)}},
		)
		assert.NilError(t, err)
		assert.Assert(t, completeResp != nil)

		// Abort multipart upload even success or fail to complete to ensure no leftover parts in S3
		err = awss3.AbortMultipartUpload(ctx, TestRegion, TestBucket, key, uploadId)
		assert.Assert(t, err != nil)
	})
}

// openFDCount returns the number of file descriptors currently open by this
// process. It works on Linux (/proc/self/fd) and macOS (/dev/fd).
// On unsupported platforms it returns -1 so callers can skip the assertion.
func openFDCount() int {
	var dir string
	switch runtime.GOOS {
	case "linux":
		dir = "/proc/self/fd"
	case "darwin":
		dir = "/dev/fd"
	default:
		return -1
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return -1
	}
	return len(entries)
}

// assertNoFDLeak takes a before-snapshot of open FDs, runs fn, then asserts
// that the FD count has not grown. The check is skipped on platforms where
// openFDCount returns -1.
func assertNoFDLeak(t *testing.T, fn func()) {
	t.Helper()
	if runtime.GOOS != "linux" && runtime.GOOS != "darwin" {
		t.Skip("FD counting not supported on this platform")
	}

	// Force a GC so that any finalizer-closed FDs are collected before we
	// snapshot the baseline.
	runtime.GC()

	before := openFDCount()

	fn()

	// GC again so that any deferred/runtime-managed FDs are released.
	runtime.GC()

	after := openFDCount()
	assert.Check(t, after <= before,
		"file descriptor leak detected: before=%d after=%d", before, after)
}

// TestDownloadFiles_FilesClosed verifies that every *os.File opened inside
// DownloadFiles is closed on both the success path and the error path.
func TestDownloadFiles_FilesClosed(t *testing.T) {
	t.Parallel()
	ctx := ctxawslocal.WithContext(
		context.Background(),
		ctxawslocal.WithS3Endpoint("http://127.0.0.1:29000"),
		ctxawslocal.WithAccessKey("DUMMYACCESSKEYEXAMPLE"),
		ctxawslocal.WithSecretAccessKey("DUMMYSECRETKEYEXAMPLE"),
	)

	// Upload a small set of objects to use as fixtures.
	s3Client, err := awss3.GetClient(ctx, TestRegion)
	assert.NilError(t, err)

	const numFiles = 5
	keys := make(awss3.Keys, numFiles)
	for i := 0; i < numFiles; i++ {
		key := fmt.Sprintf("awstest/fdtest/%s.txt", ulid.MustNew())
		uploader := transfermanager.New(s3Client)
		input := &transfermanager.UploadObjectInput{
			Body:    strings.NewReader(fmt.Sprintf("fd-test-content-%d", i)),
			Bucket:  aws.String(TestBucket),
			Key:     aws.String(key),
			Expires: aws.Time(time.Now().Add(10 * time.Minute)),
		}
		if _, err := uploader.UploadObject(ctx, input); err != nil {
			t.Fatalf("fixture upload failed: %v", err)
		}
		keys[i] = awss3.Key(key)
	}

	t.Run("success path: all FDs closed", func(t *testing.T) {
		t.Parallel()
		assertNoFDLeak(t, func() {
			paths, err := awss3.DownloadFiles(ctx, TestRegion, TestBucket, keys, t.TempDir())
			assert.NilError(t, err)
			assert.Equal(t, numFiles, len(paths))
		})
	})

	t.Run("error path: FDs closed even when key does not exist", func(t *testing.T) {
		t.Parallel()
		missingKeys := awss3.Keys{
			awss3.Key(fmt.Sprintf("awstest/fdtest/missing-%s.txt", ulid.MustNew())),
		}
		assertNoFDLeak(t, func() {
			_, err := awss3.DownloadFiles(ctx, TestRegion, TestBucket, missingKeys, t.TempDir())
			assert.Assert(t, err != nil)
		})
	})

	t.Run("error path with FileNameReplacer: FDs closed on failure", func(t *testing.T) {
		t.Parallel()
		missingKeys := awss3.Keys{
			awss3.Key(fmt.Sprintf("awstest/fdtest/missing-%s.txt", ulid.MustNew())),
		}
		assertNoFDLeak(t, func() {
			_, err := awss3.DownloadFiles(ctx, TestRegion, TestBucket, missingKeys, t.TempDir(),
				s3download.WithFileNameReplacerFunc(func(s3Key, base string) string {
					return "renamed-" + base
				}),
			)
			assert.Assert(t, err != nil)
		})
	})
}

// TestDownloadFilesParallel_FilesClosed verifies that every *os.File opened
// inside DownloadFilesParallel is closed on both the success path and the
// error path.
func TestDownloadFilesParallel_FilesClosed(t *testing.T) {
	t.Parallel()
	ctx := ctxawslocal.WithContext(
		context.Background(),
		ctxawslocal.WithS3Endpoint("http://127.0.0.1:29000"),
		ctxawslocal.WithAccessKey("DUMMYACCESSKEYEXAMPLE"),
		ctxawslocal.WithSecretAccessKey("DUMMYSECRETKEYEXAMPLE"),
	)

	s3Client, err := awss3.GetClient(ctx, TestRegion)
	assert.NilError(t, err)

	const numFiles = 5
	keys := make(awss3.Keys, numFiles)
	for i := 0; i < numFiles; i++ {
		key := fmt.Sprintf("awstest/fdtest-parallel/%s.txt", ulid.MustNew())
		uploader := transfermanager.New(s3Client)
		input := &transfermanager.UploadObjectInput{
			Body:    strings.NewReader(fmt.Sprintf("fd-test-content-parallel-%d", i)),
			Bucket:  aws.String(TestBucket),
			Key:     aws.String(key),
			Expires: aws.Time(time.Now().Add(10 * time.Minute)),
		}
		if _, err := uploader.UploadObject(ctx, input); err != nil {
			t.Fatalf("fixture upload failed: %v", err)
		}
		keys[i] = awss3.Key(key)
	}

	t.Run("success path: all FDs closed", func(t *testing.T) {
		t.Parallel()
		assertNoFDLeak(t, func() {
			paths, err := awss3.DownloadFilesParallel(ctx, TestRegion, TestBucket, keys, t.TempDir())
			assert.NilError(t, err)
			assert.Equal(t, numFiles, len(paths))
		})
	})

	t.Run("error path: FDs closed even when key does not exist", func(t *testing.T) {
		t.Parallel()
		missingKeys := awss3.Keys{
			awss3.Key(fmt.Sprintf("awstest/fdtest-parallel/missing-%s.txt", ulid.MustNew())),
		}
		assertNoFDLeak(t, func() {
			_, err := awss3.DownloadFilesParallel(ctx, TestRegion, TestBucket, missingKeys, t.TempDir())
			assert.Assert(t, err != nil)
		})
	})

	t.Run("error path with FileNameReplacer: FDs closed on failure", func(t *testing.T) {
		t.Parallel()
		missingKeys := awss3.Keys{
			awss3.Key(fmt.Sprintf("awstest/fdtest-parallel/missing-%s.txt", ulid.MustNew())),
		}
		assertNoFDLeak(t, func() {
			_, err := awss3.DownloadFilesParallel(ctx, TestRegion, TestBucket, missingKeys, t.TempDir(),
				s3download.WithFileNameReplacerFunc(func(s3Key, base string) string {
					return "renamed-" + base
				}),
			)
			assert.Assert(t, err != nil)
		})
	})

	t.Run("mixed: some keys exist, one does not — no FD leak", func(t *testing.T) {
		t.Parallel()
		mixedKeys := append(
			awss3.Keys{keys[0], keys[1]},
			awss3.Key(fmt.Sprintf("awstest/fdtest-parallel/missing-%s.txt", ulid.MustNew())),
		)
		assertNoFDLeak(t, func() {
			_, err := awss3.DownloadFilesParallel(ctx, TestRegion, TestBucket, mixedKeys, t.TempDir())
			assert.Assert(t, err != nil)
		})
	})
}
