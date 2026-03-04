package awss3

import (
	"context"
	"errors"
	"io"

	"github.com/aws/aws-sdk-go-v2/feature/s3/transfermanager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"

	"github.com/88labs/go-utils/aws/awsconfig"
	"github.com/88labs/go-utils/aws/awss3/options/s3download"
	"github.com/88labs/go-utils/aws/awss3/options/s3head"
	"github.com/88labs/go-utils/aws/awss3/options/s3list"
	"github.com/88labs/go-utils/aws/awss3/options/s3presigned"
	"github.com/88labs/go-utils/aws/awss3/options/s3selectcsv"
	"github.com/88labs/go-utils/aws/awss3/options/s3upload"
)

var ErrNotFound = errors.New("NotFound")

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
	c, err := GetClient(ctx, region) // nolint:typecheck
	if err != nil {
		return nil, err
	}
	return (&Client{client: c}).PutObject(ctx, bucketName, key, body, opts...)
}

// UploadManager
// Upload using the manager.Uploader of aws-sdk-go v2
//
// Mocks: Using ctxawslocal.WithContext, you can make requests for local mocks.
func UploadManager(
	ctx context.Context, region awsconfig.Region, bucketName BucketName, key Key, body io.Reader,
	opts ...s3upload.OptionS3Upload,
) (*transfermanager.UploadObjectOutput, error) {
	c, err := GetClient(ctx, region) // nolint:typecheck
	if err != nil {
		return nil, err
	}
	return (&Client{client: c}).UploadManager(ctx, bucketName, key, body, opts...)
}

// HeadObject
// aws-sdk-go v2 HeadObject
//
// Mocks: Using ctxawslocal.WithContext, you can make requests for local mocks.
func HeadObject(
	ctx context.Context, region awsconfig.Region, bucketName BucketName, key Key, opts ...s3head.OptionS3Head,
) (*s3.HeadObjectOutput, error) {
	c, err := GetClient(ctx, region) // nolint:typecheck
	if err != nil {
		return nil, err
	}
	return (&Client{client: c}).HeadObject(ctx, bucketName, key, opts...)
}

// ListObjects
// aws-sdk-go v2 ListObjectsV2
//
// Mocks: Using ctxawslocal.WithContext, you can make requests for local mocks.
func ListObjects(
	ctx context.Context, region awsconfig.Region, bucketName BucketName, opts ...s3list.OptionS3List,
) (Objects, error) {
	c, err := GetClient(ctx, region) // nolint:typecheck
	if err != nil {
		return nil, err
	}
	return (&Client{client: c}).ListObjects(ctx, bucketName, opts...)
}

// GetObjectWriter
// aws-sdk-go v2 GetObject output io.Writer
//
// Mocks: Using ctxawslocal.WithContext, you can make requests for local mocks.
func GetObjectWriter(ctx context.Context, region awsconfig.Region, bucketName BucketName, key Key, w io.Writer) error {
	c, err := GetClient(ctx, region) // nolint:typecheck
	if err != nil {
		return err
	}
	return (&Client{client: c}).GetObjectWriter(ctx, bucketName, key, w)
}

// DeleteObject
// aws-sdk-go v2 DeleteObject
// Mocks: Using ctxawslocal.WithContext, you can make requests for local mocks.
func DeleteObject(ctx context.Context, region awsconfig.Region, bucketName BucketName, key Key) (
	*s3.DeleteObjectOutput, error,
) {
	c, err := GetClient(ctx, region) // nolint:typecheck
	if err != nil {
		return nil, err
	}
	return (&Client{client: c}).DeleteObject(ctx, bucketName, key)
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
	c, err := GetClient(ctx, region) // nolint:typecheck
	if err != nil {
		return nil, err
	}
	return (&Client{client: c}).DownloadFiles(ctx, bucketName, keys, outputDir, opts...)
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
	c, err := GetClient(ctx, region) // nolint:typecheck
	if err != nil {
		return nil, err
	}
	return (&Client{client: c}).DownloadFilesParallel(ctx, bucketName, keys, outputDir, opts...)
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
	c, err := GetClient(ctx, region) // nolint:typecheck
	if err != nil {
		return "", err
	}
	return (&Client{client: c}).Presign(ctx, bucketName, key, opts...)
}

