package awssqs

import (
	"context"
	"fmt"
	"sync/atomic"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsConfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/sqs"

	"github.com/88labs/go-utils/aws/awsconfig"
	"github.com/88labs/go-utils/aws/ctxawslocal"
)

var sqsClientAtomic atomic.Pointer[sqs.Client]

// GetClient
// Get s3 client for aws-sdk-go v2.
// Using ctxawslocal.WithContext, you can make requests for local mocks
func GetClient(ctx context.Context, region awsconfig.Region) (*sqs.Client, error) {
	if v := sqsClientAtomic.Load(); v != nil {
		return v, nil
	}
	if localProfile, ok := getLocalEndpoint(ctx); ok {
		c, err := getClientLocal(ctx, *localProfile)
		if err != nil {
			return nil, err
		}
		sqsClientAtomic.Store(c)
		return c, nil
	}
	// SQS Client
	awsCfg, err := awsConfig.LoadDefaultConfig(ctx, awsConfig.WithRegion(region.String()))
	if err != nil {
		return nil, fmt.Errorf("unable to load SDK config, %w", err)
	}
	c := sqs.NewFromConfig(awsCfg)
	sqsClientAtomic.Store(c)
	return c, nil
}

func getClientLocal(ctx context.Context, localProfile LocalProfile) (*sqs.Client, error) {
	// https://aws.github.io/aws-sdk-go-v2/docs/configuring-sdk/endpoints/
	customResolver := aws.EndpointResolverWithOptionsFunc(func(
		service, region string, options ...interface{},
	) (aws.Endpoint, error) {
		if service == sqs.ServiceID {
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
				SessionToken:    localProfile.SessionToken,
			},
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("unable to load SDK config, %w", err)
	}
	return sqs.NewFromConfig(awsCfg), nil
}

type LocalProfile struct {
	Endpoint        string
	AccessKey       string
	SecretAccessKey string
	SessionToken    string
}

func getLocalEndpoint(ctx context.Context) (*LocalProfile, bool) {
	if c, ok := ctxawslocal.GetConf(ctx); ok {
		p := new(LocalProfile)
		p.Endpoint = c.SQSEndpoint
		p.AccessKey = c.AccessKey
		p.SecretAccessKey = c.SecretAccessKey
		p.SessionToken = c.SessionToken
		return p, true
	}
	return nil, false
}
