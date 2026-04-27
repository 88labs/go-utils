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
	"github.com/aws/smithy-go"
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
	"github.com/88labs/go-utils/aws/awss3/options/s3upload"
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
	t.Run("ListObjects error: non-existent bucket", func(t *testing.T) {
		t.Parallel()
		_, err := awss3.ListObjects(ctx, TestRegion, NonExistentBucket)
		assert.Assert(t, err != nil)
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
	t.Run("Error:KeyNotFound returns ErrNotFound", func(t *testing.T) {
		t.Parallel()
		missingKeys := awss3.Keys{
			awss3.Key(fmt.Sprintf("awstest/missing-%s.txt", ulid.MustNew())),
		}
		_, err := awss3.DownloadFilesParallel(ctx, TestRegion, TestBucket, missingKeys, t.TempDir())
		assert.Assert(t, err != nil)
		assert.ErrorIs(t, err, awss3.ErrNotFound)
	})
	t.Run("Error:no partially written files remain on failure", func(t *testing.T) {
		t.Parallel()
		missingKeys := awss3.Keys{
			awss3.Key(fmt.Sprintf("awstest/missing-%s.txt", ulid.MustNew())),
			awss3.Key(fmt.Sprintf("awstest/missing-%s.txt", ulid.MustNew())),
		}
		outDir := t.TempDir()
		_, err := awss3.DownloadFilesParallel(ctx, TestRegion, TestBucket, missingKeys, outDir)
		assert.Assert(t, err != nil)
		entries, readErr := os.ReadDir(outDir)
		assert.NilError(t, readErr)
		assert.Equal(t, 0, len(entries),
			"partially written files must be removed on failure, found: %d file(s)", len(entries))
	})
	t.Run("duplicate keys are deduplicated", func(t *testing.T) {
		t.Parallel()
		// Pass the same key three times; only one file should be downloaded.
		dupKeys := awss3.Keys{keys[0], keys[0], keys[0]}
		filePaths, err := awss3.DownloadFilesParallel(ctx, TestRegion, TestBucket, dupKeys, t.TempDir())
		assert.NilError(t, err)
		assert.Equal(t, 1, len(filePaths))
		fileBody, err := os.ReadFile(filePaths[0])
		assert.NilError(t, err)
		assert.Equal(t, getBodyText(0), string(fileBody))
	})
	t.Run("empty keys returns empty slice", func(t *testing.T) {
		t.Parallel()
		filePaths, err := awss3.DownloadFilesParallel(ctx, TestRegion, TestBucket, awss3.Keys{}, t.TempDir())
		assert.NilError(t, err)
		assert.Equal(t, 0, len(filePaths))
	})
	t.Run("single key", func(t *testing.T) {
		t.Parallel()
		filePaths, err := awss3.DownloadFilesParallel(ctx, TestRegion, TestBucket,
			awss3.Keys{keys[0]}, t.TempDir())
		assert.NilError(t, err)
		assert.Equal(t, 1, len(filePaths))
		assert.Equal(t, filepath.Base(keys[0].String()), filepath.Base(filePaths[0]))
		fileBody, err := os.ReadFile(filePaths[0])
		assert.NilError(t, err)
		assert.Equal(t, getBodyText(0), string(fileBody))
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
	t.Run("PutObject with WithExpires option", func(t *testing.T) {
		t.Parallel()
		key := fmt.Sprintf("awstest/%s.txt", ulid.MustNew())
		body := faker.Sentence()
		_, err := awss3.PutObject(ctx, TestRegion, TestBucket, awss3.Key(key), strings.NewReader(body),
			s3upload.WithS3Expires(10*time.Minute),
		)
		assert.NilError(t, err)
		filePaths, err := awss3.DownloadFiles(ctx, TestRegion, TestBucket, awss3.NewKeys(key), t.TempDir())
		assert.NilError(t, err)
		assert.Assert(t, len(filePaths) == 1)
		fileBody, err := os.ReadFile(filePaths[0])
		assert.NilError(t, err)
		assert.Equal(t, body, string(fileBody))
	})
	t.Run("PutObject error: non-existent bucket", func(t *testing.T) {
		t.Parallel()
		key := fmt.Sprintf("awstest/%s.txt", ulid.MustNew())
		_, err := awss3.PutObject(ctx, TestRegion, NonExistentBucket, awss3.Key(key), strings.NewReader("body"))
		assert.Assert(t, err != nil)
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
	t.Run("UploadManager with WithExpires option", func(t *testing.T) {
		t.Parallel()
		key := fmt.Sprintf("awstest/%s.txt", ulid.MustNew())
		body := faker.Sentence()
		_, err := awss3.UploadManager(ctx, TestRegion, TestBucket, awss3.Key(key), strings.NewReader(body),
			s3upload.WithS3Expires(10*time.Minute),
		)
		assert.NilError(t, err)
		filePaths, err := awss3.DownloadFiles(ctx, TestRegion, TestBucket, awss3.NewKeys(key), t.TempDir())
		assert.NilError(t, err)
		assert.Equal(t, 1, len(filePaths))
		fileBody, err := os.ReadFile(filePaths[0])
		assert.NilError(t, err)
		assert.Equal(t, body, string(fileBody))
	})
	t.Run("UploadManager error: non-existent bucket", func(t *testing.T) {
		t.Parallel()
		key := fmt.Sprintf("awstest/%s.txt", ulid.MustNew())
		_, err := awss3.UploadManager(ctx, TestRegion, NonExistentBucket, awss3.Key(key), strings.NewReader("body"))
		assert.Assert(t, err != nil)
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
	uploadText := func(t testing.TB) awss3.Key {
		t.Helper()
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
			t.Fatal(err)
			return ""
		}
		return awss3.Key(key)
	}
	uploadPDF := func(t testing.TB) awss3.Key {
		t.Helper()
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
			t.Fatal(err)
			return ""
		}
		return awss3.Key(key)
	}

	t.Run("Presign", func(t *testing.T) {
		t.Parallel()
		key := uploadText(t)
		presign, err := awss3.Presign(ctx, TestRegion, TestBucket, key)
		assert.NilError(t, err)
		assert.Assert(t, presign != "")
	})
	t.Run("Presign PDF", func(t *testing.T) {
		t.Parallel()
		key := uploadPDF(t)
		presign, err := awss3.Presign(ctx, TestRegion, TestBucket, key)
		assert.NilError(t, err)
		assert.Assert(t, presign != "")
	})
	t.Run("Presign with WithExpires option", func(t *testing.T) {
		t.Parallel()
		key := uploadText(t)
		presign, err := awss3.Presign(ctx, TestRegion, TestBucket, key,
			s3presigned.WithPresignExpires(30*time.Minute),
		)
		assert.NilError(t, err)
		assert.Assert(t, presign != "")
	})
	t.Run("Presign with WithPresignFileName attachment", func(t *testing.T) {
		t.Parallel()
		key := uploadText(t)
		presign, err := awss3.Presign(ctx, TestRegion, TestBucket, key,
			s3presigned.WithPresignFileName("download.txt"),
			s3presigned.WithContentDispositionType(s3presigned.ContentDispositionTypeAttachment),
		)
		assert.NilError(t, err)
		assert.Assert(t, presign != "")
		assert.Assert(t, strings.Contains(presign, "response-content-disposition"))
	})
	t.Run("Presign with WithPresignFileName inline", func(t *testing.T) {
		t.Parallel()
		key := uploadText(t)
		presign, err := awss3.Presign(ctx, TestRegion, TestBucket, key,
			s3presigned.WithPresignFileName("view.txt"),
			s3presigned.WithContentDispositionType(s3presigned.ContentDispositionTypeInline),
		)
		assert.NilError(t, err)
		assert.Assert(t, presign != "")
		assert.Assert(t, strings.Contains(presign, "response-content-disposition"))
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
	t.Run("attachment", func(t *testing.T) {
		t.Parallel()
		actual := awss3.ResponseContentDisposition(s3presigned.ContentDispositionTypeAttachment, fileName)
		assert.Assert(t, actual != "")
	})
	t.Run("inline", func(t *testing.T) {
		t.Parallel()
		actual := awss3.ResponseContentDisposition(s3presigned.ContentDispositionTypeInline, fileName)
		assert.Assert(t, actual != "")
		assert.Assert(t, strings.Contains(actual, "inline"))
	})
	t.Run("ascii filename", func(t *testing.T) {
		t.Parallel()
		actual := awss3.ResponseContentDisposition(s3presigned.ContentDispositionTypeAttachment, "simple.txt")
		assert.Equal(t, `attachment; filename*=UTF-8''simple.txt`, actual)
	})
}

func TestCopy(t *testing.T) {
	t.Parallel()
	createFixture := func(t testing.TB, ctx context.Context) awss3.Key {
		t.Helper()
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

	t.Run("Copy:Same Bucket and Other Key", func(t *testing.T) {
		t.Parallel()
		ctx := ctxawslocal.WithContext(
			context.Background(),
			ctxawslocal.WithS3Endpoint("http://127.0.0.1:29000"), // use Minio
			ctxawslocal.WithAccessKey("DUMMYACCESSKEYEXAMPLE"),
			ctxawslocal.WithSecretAccessKey("DUMMYSECRETKEYEXAMPLE"),
		)
		key := createFixture(t, ctx)
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
		key := createFixture(t, ctx)
		assert.NilError(t, awss3.Copy(ctx, TestRegion, TestBucket, key, key))
	})
	t.Run("Copy with WithExpires option", func(t *testing.T) {
		t.Parallel()
		ctx := ctxawslocal.WithContext(
			context.Background(),
			ctxawslocal.WithS3Endpoint("http://127.0.0.1:29000"), // use Minio
			ctxawslocal.WithAccessKey("DUMMYACCESSKEYEXAMPLE"),
			ctxawslocal.WithSecretAccessKey("DUMMYSECRETKEYEXAMPLE"),
		)
		key := createFixture(t, ctx)
		key2 := awss3.Key(fmt.Sprintf("awstest/%s.txt", ulid.MustNew()))
		assert.NilError(t, awss3.Copy(ctx, TestRegion, TestBucket, key, key2, s3upload.WithS3Expires(10*time.Minute)))
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

func TestReservedCharacterKeys(t *testing.T) {
	t.Parallel()
	ctx := ctxawslocal.WithContext(
		context.Background(),
		ctxawslocal.WithS3Endpoint("http://127.0.0.1:29000"), // use Minio
		ctxawslocal.WithAccessKey("DUMMYACCESSKEYEXAMPLE"),
		ctxawslocal.WithSecretAccessKey("DUMMYSECRETKEYEXAMPLE"),
	)
	ensureBucket := func(t testing.TB) {
		t.Helper()
		s3Client, err := awss3.GetClient(ctx, TestRegion)
		assert.NilError(t, err)

		_, err = s3Client.HeadBucket(ctx, &s3.HeadBucketInput{Bucket: aws.String(TestBucket)})
		if err == nil {
			return
		}

		_, err = s3Client.CreateBucket(ctx, &s3.CreateBucketInput{Bucket: aws.String(TestBucket)})
		assert.NilError(t, err)
	}

	newKey := func(prefix string) awss3.Key {
		return awss3.Key(fmt.Sprintf("awstest/%s/%s&$@=;:+,?  reserved.txt", prefix, ulid.MustNew()))
	}
	readObject := func(t testing.TB, key awss3.Key) string {
		t.Helper()
		var buf bytes.Buffer
		err := awss3.GetObjectWriter(ctx, TestRegion, TestBucket, key, &buf)
		assert.NilError(t, err)
		return buf.String()
	}
	uploadByPresignedPutObjectURL := func(t testing.TB, presignedURL, body string) {
		t.Helper()
		req, err := http.NewRequest(http.MethodPut, presignedURL, strings.NewReader(body))
		assert.NilError(t, err)

		req.Header.Set("Content-Type", "text/plain")
		resp, err := http.DefaultClient.Do(req)
		assert.NilError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
	}

	ensureBucket(t)

	t.Run("PutObject", func(t *testing.T) {
		t.Parallel()
		key := newKey("put-object")
		body := faker.Sentence()

		_, err := awss3.PutObject(ctx, TestRegion, TestBucket, key, strings.NewReader(body))
		assert.NilError(t, err)
		assert.Equal(t, body, readObject(t, key))
	})

	t.Run("UploadManager", func(t *testing.T) {
		t.Parallel()
		key := newKey("upload-manager")
		body := faker.Sentence()

		_, err := awss3.UploadManager(ctx, TestRegion, TestBucket, key, strings.NewReader(body))
		assert.NilError(t, err)
		assert.Equal(t, body, readObject(t, key))
	})

	t.Run("PresignPutObject", func(t *testing.T) {
		t.Parallel()
		key := newKey("presign-put-object")
		body := faker.Sentence()

		pURL, err := awss3.PresignPutObject(ctx, TestRegion, TestBucket, key)
		assert.NilError(t, err)
		assert.Assert(t, pURL != "")

		uploadByPresignedPutObjectURL(t, pURL, body)
		assert.Equal(t, body, readObject(t, key))
	})

	t.Run("Multipart upload", func(t *testing.T) {
		t.Parallel()
		key := newKey("multipart-upload")
		body := "This is a multipart upload body."

		uploadID, err := awss3.CreateMultipartUpload(ctx, TestRegion, TestBucket, key)
		assert.NilError(t, err)
		assert.Assert(t, uploadID != "")

		partResp, err := awss3.UploadPart(ctx, TestRegion, TestBucket, key, uploadID, 1, strings.NewReader(body))
		assert.NilError(t, err)
		assert.Assert(t, partResp != nil)

		_, err = awss3.CompleteMultipartUpload(ctx, TestRegion, TestBucket, key, uploadID, []types.CompletedPart{
			{ETag: partResp.ETag, PartNumber: aws.Int32(1)},
		})
		assert.NilError(t, err)
		assert.Equal(t, body, readObject(t, key))
	})

	t.Run("Copy", func(t *testing.T) {
		t.Parallel()
		srcKey := newKey("copy-src")
		destKey := newKey("copy-dest")
		body := faker.Sentence()

		_, err := awss3.UploadManager(ctx, TestRegion, TestBucket, srcKey, strings.NewReader(body))
		assert.NilError(t, err)

		err = awss3.Copy(ctx, TestRegion, TestBucket, srcKey, destKey)
		assert.NilError(t, err)
		assert.Equal(t, body, readObject(t, destKey))
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
	t.Run("Presign with WithExpires option", func(t *testing.T) {
		t.Parallel()
		key := awss3.Key("test_presign_put_object_expires_01.txt")
		pURL, err := awss3.PresignPutObject(ctx, TestRegion, TestBucket, key,
			s3presigned.WithPresignExpires(30*time.Minute),
		)
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
		// Minio の仕様により OS によってエラーが返される場合と返されない場合がある
		// エラーが返された場合のみ内容を検証する
		if err := awss3.AbortMultipartUpload(ctx, TestRegion, TestBucket, key, "non-existing-upload-id"); err != nil {
			var apiErr smithy.APIError
			assert.Assert(t, errors.As(err, &apiErr))
		}
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

		// Minio の仕様により OS によってエラーが返される場合と返されない場合がある
		// 完了済みの uploadId に対して Abort を呼んだ場合、エラーが返された場合のみ内容を検証する
		if err = awss3.AbortMultipartUpload(ctx, TestRegion, TestBucket, key, uploadId); err != nil {
			var apiErr smithy.APIError
			assert.Assert(t, errors.As(err, &apiErr))
		}
	})
}

// openFDCount returns the number of open file descriptors whose target path
// is inside outputDir. By scoping the count to the temporary output directory,
// we avoid false positives from socket FDs kept open by the HTTP connection
// pool or from unrelated parallel tests.
// Returns -1 when the FD directory is unreadable or on unsupported platforms.
func openFDCount(outputDir string) int {
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
	count := 0
	for _, e := range entries {
		target, err := os.Readlink(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		// Count only FDs whose resolved path starts with outputDir.
		if strings.HasPrefix(target, outputDir) {
			count++
		}
	}
	return count
}

// assertNoFDLeak snapshots the number of open FDs pointing inside outputDir
// before and after calling fn, then asserts the count has not grown.
// The check is skipped when either snapshot returns -1 (unsupported platform
// or unreadable FD directory).
func assertNoFDLeak(t *testing.T, outputDir string, fn func()) {
	t.Helper()

	// Force a GC so that any finalizer-closed FDs are flushed before
	// snapshotting the baseline.
	runtime.GC()

	before := openFDCount(outputDir)
	if before < 0 {
		t.Skip("FD counting not supported on this platform")
	}

	fn()

	// GC again so that any runtime-managed FDs are released.
	runtime.GC()

	after := openFDCount(outputDir)
	if after < 0 {
		t.Skip("FD counting not supported on this platform")
	}

	assert.Check(t, after <= before,
		"file descriptor leak detected: before=%d after=%d", before, after)
}

// TestDownloadFiles_FilesClosed verifies that every *os.File opened inside
// DownloadFiles is closed on both the success path and the error path.
func TestDownloadFiles_FilesClosed(t *testing.T) {
	// Run serially to prevent FD activity from other parallel tests from
	// polluting the before/after snapshots.
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
		outDir := t.TempDir()
		assertNoFDLeak(t, outDir, func() {
			paths, err := awss3.DownloadFiles(ctx, TestRegion, TestBucket, keys, outDir)
			assert.NilError(t, err)
			assert.Equal(t, numFiles, len(paths))
		})
	})

	t.Run("error path: FDs closed even when key does not exist", func(t *testing.T) {
		missingKeys := awss3.Keys{
			awss3.Key(fmt.Sprintf("awstest/fdtest/missing-%s.txt", ulid.MustNew())),
		}
		outDir := t.TempDir()
		assertNoFDLeak(t, outDir, func() {
			_, err := awss3.DownloadFiles(ctx, TestRegion, TestBucket, missingKeys, outDir)
			assert.Assert(t, err != nil)
		})
	})

	t.Run("error path with FileNameReplacer: FDs closed on failure", func(t *testing.T) {
		missingKeys := awss3.Keys{
			awss3.Key(fmt.Sprintf("awstest/fdtest/missing-%s.txt", ulid.MustNew())),
		}
		outDir := t.TempDir()
		assertNoFDLeak(t, outDir, func() {
			_, err := awss3.DownloadFiles(ctx, TestRegion, TestBucket, missingKeys, outDir,
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
	// Run serially to prevent FD activity from other parallel tests from
	// polluting the before/after snapshots.
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
		outDir := t.TempDir()
		assertNoFDLeak(t, outDir, func() {
			paths, err := awss3.DownloadFilesParallel(ctx, TestRegion, TestBucket, keys, outDir)
			assert.NilError(t, err)
			assert.Equal(t, numFiles, len(paths))
		})
	})

	t.Run("error path: FDs closed even when key does not exist", func(t *testing.T) {
		missingKeys := awss3.Keys{
			awss3.Key(fmt.Sprintf("awstest/fdtest-parallel/missing-%s.txt", ulid.MustNew())),
		}
		outDir := t.TempDir()
		assertNoFDLeak(t, outDir, func() {
			_, err := awss3.DownloadFilesParallel(ctx, TestRegion, TestBucket, missingKeys, outDir)
			assert.Assert(t, err != nil)
		})
	})

	t.Run("error path with FileNameReplacer: FDs closed on failure", func(t *testing.T) {
		missingKeys := awss3.Keys{
			awss3.Key(fmt.Sprintf("awstest/fdtest-parallel/missing-%s.txt", ulid.MustNew())),
		}
		outDir := t.TempDir()
		assertNoFDLeak(t, outDir, func() {
			_, err := awss3.DownloadFilesParallel(ctx, TestRegion, TestBucket, missingKeys, outDir,
				s3download.WithFileNameReplacerFunc(func(s3Key, base string) string {
					return "renamed-" + base
				}),
			)
			assert.Assert(t, err != nil)
		})
	})

	t.Run("mixed: some keys exist, one does not — no FD leak", func(t *testing.T) {
		mixedKeys := append(
			awss3.Keys{keys[0], keys[1]},
			awss3.Key(fmt.Sprintf("awstest/fdtest-parallel/missing-%s.txt", ulid.MustNew())),
		)
		outDir := t.TempDir()
		assertNoFDLeak(t, outDir, func() {
			_, err := awss3.DownloadFilesParallel(ctx, TestRegion, TestBucket, mixedKeys, outDir)
			assert.Assert(t, err != nil)
		})
	})
}

func TestKey(t *testing.T) {
	t.Parallel()

	t.Run("String", func(t *testing.T) {
		t.Parallel()
		key := awss3.Key("path/to/file.txt")
		assert.Equal(t, "path/to/file.txt", key.String())
	})
	t.Run("AWSString", func(t *testing.T) {
		t.Parallel()
		key := awss3.Key("path/to/file.txt")
		assert.DeepEqual(t, aws.String("path/to/file.txt"), key.AWSString())
	})
	t.Run("BucketJoinAWSString", func(t *testing.T) {
		t.Parallel()
		key := awss3.Key("path/to/file.txt")
		bucket := awss3.BucketName("my-bucket")
		assert.DeepEqual(t, aws.String("my-bucket/path/to/file.txt"), key.BucketJoinAWSString(bucket))
	})
	t.Run("Ext lowercase", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, ".txt", awss3.Key("file.TXT").Ext())
		assert.Equal(t, ".pdf", awss3.Key("path/to/file.PDF").Ext())
	})
	t.Run("Ext no extension", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "", awss3.Key("file").Ext())
	})
}

func TestBucketName(t *testing.T) {
	t.Parallel()

	t.Run("String", func(t *testing.T) {
		t.Parallel()
		bucket := awss3.BucketName("my-bucket")
		assert.Equal(t, "my-bucket", bucket.String())
	})
	t.Run("AWSString", func(t *testing.T) {
		t.Parallel()
		bucket := awss3.BucketName("my-bucket")
		assert.DeepEqual(t, aws.String("my-bucket"), bucket.AWSString())
	})
}

func TestNewKeys(t *testing.T) {
	t.Parallel()

	t.Run("creates Keys from strings", func(t *testing.T) {
		t.Parallel()
		keys := awss3.NewKeys("a.txt", "b.txt", "c.txt")
		assert.Equal(t, 3, len(keys))
		assert.Equal(t, awss3.Key("a.txt"), keys[0])
		assert.Equal(t, awss3.Key("b.txt"), keys[1])
		assert.Equal(t, awss3.Key("c.txt"), keys[2])
	})
	t.Run("empty", func(t *testing.T) {
		t.Parallel()
		keys := awss3.NewKeys()
		assert.Equal(t, 0, len(keys))
	})
}

func TestKeys_Unique(t *testing.T) {
	t.Parallel()

	t.Run("removes duplicates", func(t *testing.T) {
		t.Parallel()
		keys := awss3.Keys{"a.txt", "b.txt", "a.txt", "c.txt", "b.txt"}
		got := keys.Unique()
		assert.DeepEqual(t, awss3.Keys{"a.txt", "b.txt", "c.txt"}, got)
	})
	t.Run("no duplicates", func(t *testing.T) {
		t.Parallel()
		keys := awss3.Keys{"a.txt", "b.txt", "c.txt"}
		got := keys.Unique()
		assert.DeepEqual(t, awss3.Keys{"a.txt", "b.txt", "c.txt"}, got)
	})
	t.Run("empty", func(t *testing.T) {
		t.Parallel()
		keys := awss3.Keys{}
		got := keys.Unique()
		assert.Equal(t, 0, len(got))
	})
	t.Run("all duplicates", func(t *testing.T) {
		t.Parallel()
		keys := awss3.Keys{"a.txt", "a.txt", "a.txt"}
		got := keys.Unique()
		assert.DeepEqual(t, awss3.Keys{"a.txt"}, got)
	})
}

func TestObjects_Find(t *testing.T) {
	t.Parallel()

	makeKey := func(s string) *string { return &s }
	objects := awss3.Objects{
		{Key: makeKey("path/to/file1.txt")},
		{Key: makeKey("path/to/file2.txt")},
		{Key: nil},
	}

	t.Run("found", func(t *testing.T) {
		t.Parallel()
		obj, ok := objects.Find("path/to/file1.txt")
		assert.Equal(t, true, ok)
		assert.Equal(t, "path/to/file1.txt", *obj.Key)
	})
	t.Run("not found", func(t *testing.T) {
		t.Parallel()
		_, ok := objects.Find("not/exist.txt")
		assert.Equal(t, false, ok)
	})
	t.Run("nil key object is skipped", func(t *testing.T) {
		t.Parallel()
		// nil キーを持つオブジェクトは無視されるため panicしない
		_, ok := objects.Find("")
		assert.Equal(t, false, ok)
	})
}

// TestNewClient_returnsWorkingClient verifies that NewClient constructs a client
// that can successfully upload and inspect an object on S3.
func TestNewClient_returnsWorkingClient(t *testing.T) {
	t.Parallel()
	ctx := ctxawslocal.WithContext(
		context.Background(),
		ctxawslocal.WithS3Endpoint("http://127.0.0.1:29000"),
		ctxawslocal.WithAccessKey("DUMMYACCESSKEYEXAMPLE"),
		ctxawslocal.WithSecretAccessKey("DUMMYSECRETKEYEXAMPLE"),
	)

	client, err := awss3.NewClient(ctx, TestRegion)
	assert.NilError(t, err)

	key := awss3.Key(fmt.Sprintf("awstest/%s.txt", ulid.MustNew()))
	_, err = client.PutObject(ctx, TestBucket, key, bytes.NewReader(bytes.Repeat([]byte{1}, 64)))
	assert.NilError(t, err)

	res, err := client.HeadObject(ctx, TestBucket, key)
	assert.NilError(t, err)
	assert.DeepEqual(t, aws.Int64(64), res.ContentLength)
}

// TestNewClient_S3Client_exposesUnderlyingSDKClient verifies that the underlying
// *s3.Client can be retrieved for advanced usage.
func TestNewClient_S3Client_exposesUnderlyingSDKClient(t *testing.T) {
	t.Parallel()
	ctx := ctxawslocal.WithContext(
		context.Background(),
		ctxawslocal.WithS3Endpoint("http://127.0.0.1:29000"),
		ctxawslocal.WithAccessKey("DUMMYACCESSKEYEXAMPLE"),
		ctxawslocal.WithSecretAccessKey("DUMMYSECRETKEYEXAMPLE"),
	)

	client, err := awss3.NewClient(ctx, TestRegion)
	assert.NilError(t, err)
	assert.Assert(t, client.S3Client() != nil)
}

// TestNewClient_isIndependentFromSingleton verifies that a Client created via
// NewClient operates independently from the package-level singleton.
// Two separate clients must be able to reach the same object.
func TestNewClient_isIndependentFromSingleton(t *testing.T) {
	t.Parallel()
	ctx := ctxawslocal.WithContext(
		context.Background(),
		ctxawslocal.WithS3Endpoint("http://127.0.0.1:29000"),
		ctxawslocal.WithAccessKey("DUMMYACCESSKEYEXAMPLE"),
		ctxawslocal.WithSecretAccessKey("DUMMYSECRETKEYEXAMPLE"),
	)

	// Upload via the package-level singleton.
	key := awss3.Key(fmt.Sprintf("awstest/%s.txt", ulid.MustNew()))
	_, err := awss3.PutObject(ctx, TestRegion, TestBucket, key, bytes.NewReader([]byte("hello")))
	assert.NilError(t, err)

	// Read back via a separately created Client — must succeed without sharing state.
	client, err := awss3.NewClient(ctx, TestRegion)
	assert.NilError(t, err)

	res, err := client.HeadObject(ctx, TestBucket, key)
	assert.NilError(t, err)
	assert.DeepEqual(t, aws.Int64(5), res.ContentLength)
}

// TestNewClient_DeleteObject_objectIsGoneAfterDeletion verifies that an object
// uploaded via NewClient is no longer accessible after DeleteObject.
func TestNewClient_DeleteObject_objectIsGoneAfterDeletion(t *testing.T) {
	t.Parallel()
	ctx := ctxawslocal.WithContext(
		context.Background(),
		ctxawslocal.WithS3Endpoint("http://127.0.0.1:29000"),
		ctxawslocal.WithAccessKey("DUMMYACCESSKEYEXAMPLE"),
		ctxawslocal.WithSecretAccessKey("DUMMYSECRETKEYEXAMPLE"),
	)

	client, err := awss3.NewClient(ctx, TestRegion)
	assert.NilError(t, err)

	key := awss3.Key(fmt.Sprintf("awstest/%s.txt", ulid.MustNew()))
	_, err = client.PutObject(ctx, TestBucket, key, bytes.NewReader([]byte("hello")))
	assert.NilError(t, err)

	_, err = client.DeleteObject(ctx, TestBucket, key)
	assert.NilError(t, err)

	_, err = client.HeadObject(ctx, TestBucket, key)
	assert.ErrorIs(t, err, awss3.ErrNotFound)
}

// TestNewClient_DeleteObject_notFoundReturnsErrNotFound verifies that deleting
// a non-existent object returns ErrNotFound.
func TestNewClient_DeleteObject_notFoundReturnsErrNotFound(t *testing.T) {
	t.Parallel()
	ctx := ctxawslocal.WithContext(
		context.Background(),
		ctxawslocal.WithS3Endpoint("http://127.0.0.1:29000"),
		ctxawslocal.WithAccessKey("DUMMYACCESSKEYEXAMPLE"),
		ctxawslocal.WithSecretAccessKey("DUMMYSECRETKEYEXAMPLE"),
	)

	client, err := awss3.NewClient(ctx, TestRegion)
	assert.NilError(t, err)

	_, err = client.DeleteObject(ctx, TestBucket, awss3.Key(fmt.Sprintf("awstest/missing-%s.txt", ulid.MustNew())))
	assert.ErrorIs(t, err, awss3.ErrNotFound)
}
