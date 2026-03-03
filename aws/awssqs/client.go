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

// Client is an SQS client that manages its own SDK client instance.
// Unlike the package-level functions that use a singleton, each Client holds
// its own *sqs.Client, enabling external lifecycle management.
type Client struct {
	client *sqs.Client
}

// NewClient creates a new Client for the given region.
// Using ctxawslocal.WithContext, you can make requests for local mocks.
func NewClient(ctx context.Context, region awsconfig.Region) (*Client, error) {
	sdkClient, err := newSQSClient(ctx, region)
	if err != nil {
		return nil, err
	}
	return &Client{client: sdkClient}, nil
}

// SQSClient returns the underlying *sqs.Client for advanced usage.
func (c *Client) SQSClient() *sqs.Client {
	return c.client
}

// GetClient returns the package-level singleton SQS client for aws-sdk-go v2.
// Using ctxawslocal.WithContext, you can make requests for local mocks.
func GetClient(ctx context.Context, region awsconfig.Region) (*sqs.Client, error) {
	if v := sqsClientAtomic.Load(); v != nil {
		return v, nil
	}
	sdkClient, err := newSQSClient(ctx, region)
	if err != nil {
		return nil, err
	}
	sqsClientAtomic.Store(sdkClient)
	return sdkClient, nil
}

// newSQSClient creates a fresh *sqs.Client without touching the singleton.
func newSQSClient(ctx context.Context, region awsconfig.Region) (*sqs.Client, error) {
	if localProfile, ok := getLocalEndpoint(ctx); ok {
		return getClientLocal(ctx, *localProfile)
	}
	// SQS Client
	awsCfg, err := awsConfig.LoadDefaultConfig(ctx, awsConfig.WithRegion(region.String()))
	if err != nil {
		return nil, fmt.Errorf("unable to load SDK config, %w", err)
	}
	return sqs.NewFromConfig(awsCfg), nil
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
