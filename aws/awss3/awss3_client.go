package awss3

import (
	"bytes"
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/s3/transfermanager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"
	awshttp "github.com/aws/smithy-go/transport/http"
	"github.com/cenkalti/backoff/v4"
	"github.com/tomtwinkle/utfbomremover"
	"golang.org/x/sync/errgroup"
	"golang.org/x/text/transform"

	"github.com/88labs/go-utils/aws/awss3/options/s3download"
	"github.com/88labs/go-utils/aws/awss3/options/s3head"
	"github.com/88labs/go-utils/aws/awss3/options/s3list"
	"github.com/88labs/go-utils/aws/awss3/options/s3presigned"
	"github.com/88labs/go-utils/aws/awss3/options/s3selectcsv"
	"github.com/88labs/go-utils/aws/awss3/options/s3upload"
)

// PutObject uploads an object to Amazon S3.
// If there is no particular reason to use PutObject, please use UploadManager.
//
// Notes
// https://aws.github.io/aws-sdk-go-v2/docs/sdk-utilities/s3/#unseekable-streaming-input
// Amazon S3 requires the content length to be provided for all object's uploaded to a bucket.
// Since the Body input parameter does not implement io.Seeker interface the client will not be able to compute the ContentLength parameter for the request.
// The parameter must be provided by the application. The request will fail if the ContentLength parameter is not provided.
func (c *Client) PutObject(
	ctx context.Context, bucketName BucketName, key Key, body io.Reader,
	opts ...s3upload.OptionS3Upload,
) (res *s3.PutObjectOutput, err error) {
	done := c.logOperation(ctx, "PutObject",
		slog.String("bucket", bucketName.String()),
		slog.String("key", key.String()),
	)
	defer func() {
		done(err)
	}()

	conf := s3upload.GetS3UploadConf(opts...)
	input := &s3.PutObjectInput{
		Body:   body,
		Bucket: bucketName.AWSString(),
		Key:    key.AWSString(),
	}
	if conf.S3Expires != nil {
		input.Expires = aws.Time(time.Now().Add(*conf.S3Expires))
	}
	res, err = c.client.PutObject(ctx, input)
	return res, err
}

// UploadManager uploads an object using the transfer manager.
func (c *Client) UploadManager(
	ctx context.Context, bucketName BucketName, key Key, body io.Reader,
	opts ...s3upload.OptionS3Upload,
) (res *transfermanager.UploadObjectOutput, err error) {
	done := c.logOperation(ctx, "UploadManager",
		slog.String("bucket", bucketName.String()),
		slog.String("key", key.String()),
	)
	defer func() {
		done(err)
	}()

	conf := s3upload.GetS3UploadConf(opts...)
	uploader := transfermanager.New(c.client)
	input := &transfermanager.UploadObjectInput{
		Body:   body,
		Bucket: bucketName.AWSString(),
		Key:    key.AWSString(),
	}
	if conf.S3Expires != nil {
		input.Expires = aws.Time(time.Now().Add(*conf.S3Expires))
	}
	res, err = uploader.UploadObject(ctx, input)
	return res, err
}

// HeadObject retrieves metadata from an object without returning the object itself.
func (c *Client) HeadObject(
	ctx context.Context, bucketName BucketName, key Key, opts ...s3head.OptionS3Head,
) (res *s3.HeadObjectOutput, err error) {
	done := c.logOperation(ctx, "HeadObject",
		slog.String("bucket", bucketName.String()),
		slog.String("key", key.String()),
	)
	defer func() {
		done(err)
	}()

	res, err = c.headObject(ctx, bucketName, key, opts...)
	return res, err
}

