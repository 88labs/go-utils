package awss3

import (
	"bytes"
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/cenkalti/backoff/v4"

	"github.com/88labs/go-utils/aws/awss3/options/s3list"

	"github.com/88labs/go-utils/aws/awss3/options/s3head"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"
	awshttp "github.com/aws/smithy-go/transport/http"
	"github.com/tomtwinkle/utfbomremover"
	"golang.org/x/sync/errgroup"
	"golang.org/x/text/transform"

	"github.com/88labs/go-utils/aws/awsconfig"
	"github.com/88labs/go-utils/aws/awss3/options/s3download"
	"github.com/88labs/go-utils/aws/awss3/options/s3presigned"
	"github.com/88labs/go-utils/aws/awss3/options/s3selectcsv"
	"github.com/88labs/go-utils/aws/awss3/options/s3upload"
)

var ErrNotFound = errors.New("NotFound")

type BucketName string

func (k BucketName) String() string {
	return string(k)
}

func (k BucketName) AWSString() *string {
	return aws.String(string(k))
}

type Key string

func (k Key) String() string {
	return string(k)
}

func (k Key) AWSString() *string {
	return aws.String(string(k))
}

func (k Key) BucketJoinAWSString(bucketName BucketName) *string {
	return aws.String(path.Join(bucketName.String(), k.String()))
}

func (k Key) Ext() string {
	return strings.ToLower(filepath.Ext(string(k)))
}

type Keys []Key

func NewKeys(keys ...string) Keys {
	ks := make(Keys, len(keys))
	for i, k := range keys {
		ks[i] = Key(k)
	}
	return ks
}

func (ks Keys) Unique() Keys {
	keys := make(Keys, 0, len(ks))
	uniq := make(map[Key]struct{})
	for _, k := range ks {
		if _, ok := uniq[k]; ok {
			continue
		}
		uniq[k] = struct{}{}
		keys = append(keys, k)
	}
	return keys
}

type Objects []types.Object

func (o Objects) Find(key Key) (types.Object, bool) {
	for _, v := range o {
		if v.Key == nil {
			continue
		}
		if *v.Key == key.String() {
			return v, true
		}
	}
	return types.Object{}, false
}

// PutObject
// aws-sdk-go v2 PutObject
// If there is no particular reason to use PutObject, please use UploadManager
//
// Mocks: Using ctxawslocal.WithContext, you can make requests for local mocks.
//
// Notes
// https://aws.github.io/aws-sdk-go-v2/docs/sdk-utilities/s3/#unseekable-streaming-input
// Amazon S3 requires the content length to be provided for all object’s uploaded to a bucket.
// Since the Body input parameter does not implement io.Seeker interface the client will not be able to compute the ContentLength parameter for the request.
// The parameter must be provided by the application. The request will fail if the ContentLength parameter is not provided.
func PutObject(ctx context.Context, region awsconfig.Region, bucketName BucketName, key Key, body io.Reader, opts ...s3upload.OptionS3Upload) (*s3.PutObjectOutput, error) {
	c := s3upload.GetS3UploadConf(opts...)
	client, err := GetClient(ctx, region) // nolint:typecheck
	if err != nil {
		return nil, err
	}
	input := &s3.PutObjectInput{
		Body:   body,
		Bucket: bucketName.AWSString(),
		Key:    key.AWSString(),
	}
	if c.S3Expires != nil {
		input.Expires = aws.Time(time.Now().Add(*c.S3Expires))
	}
	return client.PutObject(ctx, input)
}

// UploadManager
// Upload using the manager.Uploader of aws-sdk-go v2
//
// Mocks: Using ctxawslocal.WithContext, you can make requests for local mocks.
func UploadManager(ctx context.Context, region awsconfig.Region, bucketName BucketName, key Key, body io.Reader, opts ...s3upload.OptionS3Upload) (*manager.UploadOutput, error) {
	c := s3upload.GetS3UploadConf(opts...)
	client, err := GetClient(ctx, region) // nolint:typecheck

	uploader := manager.NewUploader(client)
	if err != nil {
		return nil, err
	}
	input := &s3.PutObjectInput{
		Body:   body,
		Bucket: bucketName.AWSString(),
		Key:    key.AWSString(),
	}
	if c.S3Expires != nil {
		input.Expires = aws.Time(time.Now().Add(*c.S3Expires))
	}
	return uploader.Upload(ctx, input)
}

