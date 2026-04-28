package awss3

import (
	"bytes"
	"context"
	"encoding/gob"
	"fmt"
	"log/slog"
	"net"
	"sync/atomic"

	"github.com/aws/aws-sdk-go-v2/aws"
	awshttp "github.com/aws/aws-sdk-go-v2/aws/transport/http"
	awsConfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	"github.com/88labs/go-utils/aws/awsconfig"
	"github.com/88labs/go-utils/aws/awss3/options/global/s3dialer"
	"github.com/88labs/go-utils/aws/ctxawslocal"
)

var (
	// GlobalDialer Global http dialer settings for awss3 library
	GlobalDialer *s3dialer.ConfGlobalDialer
	// GlobalLogger is used by package-level helpers such as PutObject and HeadObject.
	// It is nil by default, which disables logging.
	GlobalLogger   *slog.Logger
	s3ClientAtomic atomic.Pointer[s3.Client]
)

// Client is an S3 client that manages its own SDK client instance.
// Unlike the package-level functions that use a singleton, each Client holds
// its own *s3.Client, enabling external lifecycle management.
type Client struct {
	client *s3.Client
	logger *slog.Logger
}

// NewClient creates a new Client for the given region.
// Using ctxawslocal.WithContext, you can make requests for local mocks.
func NewClient(ctx context.Context, region awsconfig.Region, opts ...ClientOption) (*Client, error) {
	cfg := defaultClientConfig()
	for _, opt := range opts {
		opt.apply(&cfg)
	}
	sdkClient, err := newS3Client(ctx, region)
	if err != nil {
		return nil, err
	}
	return &Client{client: sdkClient, logger: cfg.logger}, nil
}

// S3Client returns the underlying *s3.Client for advanced usage.
func (c *Client) S3Client() *s3.Client {
	return c.client
}

// GetClient
// Get s3 client for aws-sdk-go v2.
// Using ctxawslocal.WithContext, you can make requests for local mocks
func GetClient(ctx context.Context, region awsconfig.Region) (*s3.Client, error) {
	if v := s3ClientAtomic.Load(); v != nil {
		return v, nil
	}
	sdkClient, err := newS3Client(ctx, region)
	if err != nil {
		return nil, err
	}
	s3ClientAtomic.Store(sdkClient)
	return sdkClient, nil
}

// newS3Client creates a fresh *s3.Client without touching the singleton.
func newS3Client(ctx context.Context, region awsconfig.Region) (*s3.Client, error) {
	if localProfile, ok := getLocalEndpoint(ctx); ok {
		return getClientLocal(ctx, *localProfile)
	}
	awsHttpClient := awshttp.NewBuildableClient()
	if GlobalDialer != nil {
		awsHttpClient.WithDialerOptions(func(dialer *net.Dialer) {
			if GlobalDialer.Timeout != 0 {
				dialer.Timeout = GlobalDialer.Timeout
			}
			if GlobalDialer.Deadline != nil {
				dialer.Deadline = *GlobalDialer.Deadline
			}
			if GlobalDialer.KeepAlive != 0 {
				dialer.KeepAlive = GlobalDialer.KeepAlive
			}
		})
	}
	// S3 Client
	awsCfg, err := awsConfig.LoadDefaultConfig(
		ctx,
		awsConfig.WithRegion(region.String()),
		awsConfig.WithHTTPClient(awsHttpClient),
	)
	if err != nil {
		return nil, fmt.Errorf("unable to load SDK config, %w", err)
	}
	return s3.NewFromConfig(awsCfg), nil
}

func getClientLocal(ctx context.Context, localProfile LocalProfile) (*s3.Client, error) {
	awsHttpClient := awshttp.NewBuildableClient()
	if GlobalDialer != nil {
		awsHttpClient.WithDialerOptions(func(dialer *net.Dialer) {
			if GlobalDialer.Timeout != 0 {
				dialer.Timeout = GlobalDialer.Timeout
			}
			if GlobalDialer.Deadline != nil {
				dialer.Deadline = *GlobalDialer.Deadline
			}
			if GlobalDialer.KeepAlive != 0 {
				dialer.KeepAlive = GlobalDialer.KeepAlive
			}
		})
	}
	awsCfg, err := awsConfig.LoadDefaultConfig(ctx,
		awsConfig.WithHTTPClient(awsHttpClient),
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
	return s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(localProfile.Endpoint)
		o.UsePathStyle = true
	}), nil
}

func packageClientFromSDK(sdkClient *s3.Client) *Client {
	return &Client{
		client: sdkClient,
		logger: GlobalLogger,
	}
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
		p.Endpoint = c.S3Endpoint
		p.AccessKey = c.AccessKey
		p.SecretAccessKey = c.SecretAccessKey
		p.SessionToken = c.SessionToken
		return p, true
	}
	return nil, false
}

func hash(v LocalProfile) ([]byte, error) {
	var b bytes.Buffer
	if err := gob.NewEncoder(&b).Encode(v); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}
