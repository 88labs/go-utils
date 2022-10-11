package awss3

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/tomtwinkle/utfbomremover"
	"golang.org/x/text/transform"

	"github.com/88labs/go-utils/aws/awss3/options/s3selectcsv"

	"github.com/aws/smithy-go"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"

	"github.com/88labs/go-utils/aws/awsconfig"
	"github.com/88labs/go-utils/aws/awss3/options/s3donwload"
	"github.com/88labs/go-utils/aws/awss3/options/s3presigned"
	"github.com/88labs/go-utils/aws/awss3/options/s3upload"
)

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
func HeadObject(ctx context.Context, region awsconfig.Region, bucketName BucketName, key Key) (*s3.HeadObjectOutput, error) {
	client, err := GetClient(ctx, region) // nolint:typecheck
	if err != nil {
		return nil, err
	}
	return client.HeadObject(
		ctx,
		&s3.HeadObjectInput{
			Bucket: bucketName.AWSString(),
			Key:    key.AWSString(),
		})
}

// GetObjectWriter
// aws-sdk-go v2 GetObject output io.Writer
//
// Mocks: Using ctxawslocal.WithContext, you can make requests for local mocks.
func GetObjectWriter(ctx context.Context, region awsconfig.Region, bucketName BucketName, key Key, w io.Writer) error {
	client, err := GetClient(ctx, region) // nolint:typecheck
	if err != nil {
		return err
	}
	resp, err := client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: bucketName.AWSString(),
		Key:    key.AWSString(),
	})
	if err != nil {
		return err
	}
	if _, err := io.Copy(w, resp.Body); err != nil {
		return err
	}
	return nil
}

// DownloadFiles
// Batch download objects on s3 and save to directory
// If the file name is duplicated, add a sequential number to the suffix and save
//
// Mocks: Using ctxawslocal.WithContext, you can make requests for local mocks.
func DownloadFiles(ctx context.Context, region awsconfig.Region, bucketName BucketName, keys Keys, outputDir string, opts ...s3donwload.OptionS3Download) ([]string, error) {
	c := s3donwload.GetS3DownloadConf(opts...)

	client, err := GetClient(ctx, region) // nolint:typecheck
	if err != nil {
		return nil, err
	}

	uniqKeys := keys.Unique()
	downloader := manager.NewDownloader(client)
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

	for i := range uniqKeys {
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
			return nil, err
		}
	}
	return paths, nil
}

// Presign
// aws-sdk-go v2 Presign
// default expires is 15 minutes
//
// Mocks: Using ctxawslocal.WithContext, you can make requests for local mocks.
func Presign(ctx context.Context, region awsconfig.Region, bucketName BucketName, key Key, opts ...s3presigned.OptionS3Presigned) (string, error) {
	c := s3presigned.GetS3PresignedConf(opts...)
	client, err := GetClient(ctx, region) // nolint:typecheck
	if err != nil {
		return "", err
	}
	input := &s3.GetObjectInput{
		Bucket: bucketName.AWSString(),
		Key:    key.AWSString(),
	}
	if c.PresignFileName != "" {
		input.ResponseContentDisposition = aws.String(ResponseContentDisposition(c.PresignFileName))
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

// ResponseContentDisposition
// Setting ResponseContentDisposition to support file names with multibyte characters
func ResponseContentDisposition(fileName string) string {
	return fmt.Sprintf(`attachment; filename*=UTF-8''%s`, url.PathEscape(fileName))
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
				FileHeaderInfo: c.FileHeaderInfo,
			},
			CompressionType: c.CompressionType,
		},
		OutputSerialization: &types.OutputSerialization{
			CSV: &types.CSVOutput{},
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
	opts = append(opts, s3selectcsv.WithFileHeaderInfo(types.FileHeaderInfoNone))
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