func (c *Client) headObject(
	ctx context.Context, bucketName BucketName, key Key, opts ...s3head.OptionS3Head,
) (*s3.HeadObjectOutput, error) {
	conf := s3head.GetS3HeadConf(opts...)
	if conf.Timeout > 0 {
		waiter := s3.NewObjectExistsWaiter(c.client, func(options *s3.ObjectExistsWaiterOptions) {
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
	res, err := c.client.HeadObject(
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

// ListObjects lists objects in a bucket.
func (c *Client) ListObjects(
	ctx context.Context, bucketName BucketName, opts ...s3list.OptionS3List,
) (objects Objects, err error) {
	conf := s3list.GetS3ListConf(opts...)
	attrs := []slog.Attr{
		slog.String("bucket", bucketName.String()),
	}
	if conf.Prefix != nil {
		attrs = append(attrs, slog.String("prefix", *conf.Prefix))
	}
	done := c.logOperation(ctx, "ListObjects", attrs...)
	defer func() {
		done(err, slog.Int("object_count", len(objects)))
	}()

	input := &s3.ListObjectsV2Input{
		Bucket: bucketName.AWSString(),
	}
	if conf.Prefix != nil {
		input.Prefix = conf.Prefix
	}
	objects = make(Objects, 0)
	paginator := s3.NewListObjectsV2Paginator(c.client, input)
	for paginator.HasMorePages() {
		output, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		objects = append(objects, output.Contents...)
	}
	return objects, nil
}

// GetObjectWriter downloads an object and writes its content to w.
func (c *Client) GetObjectWriter(ctx context.Context, bucketName BucketName, key Key, w io.Writer) (err error) {
	done := c.logOperation(ctx, "GetObjectWriter",
		slog.String("bucket", bucketName.String()),
		slog.String("key", key.String()),
	)
	defer func() {
		done(err)
	}()

	if _, err = c.headObject(ctx, bucketName, key); err != nil {
		return err
	}
	resp, err := c.client.GetObject(ctx, &s3.GetObjectInput{
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
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
	}()
	_, err = io.Copy(w, resp.Body)
	return err
}

// DeleteObject deletes an object from a bucket.
func (c *Client) DeleteObject(ctx context.Context, bucketName BucketName, key Key) (
	res *s3.DeleteObjectOutput, err error,
) {
	done := c.logOperation(ctx, "DeleteObject",
		slog.String("bucket", bucketName.String()),
		slog.String("key", key.String()),
	)
	defer func() {
		done(err)
	}()

	if _, err = c.headObject(ctx, bucketName, key); err != nil {
		return nil, err
	}
	input := &s3.DeleteObjectInput{
		Bucket: bucketName.AWSString(),
		Key:    key.AWSString(),
	}
	res, err = c.client.DeleteObject(ctx, input)
	return res, err
}

// DownloadFiles downloads multiple objects and saves them to a directory.
// If the file name is duplicated, a sequential number is added to the suffix.
func (c *Client) DownloadFiles(
	ctx context.Context, bucketName BucketName, keys Keys, outputDir string,
	opts ...s3download.OptionS3Download,
) (paths []string, err error) {
	done := c.logOperation(ctx, "DownloadFiles",
		slog.String("bucket", bucketName.String()),
		slog.Int("key_count", len(keys)),
		slog.String("output_dir", outputDir),
	)
	downloadedCount := 0
	defer func() {
		done(err, slog.Int("downloaded_file_count", downloadedCount))
	}()

	conf := s3download.GetS3DownloadConf(opts...)
	uniqKeys := keys.Unique()
	downloader := transfermanager.New(c.client, func(o *transfermanager.Options) {
		o.GetObjectBufferSize = 5 * 1024 * 1024
	})
	paths = make([]string, len(uniqKeys))

	getFilePath := func(s3Key string) string {
		fileName := filepath.Base(s3Key)
		if conf.FileNameReplacer != nil {
			fileName = conf.FileNameReplacer(s3Key, fileName)
		}
		filePath := path.Join(outputDir, fileName)
		var existsFileCount int
		for {
			if existsFileCount > 0 {
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
		_, dlErr := downloader.DownloadObject(ctx, &transfermanager.DownloadObjectInput{
			Bucket:   bucketName.AWSString(),
			Key:      s3Key.AWSString(),
			WriterAt: f,
		})
		if closeErr := f.Close(); closeErr != nil && dlErr == nil {
			dlErr = closeErr
		}
		if dlErr != nil {
			_ = os.Remove(filePath)
			var oe *smithy.OperationError
			if errors.As(dlErr, &oe) {
				var resErr *awshttp.ResponseError
				if errors.As(oe.Err, &resErr) {
					if resErr.Response.StatusCode == http.StatusNotFound {
						return nil, ErrNotFound
					}
				}
			}
			return nil, dlErr
		}
		downloadedCount++
	}
	return paths, nil
}

// DownloadFilesParallel downloads multiple objects in parallel and saves them to a directory.
// If the file name is duplicated, a sequential number is added to the suffix.
func (c *Client) DownloadFilesParallel(
	ctx context.Context, bucketName BucketName, keys Keys, outputDir string,
	opts ...s3download.OptionS3Download,
) (paths []string, err error) {
	done := c.logOperation(ctx, "DownloadFilesParallel",
		slog.String("bucket", bucketName.String()),
		slog.Int("key_count", len(keys)),
		slog.String("output_dir", outputDir),
	)
	downloadedCount := 0
	defer func() {
		done(err, slog.Int("downloaded_file_count", downloadedCount))
	}()

	conf := s3download.GetS3DownloadConf(opts...)
	uniqKeys := keys.Unique()
	downloader := transfermanager.New(c.client, func(o *transfermanager.Options) {
		o.GetObjectBufferSize = 5 * 1024 * 1024
	})
	paths = make([]string, len(uniqKeys))

	resolveFilePath := func(s3Key string) string {
		fileName := filepath.Base(s3Key)
		if conf.FileNameReplacer != nil {
			fileName = conf.FileNameReplacer(s3Key, fileName)
		}
		filePath := path.Join(outputDir, fileName)
		var existsFileCount int
		for {
			if existsFileCount > 0 {
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

	files := make([]*os.File, len(uniqKeys))
	for i, s3Key := range uniqKeys {
		filePath := resolveFilePath(s3Key.String())
		paths[i] = filePath
		f, err := os.Create(filePath)
		if err != nil {
			for j := 0; j < i; j++ {
				_ = files[j].Close()
				_ = os.Remove(paths[j])
			}
			return nil, err
		}
		files[i] = f
	}

	var eg errgroup.Group
	egCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	for i := range uniqKeys {
		i := i
		s3Key := uniqKeys[i]
		f := files[i]
		filePath := paths[i]
		eg.Go(func() error {
			var gErr error
			b := backoff.WithContext(backoff.NewExponentialBackOff(), egCtx)
			dlErr := backoff.Retry(func() error {
				if _, err := downloader.DownloadObject(egCtx, &transfermanager.DownloadObjectInput{
					Bucket:   bucketName.AWSString(),
					Key:      s3Key.AWSString(),
					WriterAt: f,
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
			}, b)
			if closeErr := f.Close(); closeErr != nil && dlErr == nil {
				dlErr = closeErr
			}
			if dlErr != nil {
				_ = os.Remove(filePath)
				if gErr != nil {
					return gErr
				}
				return dlErr
			}
			return nil
		})
	}
	if err := eg.Wait(); err != nil {
		return nil, err
	}
	downloadedCount = len(paths)
	return paths, nil
}

// Presign generates a pre-signed URL for an object.
// Default expires is 15 minutes.
func (c *Client) Presign(
	ctx context.Context, bucketName BucketName, key Key,
	opts ...s3presigned.OptionS3Presigned,
) (url string, err error) {
	conf := s3presigned.GetS3PresignedConf(opts...)
	done := c.logOperation(ctx, "Presign",
		slog.String("bucket", bucketName.String()),
		slog.String("key", key.String()),
	)
	defer func() {
		done(err, slog.Duration("expires", conf.PresignExpires))
	}()

	if _, err = c.headObject(ctx, bucketName, key); err != nil {
		return "", err
	}
	input := &s3.GetObjectInput{
		Bucket: bucketName.AWSString(),
		Key:    key.AWSString(),
	}
	if conf.PresignFileName != "" {
		input.ResponseContentDisposition = aws.String(ResponseContentDisposition(conf.ContentDispositionType,
			conf.PresignFileName))
	}
	if key.Ext() == ".pdf" {
		input.ResponseContentType = aws.String("application/pdf")
	}
	ps := s3.NewPresignClient(c.client)
	resp, err := ps.PresignGetObject(ctx, input, func(o *s3.PresignOptions) {
		o.Expires = conf.PresignExpires
	})
	if err != nil {
		return "", err
	}
	url = resp.URL
	return url, nil
}

// Copy copies an Amazon S3 object within the same bucket.
func (c *Client) Copy(
	ctx context.Context, bucketName BucketName, srcKey, destKey Key,
	opts ...s3upload.OptionS3Upload,
) (err error) {
	done := c.logOperation(ctx, "Copy",
		slog.String("bucket", bucketName.String()),
		slog.String("src_key", srcKey.String()),
		slog.String("dest_key", destKey.String()),
	)
	defer func() {
		done(err)
	}()

	conf := s3upload.GetS3UploadConf(opts...)
	req := &s3.CopyObjectInput{
		Bucket:            bucketName.AWSString(),
		CopySource:        srcKey.bucketJoinEscapedAWSString(bucketName),
		Key:               destKey.AWSString(),
		MetadataDirective: types.MetadataDirectiveReplace,
	}
	if conf.S3Expires != nil {
		req.Expires = aws.Time(time.Now().Add(*conf.S3Expires))
	}
	if _, err := c.client.CopyObject(ctx, req); err != nil {
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

// SelectCSVAll executes a SQL expression against S3 Select and writes results to w.
// SQL Reference: https://docs.aws.amazon.com/AmazonS3/latest/userguide/s3-glacier-select-sql-reference-select.html
func (c *Client) SelectCSVAll(
	ctx context.Context, bucketName BucketName, key Key, query string, w io.Writer,
	opts ...s3selectcsv.OptionS3SelectCSV,
) (err error) {
	done := c.logOperation(ctx, "SelectCSVAll",
		slog.String("bucket", bucketName.String()),
		slog.String("key", key.String()),
	)
	defer func() {
		done(err)
	}()

	return c.selectCSVAll(ctx, bucketName, key, query, w, opts...)
}

func (c *Client) selectCSVAll(
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
	resp, err := c.client.SelectObjectContent(ctx, req)
	if err != nil {
		var apiErr smithy.APIError
		if errors.As(err, &apiErr) && apiErr.ErrorCode() == "InvalidRange" {
			return nil
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

// SelectCSVHeaders retrieves the CSV header row.
// Valid options: CompressionType
func (c *Client) SelectCSVHeaders(
	ctx context.Context, bucketName BucketName, key Key,
	opts ...s3selectcsv.OptionS3SelectCSV,
) (headers []string, err error) {
	done := c.logOperation(ctx, "SelectCSVHeaders",
		slog.String("bucket", bucketName.String()),
		slog.String("key", key.String()),
	)
	defer func() {
		done(err, slog.Int("header_count", len(headers)))
	}()

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
	if err = c.selectCSVAll(ctx, bucketName, key, SelectCSVLimit1Query, &buf, opts...); err != nil {
		return nil, err
	}
	r := csv.NewReader(&buf)
	headers, err = r.Read()
	if err != nil {
		return nil, err
	}
	return headers, nil
}

// PresignPutObject generates a pre-signed URL for uploading an object.
func (c *Client) PresignPutObject(
	ctx context.Context, bucketName BucketName, key Key,
	opts ...s3presigned.OptionS3Presigned,
) (url string, err error) {
	conf := s3presigned.GetS3PresignedConf(opts...)
	done := c.logOperation(ctx, "PresignPutObject",
		slog.String("bucket", bucketName.String()),
		slog.String("key", key.String()),
	)
	defer func() {
		done(err, slog.Duration("expires", conf.PresignExpires))
	}()

	input := &s3.PutObjectInput{
		Bucket: bucketName.AWSString(),
		Key:    key.AWSString(),
	}
	ps := s3.NewPresignClient(c.client)
	resp, err := ps.PresignPutObject(ctx, input, func(o *s3.PresignOptions) {
		o.Expires = conf.PresignExpires
	})
	if err != nil {
		return "", err
	}
	url = resp.URL
	return url, nil
}

// CreateMultipartUpload initiates a multipart upload.
// ref: https://docs.aws.amazon.com/AmazonS3/latest/API/API_CreateMultipartUpload.html
func (c *Client) CreateMultipartUpload(
	ctx context.Context, bucketName BucketName, key Key,
) (uploadID string, err error) {
	done := c.logOperation(ctx, "CreateMultipartUpload",
		slog.String("bucket", bucketName.String()),
		slog.String("key", key.String()),
	)
	defer func() {
		done(err)
	}()

	input := &s3.CreateMultipartUploadInput{
		Bucket: bucketName.AWSString(),
		Key:    key.AWSString(),
	}
	resp, err := c.client.CreateMultipartUpload(ctx, input)
	if err != nil {
		return "", err
	}
	uploadID = *resp.UploadId
	return uploadID, nil
}

// UploadPart uploads a part in a multipart upload.
// ref: https://docs.aws.amazon.com/AmazonS3/latest/API/API_UploadPart.html
func (c *Client) UploadPart(
	ctx context.Context, bucketName BucketName, key Key, uploadID string, partNumber int32,
	body io.Reader,
) (res *s3.UploadPartOutput, err error) {
	done := c.logOperation(ctx, "UploadPart",
		slog.String("bucket", bucketName.String()),
		slog.String("key", key.String()),
		slog.Int64("part_number", int64(partNumber)),
	)
	defer func() {
		done(err)
	}()

	input := &s3.UploadPartInput{
		Bucket:     bucketName.AWSString(),
		Key:        key.AWSString(),
		PartNumber: aws.Int32(partNumber),
		UploadId:   aws.String(uploadID),
		Body:       body,
	}
	resp, err := c.client.UploadPart(ctx, input)
	if err != nil {
		return nil, err
	}
	res = resp
	return res, nil
}

// CompleteMultipartUpload completes a multipart upload.
// ref: https://docs.aws.amazon.com/AmazonS3/latest/API/API_CompleteMultipartUpload.html
func (c *Client) CompleteMultipartUpload(
	ctx context.Context, bucketName BucketName, key Key, uploadID string,
	completedParts []types.CompletedPart,
) (res *s3.CompleteMultipartUploadOutput, err error) {
	done := c.logOperation(ctx, "CompleteMultipartUpload",
		slog.String("bucket", bucketName.String()),
		slog.String("key", key.String()),
		slog.Int("part_count", len(completedParts)),
	)
	defer func() {
		done(err)
	}()

	input := &s3.CompleteMultipartUploadInput{
		Bucket:   bucketName.AWSString(),
		Key:      key.AWSString(),
		UploadId: aws.String(uploadID),
		MultipartUpload: &types.CompletedMultipartUpload{
			Parts: completedParts,
		},
	}
	resp, err := c.client.CompleteMultipartUpload(ctx, input)
	if err != nil {
		return nil, err
	}
	res = resp
	return res, nil
}

// AbortMultipartUpload aborts a multipart upload.
// ref: https://docs.aws.amazon.com/AmazonS3/latest/API/API_AbortMultipartUpload.html
func (c *Client) AbortMultipartUpload(
	ctx context.Context, bucketName BucketName, key Key, uploadID string,
) (err error) {
	done := c.logOperation(ctx, "AbortMultipartUpload",
		slog.String("bucket", bucketName.String()),
		slog.String("key", key.String()),
	)
	defer func() {
		done(err)
	}()

	input := &s3.AbortMultipartUploadInput{
		Bucket:   bucketName.AWSString(),
		Key:      key.AWSString(),
		UploadId: aws.String(uploadID),
	}
	_, err = c.client.AbortMultipartUpload(ctx, input)
	return err
}
