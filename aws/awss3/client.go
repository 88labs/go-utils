package awss3

import (
	"context"
	"fmt"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsConfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	"github.com/88labs/go-utils/aws/awsconfig"
	"github.com/88labs/go-utils/aws/ctxawslocal"
)

var s3Client *s3.Client
var once sync.Once

func GetClient(ctx context.Context, region awsconfig.Region) (*s3.Client, error) {
	if localProfile, ok := getLocalEndpoint(ctx); ok {
		return getClientLocal(ctx, *localProfile)
	}
	var responseError error
	once.Do(func() {
		// S3 Client
		awsCfg, err := awsConfig.LoadDefaultConfig(ctx, awsConfig.WithRegion(region.Value()))
		if err != nil {
			responseError = fmt.Errorf("unable to load SDK config, %w", err)
		} else {
			responseError = nil
		}
		s3Client = s3.NewFromConfig(awsCfg)
	})
	return s3Client, responseError
}

func getClientLocal(ctx context.Context, localProfile LocalProfile) (*s3.Client, error) {
	var responseError error
	once.Do(func() {
		// https://aws.github.io/aws-sdk-go-v2/docs/configuring-sdk/endpoints/
		customResolver := aws.EndpointResolverWithOptionsFunc(func(service, region string, options ...interface{}) (aws.Endpoint, error) {
			if service == s3.ServiceID {
				return aws.Endpoint{
					PartitionID:       "aws",
					URL:               localProfile.Endpoint,
					SigningRegion:     region,
					HostnameImmutable: true,
				}, nil
			}
			// returning EndpointNotFoundError will allow the service to fallback to it's default resolution
			return aws.Endpoint{}, &aws.EndpointNotFoundError{}
		})
		awsCfg, err := awsConfig.LoadDefaultConfig(ctx,
			awsConfig.WithEndpointResolverWithOptions(customResolver),
			awsConfig.WithCredentialsProvider(credentials.StaticCredentialsProvider{
				Value: aws.Credentials{
					AccessKeyID:     localProfile.AccessKey,
					SecretAccessKey: localProfile.SecretAccessKey,
				},
			}),
		)
		if err != nil {
			responseError = fmt.Errorf("unable to load SDK config, %w", err)
			return
		} else {
			responseError = nil
		}
		s3Client = s3.NewFromConfig(awsCfg)
	})
	return s3Client, responseError
}

type LocalProfile struct {
	Endpoint        string
	AccessKey       string
	SecretAccessKey string
}

func getLocalEndpoint(ctx context.Context) (*LocalProfile, bool) {
	if c, ok := ctxawslocal.GetConf(ctx); ok {
		p := new(LocalProfile)
		p.Endpoint = fmt.Sprintf("%s:%d", c.Endpoint, c.S3Port)
		p.AccessKey = c.AccessKey
		p.SecretAccessKey = c.SecretAccessKey
		return p, true
	}
	return nil, false
}
