package awss3

import (
	"bytes"
	"context"
	"encoding/gob"
	"fmt"
	"net"

	"github.com/88labs/go-utils/aws/awsconfig"
	"github.com/88labs/go-utils/aws/awss3/options/global/s3dialer"
	"github.com/88labs/go-utils/aws/ctxawslocal"
	"github.com/aws/aws-sdk-go-v2/aws"
	awshttp "github.com/aws/aws-sdk-go-v2/aws/transport/http"
	awsConfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

var (
	// GlobalDialer Global http dialer settings for awss3 library
	GlobalDialer *s3dialer.ConfGlobalDialer
)

// GetClient
// Get s3 client for aws-sdk-go v2.
// Using ctxawslocal.WithContext, you can make requests for local mocks
func GetClient(ctx context.Context, region awsconfig.Region) (*s3.Client, error) {
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
	return s3.NewFromConfig(awsCfg), nil
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