// HeadObject
// aws-sdk-go v2 HeadObject
//
// Mocks: Using ctxawslocal.WithContext, you can make requests for local mocks.
func HeadObject(ctx context.Context, region awsconfig.Region, bucketName BucketName, key Key, opts ...s3head.OptionS3Head) (*s3.HeadObjectOutput, error) {
	client, err := GetClient(ctx, region) // nolint:typecheck
	if err != nil {
		return nil, err
	}

	c := s3head.GetS3HeadConf(opts...)
	if c.Timeout > 0 {
		waiter := s3.NewObjectExistsWaiter(client, func(options *s3.ObjectExistsWaiterOptions) {
			options.MinDelay = c.MinDelay
			options.MaxDelay = c.MaxDelay
			options.LogWaitAttempts = c.LogWaitAttempts
		})
		err := waiter.Wait(ctx, &s3.HeadObjectInput{
			Bucket: bucketName.AWSString(),
			Key:    key.AWSString(),
		}, c.Timeout)
		if err != nil {
			return nil, fmt.Errorf("%w:%v", ErrNotFound, err)
		}
	}
	res, err := client.HeadObject(
		ctx,
		&s3.HeadObjectInput{
			Bucket: bucketName.AWSString(),
			Key:    key.AWSString(),
		})
	if err != nil {
		var nond *types.NotFound
		if errors.As(err, &nond) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return res, nil
}

// ListObjects
// aws-sdk-go v2 ListObjectsV2
//
// Mocks: Using ctxawslocal.WithContext, you can make requests for local mocks.
func ListObjects(ctx context.Context, region awsconfig.Region, bucketName BucketName, opts ...s3list.OptionS3List) (Objects, error) {
	client, err := GetClient(ctx, region) // nolint:typecheck
	if err != nil {
		return nil, err
	}
	c := s3list.GetS3ListConf(opts...)

	input := &s3.ListObjectsV2Input{
		Bucket: bucketName.AWSString(),
	}
	if c.Prefix != nil {
		input.Prefix = c.Prefix
	}
	objects := make(Objects, 0)
	paginator := s3.NewListObjectsV2Paginator(client, input)
	for paginator.HasMorePages() {
		output, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		objects = append(objects, output.Contents...)
	}
	return objects, nil
}

// GetObjectWriter
// aws-sdk-go v2 GetObject output io.Writer
//
// Mocks: Using ctxawslocal.WithContext, you can make requests for local mocks.
func GetObjectWriter(ctx context.Context, region awsconfig.Region, bucketName BucketName, key Key, w io.Writer) error {
	if _, err := HeadObject(ctx, region, bucketName, key); err != nil {
		return err
	}

	client, err := GetClient(ctx, region) // nolint:typecheck
	if err != nil {
		return err
	}
	resp, err := client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: bucketName.AWSString(),
		Key:    key.AWSString(),
	})
	if err != nil {
		var nond *types.NotFound
		if errors.As(err, &nond) {
			return ErrNotFound
		}
		return err
	}
	if _, err := io.Copy(w, resp.Body); err != nil {
		return err
	}
	return nil
}

// DeleteObject
// aws-sdk-go v2 DeleteObject
// Mocks: Using ctxawslocal.WithContext, you can make requests for local mocks.
func DeleteObject(ctx context.Context, region awsconfig.Region, bucketName BucketName, key Key) (*s3.DeleteObjectOutput, error) {
	if _, err := HeadObject(ctx, region, bucketName, key); err != nil {
		return nil, err
	}

	client, err := GetClient(ctx, region) // nolint:typecheck
	if err != nil {
		return nil, err
	}
	input := &s3.DeleteObjectInput{
		Bucket: bucketName.AWSString(),
		Key:    key.AWSString(),
	}
	return client.DeleteObject(ctx, input)
}

