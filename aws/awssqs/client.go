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
	awsCfg, err := awsConfig.LoadDefaultConfig(ctx,
		awsConfig.WithCredentialsProvider(credentials.StaticCredentialsProvider{
			Value: aws.Credentials{
				AccessKeyID:     localProfile.AccessKey,
				SecretAccessKey: localProfile.SecretAccessKey,
				SessionToken:    localProfile.SessionToken,
			},
		}),
		awsConfig.WithDefaultRegion(awsconfig.RegionTokyo.String()),
	)
	if err != nil {
		return nil, fmt.Errorf("unable to load SDK config, %w", err)
	}
	return sqs.NewFromConfig(awsCfg, func(o *sqs.Options) {
		o.BaseEndpoint = aws.String(localProfile.Endpoint)
	}), nil
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
