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

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"
	awshttp "github.com/aws/smithy-go/transport/http"
	"github.com/cenkalti/backoff/v4"
	"github.com/tomtwinkle/utfbomremover"
	"golang.org/x/sync/errgroup"
	"golang.org/x/text/transform"

	"github.com/88labs/go-utils/aws/awsconfig"
	"github.com/88labs/go-utils/aws/awss3/options/s3download"
	"github.com/88labs/go-utils/aws/awss3/options/s3head"
	"github.com/88labs/go-utils/aws/awss3/options/s3list"
	"github.com/88labs/go-utils/aws/awss3/options/s3presigned"
	"github.com/88labs/go-utils/aws/awss3/options/s3selectcsv"
	"github.com/88labs/go-utils/aws/awss3/options/s3upload"
)

var ErrNotFound = errors.New("NotFound")

// PutObject on Client
// aws-sdk-go v2 PutObject
func (c *Client) PutObject(
	ctx context.Context, bucketName BucketName, key Key, body io.Reader,
	opts ...s3upload.OptionS3Upload,
) (*s3.PutObjectOutput, error) {
	conf := s3upload.GetS3UploadConf(opts...)
	input := &s3.PutObjectInput{
		Body:   body,
		Bucket: bucketName.AWSString(),
		Key:    key.AWSString(),
	}
	if conf.S3Expires != nil {
		input.Expires = aws.Time(time.Now().Add(*conf.S3Expires))
	}
	return c.raw.PutObject(ctx, input)
}

// PutObject
// aws-sdk-go v2 PutObject
// If there is no particular reason to use PutObject, please use UploadManager
//
// Mocks: Using ctxawslocal.WithContext, you can make requests for local mocks.
//
// Notes
// https://aws.github.io/aws-sdk-go-v2/docs/sdk-utilities/s3/#unseekable-streaming-input
// Amazon S3 requires the content length to be provided for all object's uploaded to a bucket.
// Since the Body input parameter does not implement io.Seeker interface the client will not be able to compute the ContentLength parameter for the request.
// The parameter must be provided by the application. The request will fail if the ContentLength parameter is not provided.
func PutObject(
	ctx context.Context, region awsconfig.Region, bucketName BucketName, key Key, body io.Reader,
	opts ...s3upload.OptionS3Upload,
) (*s3.PutObjectOutput, error) {
	s3Client, err := GetClient(ctx, region)
	if err != nil {
		return nil, err
	}
	return (&Client{raw: s3Client}).PutObject(ctx, bucketName, key, body, opts...)
}

// UploadManager on Client
// Upload using the manager.Uploader of aws-sdk-go v2
func (c *Client) UploadManager(
	ctx context.Context, bucketName BucketName, key Key, body io.Reader,
	opts ...s3upload.OptionS3Upload,
) (*manager.UploadOutput, error) {
	conf := s3upload.GetS3UploadConf(opts...)
	uploader := manager.NewUploader(c.raw)
	input := &s3.PutObjectInput{
		Body:   body,
		Bucket: bucketName.AWSString(),
		Key:    key.AWSString(),
	}
	if conf.S3Expires != nil {
		input.Expires = aws.Time(time.Now().Add(*conf.S3Expires))
	}
	return uploader.Upload(ctx, input)
}

// UploadManager
// Upload using the manager.Uploader of aws-sdk-go v2
//
// Mocks: Using ctxawslocal.WithContext, you can make requests for local mocks.
func UploadManager(
	ctx context.Context, region awsconfig.Region, bucketName BucketName, key Key, body io.Reader,
	opts ...s3upload.OptionS3Upload,
) (*manager.UploadOutput, error) {
	s3Client, err := GetClient(ctx, region)
	if err != nil {
		return nil, err
	}
	return (&Client{raw: s3Client}).UploadManager(ctx, bucketName, key, body, opts...)
}

