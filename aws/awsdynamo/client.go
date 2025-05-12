package awsdynamo

import (
	"context"
	"sync/atomic"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/retry"
	awsConfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"github.com/88labs/go-utils/aws/awsconfig"
	"github.com/88labs/go-utils/aws/ctxawslocal"
)

var dynamoDBClientAtomic atomic.Pointer[dynamodb.Client]

func GetClient(
	ctx context.Context, region awsconfig.Region, limitAttempts int, limitBackOffDelay time.Duration,
) (*dynamodb.Client, error) {
	if v := dynamoDBClientAtomic.Load(); v != nil {
		return v, nil
	}
	if localProfile, ok := getLocalEndpoint(ctx); ok {
		c, err := getClientLocal(ctx, *localProfile)
		if err != nil {
			return nil, err
		}
		dynamoDBClientAtomic.Store(c)
		return c, nil
	}
	// S3 Client
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
		return nil, err
	}
	c := dynamodb.NewFromConfig(awsCfg)
	dynamoDBClientAtomic.Store(c)
	return c, nil
}

func getClientLocal(ctx context.Context, localProfile LocalProfile) (*dynamodb.Client, error) {
	// https://aws.github.io/aws-sdk-go-v2/docs/configuring-sdk/endpoints/
	customResolver := aws.EndpointResolverWithOptionsFunc(func(
		service, region string, options ...interface{},
	) (aws.Endpoint, error) {
		if service == dynamodb.ServiceID {
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
		return nil, err
	}
	return dynamodb.NewFromConfig(awsCfg), nil
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
