package awssqs

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsConfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/sqs"

	"github.com/88labs/go-utils/aws/awsconfig"
	"github.com/88labs/go-utils/aws/ctxawslocal"
	"github.com/88labs/go-utils/aws/internal/awstrace"
)

var (
	sqsClientAtomic atomic.Pointer[sqs.Client]
	sqsClientConfig atomic.Pointer[clientConfig]
	sqsClientInitMu sync.Mutex
)

// Client is an SQS client that manages its own SDK client instance.
// Unlike the package-level functions that use a singleton, each Client holds
// its own *sqs.Client, enabling external lifecycle management.
type Client struct {
	client *sqs.Client
	config clientConfig
}

// NewClient creates a new Client for the given region.
// Using ctxawslocal.WithContext, you can make requests for local mocks.
func NewClient(ctx context.Context, region awsconfig.Region, opts ...ClientOption) (*Client, error) {
	cfg := clientConfig{}
	for _, opt := range opts {
		if opt != nil {
			opt.apply(&cfg)
		}
	}
	sdkClient, err := newSQSClient(ctx, region, cfg)
	if err != nil {
		return nil, err
	}
	return &Client{client: sdkClient, config: cfg}, nil
}

// SQSClient returns the underlying *sqs.Client for advanced usage.
func (c *Client) SQSClient() *sqs.Client {
	return c.client
}

// GetClient returns the package-level singleton SQS client for aws-sdk-go v2.
// Using ctxawslocal.WithContext, you can make requests for local mocks.
// Options are used only when the singleton is initialized.
func GetClient(ctx context.Context, region awsconfig.Region, opts ...ClientOption) (*sqs.Client, error) {
	if v := sqsClientAtomic.Load(); v != nil {
		return v, nil
	}
	sqsClientInitMu.Lock()
	defer sqsClientInitMu.Unlock()
	if v := sqsClientAtomic.Load(); v != nil {
		return v, nil
	}
	cfg := clientConfig{}
	for _, opt := range opts {
		if opt != nil {
			opt.apply(&cfg)
		}
	}
	sdkClient, err := newSQSClient(ctx, region, cfg)
	if err != nil {
		return nil, err
	}
	sqsClientConfig.Store(&cfg)
	sqsClientAtomic.Store(sdkClient)
	return sdkClient, nil
}

func packageClientFromSDK(sdkClient *sqs.Client) *Client {
	cfg := clientConfig{}
	if stored := sqsClientConfig.Load(); stored != nil {
		cfg = *stored
	}
	return &Client{client: sdkClient, config: cfg}
}

// newSQSClient creates a fresh *sqs.Client without touching the singleton.
func newSQSClient(ctx context.Context, region awsconfig.Region, cfg clientConfig) (*sqs.Client, error) {
	if localProfile, ok := getLocalEndpoint(ctx); ok {
		return getClientLocal(ctx, *localProfile, cfg)
	}
	// SQS Client
	awsCfg, err := awsConfig.LoadDefaultConfig(ctx, awsConfig.WithRegion(region.String()))
	if err != nil {
		return nil, fmt.Errorf("unable to load SDK config, %w", err)
	}
	return sqs.NewFromConfig(awsCfg, func(o *sqs.Options) {
		if cfg.traceEnabled {
			awstrace.AppendMiddlewares(&o.APIOptions, cfg.traceProvider)
		}
	}), nil
}

func getClientLocal(ctx context.Context, localProfile LocalProfile, cfg clientConfig) (*sqs.Client, error) {
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
		if cfg.traceEnabled {
			awstrace.AppendMiddlewares(&o.APIOptions, cfg.traceProvider)
		}
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