// DownloadFiles
// Batch download objects on s3 and save to directory
// If the file name is duplicated, add a sequential number to the suffix and save
//
// Mocks: Using ctxawslocal.WithContext, you can make requests for local mocks.
func DownloadFiles(ctx context.Context, region awsconfig.Region, bucketName BucketName, keys Keys, outputDir string, opts ...s3download.OptionS3Download) ([]string, error) {
	c := s3download.GetS3DownloadConf(opts...)

	client, err := GetClient(ctx, region) // nolint:typecheck
	if err != nil {
		return nil, err
	}

	uniqKeys := keys.Unique()
	option := func(d *manager.Downloader) {
		d.BufferProvider = manager.NewPooledBufferedWriterReadFromProvider(5 * 1024 * 1024)
	}
	downloader := manager.NewDownloader(client, option)
	paths := make([]string, len(uniqKeys))

	getFilePath := func(s3Key string) string {
		fileName := filepath.Base(s3Key)
		if c.FileNameReplacer != nil {
			fileName = c.FileNameReplacer(s3Key, fileName)
		}
		filePath := path.Join(outputDir, fileName)
		var existsFileCount int
		for {
			if existsFileCount > 0 {
				// If the file name is duplicated, add a sequential number to the suffix
				ext := filepath.Ext(fileName)
				newFileName := fmt.Sprintf("%s_%d%s", strings.TrimSuffix(fileName, ext), existsFileCount+1, ext)
				filePath = path.Join(outputDir, newFileName)
			}
			if _, err := os.Stat(filePath); err != nil {
				break
			}
			existsFileCount++
		}
		return filePath
	}

	var eg errgroup.Group
	for i := range uniqKeys {
		i := i
		s3Key := uniqKeys[i]
		filePath := getFilePath(s3Key.String())
		paths[i] = filePath
		f, err := os.Create(filePath)
		if err != nil {
			return nil, err
		}
		eg.Go(func() error {
			b := backoff.WithContext(backoff.NewExponentialBackOff(), ctx)
			if err := backoff.Retry(func() error {
				if _, err := downloader.Download(ctx, f, &s3.GetObjectInput{
					Bucket: bucketName.AWSString(),
					Key:    s3Key.AWSString(),
				}); err != nil {
					return err
				}
				return nil
			}, b); err != nil {
				return err
			}
			return nil
		})
	}
	if err := eg.Wait(); err != nil {
		return nil, err
	}
	return paths, nil
}

// Presign
// aws-sdk-go v2 Presign
// default expires is 15 minutes
//
// Mocks: Using ctxawslocal.WithContext, you can make requests for local mocks.
func Presign(ctx context.Context, region awsconfig.Region, bucketName BucketName, key Key, opts ...s3presigned.OptionS3Presigned) (string, error) {
	if _, err := HeadObject(ctx, region, bucketName, key); err != nil {
		return "", err
	}

	c := s3presigned.GetS3PresignedConf(opts...)
	client, err := GetClient(ctx, region) // nolint:typecheck
	if err != nil {
		return "", err
	}

	// @todo: not been able to test with and without option. create a separate function for input settings.
	input := &s3.GetObjectInput{
		Bucket: bucketName.AWSString(),
		Key:    key.AWSString(),
	}
	if c.PresignFileName != "" {
		input.ResponseContentDisposition = aws.String(ResponseContentDisposition(c.ContentDispositionType, c.PresignFileName))
	}
	// Note: Fixed a bug in which the response is returned with `Content-Type:pdf` in case of PDF.
	// convert to Content-Type: application/pdf.
	if key.Ext() == ".pdf" {
		input.ResponseContentType = aws.String("application/pdf")
	}
	ps := s3.NewPresignClient(client)
	resp, err := ps.PresignGetObject(ctx, input, func(o *s3.PresignOptions) {
		o.Expires = c.PresignExpires
	})
	if err != nil {
		return "", err
	}
	return resp.URL, nil
}

func ResponseContentDisposition(tp s3presigned.ContentDispositionType, fileName string) string {
	var dispositionType string
	switch tp {
	case s3presigned.ContentDispositionTypeAttachment:
		dispositionType = "attachment"
	case s3presigned.ContentDispositionTypeInline:
		dispositionType = "inline"
	}
	return fmt.Sprintf(`%s; filename*=UTF-8''%s`, dispositionType, url.PathEscape(fileName))
}

// Copy copies an Amazon S3 object from one bucket to same.
//
// Mocks: Using ctxawslocal.WithContext, you can make requests for local mocks.
func Copy(ctx context.Context, region awsconfig.Region, bucketName BucketName, srcKey, destKey Key, opts ...s3upload.OptionS3Upload) error {
	c := s3upload.GetS3UploadConf(opts...)
	client, err := GetClient(ctx, region) // nolint:typecheck
	if err != nil {
		return err
	}
	req := &s3.CopyObjectInput{
		Bucket:            bucketName.AWSString(),
		CopySource:        srcKey.BucketJoinAWSString(bucketName),
		Key:               destKey.AWSString(),
		MetadataDirective: types.MetadataDirectiveReplace,
	}
	if c.S3Expires != nil {
		req.Expires = aws.Time(time.Now().Add(*c.S3Expires))
	}
	if _, err := client.CopyObject(ctx, req); err != nil {
		var oe *smithy.OperationError
		if errors.As(err, &oe) {
			var resErr *awshttp.ResponseError
			if errors.As(oe.Err, &resErr) {
				if resErr.Response.StatusCode == http.StatusNotFound {
					return ErrNotFound
				}
			}
		}
		return err
	}
	return nil
}

