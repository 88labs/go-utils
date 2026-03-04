package awsdynamo

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/retry"
	awsConfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"github.com/88labs/go-utils/aws/awsconfig"
	"github.com/88labs/go-utils/aws/awsdynamo/dynamooptions"
	"github.com/88labs/go-utils/aws/ctxawslocal"
)

var dynamoDBClientAtomic atomic.Pointer[dynamodb.Client]

// Client is a DynamoDB client that manages its own SDK client instance.
// Unlike the package-level functions that use a singleton, each Client holds
// its own *dynamodb.Client, enabling external lifecycle management.
//
// Note: Generic package-level functions (PutItem, GetItem, etc.) remain at the
// package level because Go does not allow type parameters on methods.
type Client struct {
	client *dynamodb.Client
}

// NewClient creates a new Client for the given region.
// Using ctxawslocal.WithContext, you can make requests for local mocks.
func NewClient(ctx context.Context, region awsconfig.Region, opts ...dynamooptions.OptionDynamo) (*Client, error) {
	c := dynamooptions.GetDynamoConf(opts...)
	sdkClient, err := newDynamoDBClient(ctx, region, c.MaxAttempts, c.MaxBackoffDelay)
	if err != nil {
		return nil, err
	}
	return &Client{client: sdkClient}, nil
}

// DynamoDBClient returns the underlying *dynamodb.Client for advanced usage.
func (c *Client) DynamoDBClient() *dynamodb.Client {
	return c.client
}

func GetClient(
	ctx context.Context, region awsconfig.Region, limitAttempts int, limitBackOffDelay time.Duration,
) (*dynamodb.Client, error) {
	if v := dynamoDBClientAtomic.Load(); v != nil {
		return v, nil
	}
	sdkClient, err := newDynamoDBClient(ctx, region, limitAttempts, limitBackOffDelay)
	if err != nil {
		return nil, err
	}
	dynamoDBClientAtomic.Store(sdkClient)
	return sdkClient, nil
}

// newDynamoDBClient creates a fresh *dynamodb.Client without touching the singleton.
func newDynamoDBClient(
	ctx context.Context, region awsconfig.Region, limitAttempts int, limitBackOffDelay time.Duration,
) (*dynamodb.Client, error) {
	if localProfile, ok := getLocalEndpoint(ctx); ok {
		return getClientLocal(ctx, *localProfile)
	}
	awsCfg, err := awsConfig.LoadDefaultConfig(ctx, awsConfig.WithRegion(region.String()),
		awsConfig.WithRetryer(func() aws.Retryer {
			r := retry.AddWithMaxAttempts(retry.NewStandard(), limitAttempts)
			r = retry.AddWithMaxBackoffDelay(r, limitBackOffDelay)
			r = retry.AddWithErrorCodes(r,
				string(types.BatchStatementErrorCodeEnumItemCollectionSizeLimitExceeded),
				string(types.BatchStatementErrorCodeEnumRequestLimitExceeded),
				string(types.BatchStatementErrorCodeEnumProvisionedThroughputExceeded),
				string(types.BatchStatementErrorCodeEnumInternalServerError),
				string(types.BatchStatementErrorCodeEnumThrottlingError),
			)
			return r
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("unable to load SDK config, %w", err)
	}
	return dynamodb.NewFromConfig(awsCfg), nil
}

func getClientLocal(ctx context.Context, localProfile LocalProfile) (*dynamodb.Client, error) {
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
	return dynamodb.NewFromConfig(awsCfg, func(o *dynamodb.Options) {
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
		p.Endpoint = c.DynamoEndpoint
		p.AccessKey = c.AccessKey
		p.SecretAccessKey = c.SecretAccessKey
		p.SessionToken = c.SessionToken
		return p, true
	}
	return nil, false
}