// Copy copies an Amazon S3 object from one bucket to same.
//
// Mocks: Using ctxawslocal.WithContext, you can make requests for local mocks.
func Copy(
	ctx context.Context, region awsconfig.Region, bucketName BucketName, srcKey, destKey Key,
	opts ...s3upload.OptionS3Upload,
) error {
	c, err := GetClient(ctx, region) // nolint:typecheck
	if err != nil {
		return err
	}
	return (&Client{client: c}).Copy(ctx, bucketName, srcKey, destKey, opts...)
}

const (
	SelectCSVAllQuery    = "SELECT * FROM S3Object"
	SelectCSVLimit1Query = "SELECT * FROM S3Object LIMIT 1"
)

// SelectCSVAll
// SQL Reference : https://docs.aws.amazon.com/AmazonS3/latest/userguide/s3-glacier-select-sql-reference-select.html
func SelectCSVAll(
	ctx context.Context, region awsconfig.Region, bucketName BucketName, key Key, query string, w io.Writer,
	opts ...s3selectcsv.OptionS3SelectCSV,
) error {
	c, err := GetClient(ctx, region) // nolint:typecheck
	if err != nil {
		return err
	}
	return (&Client{client: c}).SelectCSVAll(ctx, bucketName, key, query, w, opts...)
}

// SelectCSVHeaders
// Get CSV headers
// Valid options: CompressionType
func SelectCSVHeaders(
	ctx context.Context, region awsconfig.Region, bucketName BucketName, key Key,
	opts ...s3selectcsv.OptionS3SelectCSV,
) ([]string, error) {
	c, err := GetClient(ctx, region) // nolint:typecheck
	if err != nil {
		return nil, err
	}
	return (&Client{client: c}).SelectCSVHeaders(ctx, bucketName, key, opts...)
}

func PresignPutObject(
	ctx context.Context, region awsconfig.Region, bucketName BucketName, key Key,
	opts ...s3presigned.OptionS3Presigned,
) (string, error) {
	c, err := GetClient(ctx, region) // nolint:typecheck
	if err != nil {
		return "", err
	}
	return (&Client{client: c}).PresignPutObject(ctx, bucketName, key, opts...)
}

// ref: https://docs.aws.amazon.com/AmazonS3/latest/API/API_CreateMultipartUpload.html
func CreateMultipartUpload(
	ctx context.Context, region awsconfig.Region, bucketName BucketName, key Key, opts ...s3upload.OptionS3Upload,
) (string, error) {
	c, err := GetClient(ctx, region) // nolint:typecheck
	if err != nil {
		return "", err
	}
	_ = opts // opts not used by the SDK call; kept for API compatibility
	return (&Client{client: c}).CreateMultipartUpload(ctx, bucketName, key)
}

// ref: https://docs.aws.amazon.com/AmazonS3/latest/API/API_UploadPart.html
func UploadPart(
	ctx context.Context, region awsconfig.Region, bucketName BucketName, key Key, uploadID string, partNumber int32,
	body io.Reader,
) (*s3.UploadPartOutput, error) {
	c, err := GetClient(ctx, region) // nolint:typecheck
	if err != nil {
		return nil, err
	}
	return (&Client{client: c}).UploadPart(ctx, bucketName, key, uploadID, partNumber, body)
}

// ref: https://docs.aws.amazon.com/AmazonS3/latest/API/API_CompleteMultipartUpload.html
func CompleteMultipartUpload(
	ctx context.Context, region awsconfig.Region, bucketName BucketName, key Key, uploadID string,
	completedParts []types.CompletedPart,
) (*s3.CompleteMultipartUploadOutput, error) {
	c, err := GetClient(ctx, region) // nolint:typecheck
	if err != nil {
		return nil, err
	}
	return (&Client{client: c}).CompleteMultipartUpload(ctx, bucketName, key, uploadID, completedParts)
}

// ref: https://docs.aws.amazon.com/AmazonS3/latest/API/API_AbortMultipartUpload.html
func AbortMultipartUpload(
	ctx context.Context, region awsconfig.Region, bucketName BucketName, key Key, uploadID string,
) error {
	c, err := GetClient(ctx, region) // nolint:typecheck
	if err != nil {
		return err
	}
	return (&Client{client: c}).AbortMultipartUpload(ctx, bucketName, key, uploadID)
}