const (
	SelectCSVAllQuery    = "SELECT * FROM S3Object"
	SelectCSVLimit1Query = "SELECT * FROM S3Object LIMIT 1"
)

// SelectCSVAll
// SQL Reference : https://docs.aws.amazon.com/AmazonS3/latest/userguide/s3-glacier-select-sql-reference-select.html
func SelectCSVAll(ctx context.Context, region awsconfig.Region, bucketName BucketName, key Key, query string, w io.Writer, opts ...s3selectcsv.OptionS3SelectCSV) error {
	c := s3selectcsv.GetS3SelectCSVConf(opts...)
	client, err := GetClient(ctx, region) // nolint:typecheck
	if err != nil {
		return err
	}

	req := &s3.SelectObjectContentInput{
		Bucket:         bucketName.AWSString(),
		Key:            key.AWSString(),
		ExpressionType: types.ExpressionTypeSql,
		Expression:     aws.String(query),
		InputSerialization: &types.InputSerialization{
			CSV: &types.CSVInput{
				AllowQuotedRecordDelimiter: c.CSVInput.AllowQuotedRecordDelimiter,
				Comments:                   c.CSVInput.Comments,
				FieldDelimiter:             c.CSVInput.FieldDelimiter,
				FileHeaderInfo:             c.CSVInput.FileHeaderInfo,
				QuoteCharacter:             c.CSVInput.QuoteCharacter,
				QuoteEscapeCharacter:       c.CSVInput.QuoteEscapeCharacter,
				RecordDelimiter:            c.CSVInput.RecordDelimiter,
			},
			CompressionType: c.CompressionType,
		},
		OutputSerialization: &types.OutputSerialization{
			CSV: &types.CSVOutput{
				FieldDelimiter:       c.CSVOutput.FieldDelimiter,
				QuoteCharacter:       c.CSVOutput.QuoteCharacter,
				QuoteEscapeCharacter: c.CSVOutput.QuoteEscapeCharacter,
				QuoteFields:          c.CSVOutput.QuoteFields,
				RecordDelimiter:      c.CSVOutput.RecordDelimiter,
			},
		},
	}
	if c.SkipByteSize > 0 {
		req.ScanRange = &types.ScanRange{Start: c.SkipByteSize}
	}
	resp, err := client.SelectObjectContent(ctx, req)
	if err != nil {
		if awsErr, ok := err.(*smithy.OperationError); ok {
			// 最終行まで取得してしまった場合レコードが0件になってしまうのでInvalidRange errorが発生する
			if awsErr.OperationName == "InvalidRange" {
				return nil
			}
		}
		return err
	}
	t := transform.NewWriter(w, utfbomremover.NewTransformer())
	for event := range resp.GetStream().Events() {
		switch v := event.(type) {
		case *types.SelectObjectContentEventStreamMemberRecords:
			// buffer毎にcall
			if _, err := t.Write(v.Value.Payload); err != nil {
				return err
			}
		case *types.SelectObjectContentEventStreamMemberStats:
			// 終了時1回のみ呼ばれる
			// v.Value.Details
		case *types.SelectObjectContentEventStreamMemberEnd:
			// SelectObjectContent completed
		}
	}
	if err := resp.GetStream().Close(); err != nil {
		return err
	}
	return nil
}

// SelectCSVHeaders
// Get CSV headers
// Valid options: CompressionType
func SelectCSVHeaders(ctx context.Context, region awsconfig.Region, bucketName BucketName, key Key, opts ...s3selectcsv.OptionS3SelectCSV) ([]string, error) {
	c := s3selectcsv.GetS3SelectCSVConf(opts...)
	opts = append(opts, s3selectcsv.WithCSVInput(types.CSVInput{
		AllowQuotedRecordDelimiter: c.CSVInput.AllowQuotedRecordDelimiter,
		Comments:                   c.CSVInput.Comments,
		FieldDelimiter:             c.CSVInput.FieldDelimiter,
		FileHeaderInfo:             types.FileHeaderInfoNone,
		QuoteCharacter:             c.CSVInput.QuoteCharacter,
		QuoteEscapeCharacter:       c.CSVInput.QuoteEscapeCharacter,
		RecordDelimiter:            c.CSVInput.RecordDelimiter,
	}))
	opts = append(opts, s3selectcsv.WithSkipByteSize(0))
	var buf bytes.Buffer
	if err := SelectCSVAll(ctx, region, bucketName, key, SelectCSVLimit1Query, &buf, opts...); err != nil {
		return nil, err
	}
	r := csv.NewReader(&buf)
	headers, err := r.Read()
	if err != nil {
		return nil, err
	}
	return headers, nil
}
