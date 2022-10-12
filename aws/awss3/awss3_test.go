package awss3_test

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

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

	t.Run("GetObjectWriter", func(t *testing.T) {
		key := createFixture()
		var buf bytes.Buffer
		err := awss3.GetObjectWriter(ctx, TestRegion, TestBucket, key, &buf)
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
	})
	t.Run("FileNameReplacer:duplicate", func(t *testing.T) {
		filePaths, err := awss3.DownloadFiles(ctx, TestRegion, TestBucket, keys, t.TempDir(),
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
		ctxawslocal.WithAccessKey("DUMMYACCESSKEYEXAMPLE"),
		ctxawslocal.WithSecretAccessKey("DUMMYSECRETKEYEXAMPLE"),
	)
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