// HeadObject on Client
// aws-sdk-go v2 HeadObject
func (c *Client) HeadObject(
	ctx context.Context, bucketName BucketName, key Key, opts ...s3head.OptionS3Head,
) (*s3.HeadObjectOutput, error) {
	conf := s3head.GetS3HeadConf(opts...)
	if conf.Timeout > 0 {
		waiter := s3.NewObjectExistsWaiter(c.raw, func(options *s3.ObjectExistsWaiterOptions) {
			options.MinDelay = conf.MinDelay
			options.MaxDelay = conf.MaxDelay
			options.LogWaitAttempts = conf.LogWaitAttempts
		})
		err := waiter.Wait(ctx, &s3.HeadObjectInput{
			Bucket: bucketName.AWSString(),
			Key:    key.AWSString(),
		}, conf.Timeout)
		if err != nil {
			return nil, fmt.Errorf("%w:%v", ErrNotFound, err)
		}
	}
	res, err := c.raw.HeadObject(
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

// HeadObject
// aws-sdk-go v2 HeadObject
//
// Mocks: Using ctxawslocal.WithContext, you can make requests for local mocks.
func HeadObject(
	ctx context.Context, region awsconfig.Region, bucketName BucketName, key Key, opts ...s3head.OptionS3Head,
) (*s3.HeadObjectOutput, error) {
	s3Client, err := GetClient(ctx, region)
	if err != nil {
		return nil, err
	}
	return (&Client{raw: s3Client}).HeadObject(ctx, bucketName, key, opts...)
}

// ListObjects on Client
// aws-sdk-go v2 ListObjectsV2
func (c *Client) ListObjects(
	ctx context.Context, bucketName BucketName, opts ...s3list.OptionS3List,
) (Objects, error) {
	conf := s3list.GetS3ListConf(opts...)
	input := &s3.ListObjectsV2Input{
		Bucket: bucketName.AWSString(),
	}
	if conf.Prefix != nil {
		input.Prefix = conf.Prefix
	}
	objects := make(Objects, 0)
	paginator := s3.NewListObjectsV2Paginator(c.raw, input)
	for paginator.HasMorePages() {
		output, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		objects = append(objects, output.Contents...)
	}
	return objects, nil
}

// ListObjects
// aws-sdk-go v2 ListObjectsV2
//
// Mocks: Using ctxawslocal.WithContext, you can make requests for local mocks.
func ListObjects(
	ctx context.Context, region awsconfig.Region, bucketName BucketName, opts ...s3list.OptionS3List,
) (Objects, error) {
	s3Client, err := GetClient(ctx, region)
	if err != nil {
		return nil, err
	}
	return (&Client{raw: s3Client}).ListObjects(ctx, bucketName, opts...)
}

// GetObjectWriter on Client
// aws-sdk-go v2 GetObject output io.Writer
func (c *Client) GetObjectWriter(ctx context.Context, bucketName BucketName, key Key, w io.Writer) error {
	if _, err := c.HeadObject(ctx, bucketName, key); err != nil {
		return err
	}

	resp, err := c.raw.GetObject(ctx, &s3.GetObjectInput{
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

// GetObjectWriter
// aws-sdk-go v2 GetObject output io.Writer
//
// Mocks: Using ctxawslocal.WithContext, you can make requests for local mocks.
func GetObjectWriter(ctx context.Context, region awsconfig.Region, bucketName BucketName, key Key, w io.Writer) error {
	s3Client, err := GetClient(ctx, region)
	if err != nil {
		return err
	}
	return (&Client{raw: s3Client}).GetObjectWriter(ctx, bucketName, key, w)
}

// DeleteObject on Client
// aws-sdk-go v2 DeleteObject
func (c *Client) DeleteObject(ctx context.Context, bucketName BucketName, key Key) (
	*s3.DeleteObjectOutput, error,
) {
	if _, err := c.HeadObject(ctx, bucketName, key); err != nil {
		return nil, err
	}

	input := &s3.DeleteObjectInput{
		Bucket: bucketName.AWSString(),
		Key:    key.AWSString(),
	}
	return c.raw.DeleteObject(ctx, input)
}

// DeleteObject
// aws-sdk-go v2 DeleteObject
// Mocks: Using ctxawslocal.WithContext, you can make requests for local mocks.
func DeleteObject(ctx context.Context, region awsconfig.Region, bucketName BucketName, key Key) (
	*s3.DeleteObjectOutput, error,
) {
	s3Client, err := GetClient(ctx, region)
	if err != nil {
		return nil, err
	}
	return (&Client{raw: s3Client}).DeleteObject(ctx, bucketName, key)
}

// DownloadFiles on Client
// Batch download objects on s3 and save to directory
func (c *Client) DownloadFiles(
	ctx context.Context, bucketName BucketName, keys Keys, outputDir string,
	opts ...s3download.OptionS3Download,
) ([]string, error) {
	conf := s3download.GetS3DownloadConf(opts...)

	uniqKeys := keys.Unique()
	option := func(d *manager.Downloader) {
		d.BufferProvider = manager.NewPooledBufferedWriterReadFromProvider(5 * 1024 * 1024)
	}
	downloader := manager.NewDownloader(c.raw, option)
	paths := make([]string, len(uniqKeys))

	getFilePath := func(s3Key string) string {
		fileName := filepath.Base(s3Key)
		if conf.FileNameReplacer != nil {
			fileName = conf.FileNameReplacer(s3Key, fileName)
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

	for i := range uniqKeys {
		i := i
		s3Key := uniqKeys[i]
		filePath := getFilePath(s3Key.String())
		paths[i] = filePath
		f, err := os.Create(filePath)
		if err != nil {
			return nil, err
		}
		if _, err := downloader.Download(ctx, f, &s3.GetObjectInput{
			Bucket: bucketName.AWSString(),
			Key:    s3Key.AWSString(),
		}); err != nil {
			var oe *smithy.OperationError
			if errors.As(err, &oe) {
				var resErr *awshttp.ResponseError
				if errors.As(oe.Err, &resErr) {
					if resErr.Response.StatusCode == http.StatusNotFound {
						return nil, ErrNotFound
					}
				}
			}
			return nil, err
		}
	}
	return paths, nil
}

// DownloadFiles
// Batch download objects on s3 and save to directory
// If the file name is duplicated, add a sequential number to the suffix and save
//
// Mocks: Using ctxawslocal.WithContext, you can make requests for local mocks.
func DownloadFiles(
	ctx context.Context, region awsconfig.Region, bucketName BucketName, keys Keys, outputDir string,
	opts ...s3download.OptionS3Download,
) ([]string, error) {
	s3Client, err := GetClient(ctx, region)
	if err != nil {
		return nil, err
	}
	return (&Client{raw: s3Client}).DownloadFiles(ctx, bucketName, keys, outputDir, opts...)
}

// DownloadFilesParallel on Client
// Batch download objects on s3 and save to directory
func (c *Client) DownloadFilesParallel(
	ctx context.Context, bucketName BucketName, keys Keys, outputDir string,
	opts ...s3download.OptionS3Download,
) ([]string, error) {
	conf := s3download.GetS3DownloadConf(opts...)

	uniqKeys := keys.Unique()
	option := func(d *manager.Downloader) {
		d.BufferProvider = manager.NewPooledBufferedWriterReadFromProvider(5 * 1024 * 1024)
	}
	downloader := manager.NewDownloader(c.raw, option)
	paths := make([]string, len(uniqKeys))

	getFilePath := func(s3Key string) string {
		fileName := filepath.Base(s3Key)
		if conf.FileNameReplacer != nil {
			fileName = conf.FileNameReplacer(s3Key, fileName)
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
			ctx, cancel := context.WithCancel(ctx)
			b := backoff.WithContext(backoff.NewExponentialBackOff(), ctx)
			var gErr error
			if err := backoff.Retry(func() error {
				if _, err := downloader.Download(ctx, f, &s3.GetObjectInput{
					Bucket: bucketName.AWSString(),
					Key:    s3Key.AWSString(),
				}); err != nil {
					var oe *smithy.OperationError
					if errors.As(err, &oe) {
						var resErr *awshttp.ResponseError
						if errors.As(oe.Err, &resErr) {
							switch resErr.Response.StatusCode {
							case http.StatusNotFound:
								gErr = ErrNotFound
								cancel()
							case http.StatusBadRequest, http.StatusUnauthorized, http.StatusPaymentRequired,
								http.StatusForbidden, http.StatusMethodNotAllowed, http.StatusUnprocessableEntity,
								http.StatusMisdirectedRequest, http.StatusRequestURITooLong, http.StatusRequestEntityTooLarge,
								http.StatusPreconditionFailed:
								gErr = resErr.Err
								cancel()
							}
						}
					}
					return err
				}
				return nil
			}, b); err != nil {
				if gErr != nil {
					return gErr
				}
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

// DownloadFilesParallel
// Batch download objects on s3 and save to directory
// If the file name is duplicated, add a sequential number to the suffix and save
//
// Mocks: Using ctxawslocal.WithContext, you can make requests for local mocks.
func DownloadFilesParallel(
	ctx context.Context, region awsconfig.Region, bucketName BucketName, keys Keys, outputDir string,
	opts ...s3download.OptionS3Download,
) ([]string, error) {
	s3Client, err := GetClient(ctx, region)
	if err != nil {
		return nil, err
	}
	return (&Client{raw: s3Client}).DownloadFilesParallel(ctx, bucketName, keys, outputDir, opts...)
}

// Presign on Client
// aws-sdk-go v2 Presign
// default expires is 15 minutes
func (c *Client) Presign(
	ctx context.Context, bucketName BucketName, key Key,
	opts ...s3presigned.OptionS3Presigned,
) (string, error) {
	if _, err := c.HeadObject(ctx, bucketName, key); err != nil {
		return "", err
	}

	conf := s3presigned.GetS3PresignedConf(opts...)

	// @todo: not been able to test with and without option. create a separate function for input settings.
	input := &s3.GetObjectInput{
		Bucket: bucketName.AWSString(),
		Key:    key.AWSString(),
	}
	if conf.PresignFileName != "" {
		input.ResponseContentDisposition = aws.String(ResponseContentDisposition(conf.ContentDispositionType,
			conf.PresignFileName))
	}
	// Note: Fixed a bug in which the response is returned with `Content-Type:pdf` in case of PDF.
	// convert to Content-Type: application/pdf.
	if key.Ext() == ".pdf" {
		input.ResponseContentType = aws.String("application/pdf")
	}
	ps := s3.NewPresignClient(c.raw)
	resp, err := ps.PresignGetObject(ctx, input, func(o *s3.PresignOptions) {
		o.Expires = conf.PresignExpires
	})
	if err != nil {
		return "", err
	}
	return resp.URL, nil
}

// Presign
// aws-sdk-go v2 Presign
// default expires is 15 minutes
//
// Mocks: Using ctxawslocal.WithContext, you can make requests for local mocks.
func Presign(
	ctx context.Context, region awsconfig.Region, bucketName BucketName, key Key,
	opts ...s3presigned.OptionS3Presigned,
) (string, error) {
	s3Client, err := GetClient(ctx, region)
	if err != nil {
		return "", err
	}
	return (&Client{raw: s3Client}).Presign(ctx, bucketName, key, opts...)
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

// Copy on Client copies an Amazon S3 object from one bucket to same.
func (c *Client) Copy(
	ctx context.Context, bucketName BucketName, srcKey, destKey Key,
	opts ...s3upload.OptionS3Upload,
) error {
	conf := s3upload.GetS3UploadConf(opts...)
	req := &s3.CopyObjectInput{
		Bucket:            bucketName.AWSString(),
		CopySource:        srcKey.BucketJoinAWSString(bucketName),
		Key:               destKey.AWSString(),
		MetadataDirective: types.MetadataDirectiveReplace,
	}
	if conf.S3Expires != nil {
		req.Expires = aws.Time(time.Now().Add(*conf.S3Expires))
	}
	if _, err := c.raw.CopyObject(ctx, req); err != nil {
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

// Copy copies an Amazon S3 object from one bucket to same.
//
// Mocks: Using ctxawslocal.WithContext, you can make requests for local mocks.
func Copy(
	ctx context.Context, region awsconfig.Region, bucketName BucketName, srcKey, destKey Key,
	opts ...s3upload.OptionS3Upload,
) error {
	s3Client, err := GetClient(ctx, region)
	if err != nil {
		return err
	}
	return (&Client{raw: s3Client}).Copy(ctx, bucketName, srcKey, destKey, opts...)
}

const (
	SelectCSVAllQuery    = "SELECT * FROM S3Object"
	SelectCSVLimit1Query = "SELECT * FROM S3Object LIMIT 1"
)

// SelectCSVAll on Client
// SQL Reference : https://docs.aws.amazon.com/AmazonS3/latest/userguide/s3-glacier-select-sql-reference-select.html
func (c *Client) SelectCSVAll(
	ctx context.Context, bucketName BucketName, key Key, query string, w io.Writer,
	opts ...s3selectcsv.OptionS3SelectCSV,
) error {
	conf := s3selectcsv.GetS3SelectCSVConf(opts...)

	req := &s3.SelectObjectContentInput{
		Bucket:         bucketName.AWSString(),
		Key:            key.AWSString(),
		ExpressionType: types.ExpressionTypeSql,
		Expression:     aws.String(query),
		InputSerialization: &types.InputSerialization{
			CSV: &types.CSVInput{
				AllowQuotedRecordDelimiter: conf.CSVInput.AllowQuotedRecordDelimiter,
				Comments:                   conf.CSVInput.Comments,
				FieldDelimiter:             conf.CSVInput.FieldDelimiter,
				FileHeaderInfo:             conf.CSVInput.FileHeaderInfo,
				QuoteCharacter:             conf.CSVInput.QuoteCharacter,
				QuoteEscapeCharacter:       conf.CSVInput.QuoteEscapeCharacter,
				RecordDelimiter:            conf.CSVInput.RecordDelimiter,
			},
			CompressionType: conf.CompressionType,
		},
		OutputSerialization: &types.OutputSerialization{
			CSV: &types.CSVOutput{
				FieldDelimiter:       conf.CSVOutput.FieldDelimiter,
				QuoteCharacter:       conf.CSVOutput.QuoteCharacter,
				QuoteEscapeCharacter: conf.CSVOutput.QuoteEscapeCharacter,
				QuoteFields:          conf.CSVOutput.QuoteFields,
				RecordDelimiter:      conf.CSVOutput.RecordDelimiter,
			},
		},
	}
	if conf.SkipByteSize != nil {
		req.ScanRange = &types.ScanRange{Start: conf.SkipByteSize}
	}
	resp, err := c.raw.SelectObjectContent(ctx, req)
	if err != nil {
		if awsErr, ok := err.(*smithy.OperationError); ok {
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
			if _, err := t.Write(v.Value.Payload); err != nil {
				return err
			}
		case *types.SelectObjectContentEventStreamMemberStats:
		case *types.SelectObjectContentEventStreamMemberEnd:
		}
	}
	if err := resp.GetStream().Close(); err != nil {
		return err
	}
	return nil
}

// SelectCSVAll
// SQL Reference : https://docs.aws.amazon.com/AmazonS3/latest/userguide/s3-glacier-select-sql-reference-select.html
func SelectCSVAll(
	ctx context.Context, region awsconfig.Region, bucketName BucketName, key Key, query string, w io.Writer,
	opts ...s3selectcsv.OptionS3SelectCSV,
) error {
	s3Client, err := GetClient(ctx, region)
	if err != nil {
		return err
	}
	return (&Client{raw: s3Client}).SelectCSVAll(ctx, bucketName, key, query, w, opts...)
}

// SelectCSVHeaders on Client
// Get CSV headers
// Valid options: CompressionType
func (c *Client) SelectCSVHeaders(
	ctx context.Context, bucketName BucketName, key Key,
	opts ...s3selectcsv.OptionS3SelectCSV,
) ([]string, error) {
	conf := s3selectcsv.GetS3SelectCSVConf(opts...)
	opts = append(opts, s3selectcsv.WithCSVInput(types.CSVInput{
		AllowQuotedRecordDelimiter: conf.CSVInput.AllowQuotedRecordDelimiter,
		Comments:                   conf.CSVInput.Comments,
		FieldDelimiter:             conf.CSVInput.FieldDelimiter,
		FileHeaderInfo:             types.FileHeaderInfoNone,
		QuoteCharacter:             conf.CSVInput.QuoteCharacter,
		QuoteEscapeCharacter:       conf.CSVInput.QuoteEscapeCharacter,
		RecordDelimiter:            conf.CSVInput.RecordDelimiter,
	}))
	var buf bytes.Buffer
	if err := c.SelectCSVAll(ctx, bucketName, key, SelectCSVLimit1Query, &buf, opts...); err != nil {
		return nil, err
	}
	r := csv.NewReader(&buf)
	headers, err := r.Read()
	if err != nil {
		return nil, err
	}
	return headers, nil
}

// SelectCSVHeaders
// Get CSV headers
// Valid options: CompressionType
func SelectCSVHeaders(
	ctx context.Context, region awsconfig.Region, bucketName BucketName, key Key,
	opts ...s3selectcsv.OptionS3SelectCSV,
) ([]string, error) {
	s3Client, err := GetClient(ctx, region)
	if err != nil {
		return nil, err
	}
	return (&Client{raw: s3Client}).SelectCSVHeaders(ctx, bucketName, key, opts...)
}

// PresignPutObject on Client
func (c *Client) PresignPutObject(
	ctx context.Context, bucketName BucketName, key Key,
	opts ...s3presigned.OptionS3Presigned,
) (string, error) {
	conf := s3presigned.GetS3PresignedConf(opts...)
	input := &s3.PutObjectInput{
		Bucket: bucketName.AWSString(),
		Key:    key.AWSString(),
	}
	ps := s3.NewPresignClient(c.raw)
	resp, err := ps.PresignPutObject(ctx, input, func(o *s3.PresignOptions) {
		o.Expires = conf.PresignExpires
	})
	if err != nil {
		return "", err
	}
	return resp.URL, nil
}

func PresignPutObject(
	ctx context.Context, region awsconfig.Region, bucketName BucketName, key Key,
	opts ...s3presigned.OptionS3Presigned,
) (string, error) {
	s3Client, err := GetClient(ctx, region)
	if err != nil {
		return "", err
	}
	return (&Client{raw: s3Client}).PresignPutObject(ctx, bucketName, key, opts...)
}

// CreateMultipartUpload on Client
// ref: https://docs.aws.amazon.com/AmazonS3/latest/API/API_CreateMultipartUpload.html
func (c *Client) CreateMultipartUpload(
	ctx context.Context, bucketName BucketName, key Key, opts ...s3upload.OptionS3Upload,
) (string, error) {
	input := &s3.CreateMultipartUploadInput{
		Bucket: bucketName.AWSString(),
		Key:    key.AWSString(),
	}
	resp, err := c.raw.CreateMultipartUpload(ctx, input)
	if err != nil {
		return "", err
	}

	return *resp.UploadId, nil
}

// ref: https://docs.aws.amazon.com/AmazonS3/latest/API/API_CreateMultipartUpload.html
func CreateMultipartUpload(
	ctx context.Context, region awsconfig.Region, bucketName BucketName, key Key, opts ...s3upload.OptionS3Upload,
) (string, error) {
	s3Client, err := GetClient(ctx, region)
	if err != nil {
		return "", err
	}
	return (&Client{raw: s3Client}).CreateMultipartUpload(ctx, bucketName, key, opts...)
}

// UploadPart on Client
// ref: https://docs.aws.amazon.com/AmazonS3/latest/API/API_UploadPart.html
func (c *Client) UploadPart(
	ctx context.Context, bucketName BucketName, key Key, uploadID string, partNumber int32,
	body io.Reader,
) (*s3.UploadPartOutput, error) {
	input := &s3.UploadPartInput{
		Bucket:     bucketName.AWSString(),
		Key:        key.AWSString(),
		PartNumber: aws.Int32(partNumber),
		UploadId:   aws.String(uploadID),
		Body:       body,
	}
	resp, err := c.raw.UploadPart(ctx, input)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

// ref: https://docs.aws.amazon.com/AmazonS3/latest/API/API_UploadPart.html
func UploadPart(
	ctx context.Context, region awsconfig.Region, bucketName BucketName, key Key, uploadID string, partNumber int32,
	body io.Reader,
) (*s3.UploadPartOutput, error) {
	s3Client, err := GetClient(ctx, region)
	if err != nil {
		return nil, err
	}
	return (&Client{raw: s3Client}).UploadPart(ctx, bucketName, key, uploadID, partNumber, body)
}

// CompleteMultipartUpload on Client
// ref: https://docs.aws.amazon.com/AmazonS3/latest/API/API_CompleteMultipartUpload.html
func (c *Client) CompleteMultipartUpload(
	ctx context.Context, bucketName BucketName, key Key, uploadID string,
	completedParts []types.CompletedPart,
) (*s3.CompleteMultipartUploadOutput, error) {
	input := &s3.CompleteMultipartUploadInput{
		Bucket:   bucketName.AWSString(),
		Key:      key.AWSString(),
		UploadId: aws.String(uploadID),
		MultipartUpload: &types.CompletedMultipartUpload{
			Parts: completedParts,
		},
	}
	resp, err := c.raw.CompleteMultipartUpload(ctx, input)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

// ref: https://docs.aws.amazon.com/AmazonS3/latest/API/API_CompleteMultipartUpload.html
func CompleteMultipartUpload(
	ctx context.Context, region awsconfig.Region, bucketName BucketName, key Key, uploadID string,
	completedParts []types.CompletedPart,
) (*s3.CompleteMultipartUploadOutput, error) {
	s3Client, err := GetClient(ctx, region)
	if err != nil {
		return nil, err
	}
	return (&Client{raw: s3Client}).CompleteMultipartUpload(ctx, bucketName, key, uploadID, completedParts)
}

// AbortMultipartUpload on Client
// ref: https://docs.aws.amazon.com/AmazonS3/latest/API/API_AbortMultipartUpload.html
func (c *Client) AbortMultipartUpload(
	ctx context.Context, bucketName BucketName, key Key, uploadID string,
) error {
	input := &s3.AbortMultipartUploadInput{
		Bucket:   bucketName.AWSString(),
		Key:      key.AWSString(),
		UploadId: aws.String(uploadID),
	}
	_, err := c.raw.AbortMultipartUpload(ctx, input)
	if err != nil {
		return err
	}

	return nil
}

// ref: https://docs.aws.amazon.com/AmazonS3/latest/API/API_AbortMultipartUpload.html
func AbortMultipartUpload(
	ctx context.Context, region awsconfig.Region, bucketName BucketName, key Key, uploadID string,
) error {
	s3Client, err := GetClient(ctx, region)
	if err != nil {
		return err
	}
	return (&Client{raw: s3Client}).AbortMultipartUpload(ctx, bucketName, key, uploadID)
}
