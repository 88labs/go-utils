package awscognito

import (
	"context"
	"fmt"
	"sync/atomic"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsConfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/cognitoidentity"

	"github.com/88labs/go-utils/aws/awsconfig"
	"github.com/88labs/go-utils/aws/ctxawslocal"
)

var cognitoidentityClientAtomic atomic.Pointer[cognitoidentity.Client]

// Client is a Cognito Identity client that manages its own SDK client instance.
// Unlike the package-level functions that use a singleton, each Client holds
// its own *cognitoidentity.Client, enabling external lifecycle management.
type Client struct {
	client *cognitoidentity.Client
}

// NewClient creates a new Client for the given region.
// Using ctxawslocal.WithContext, you can redirect requests to a local mock
// (e.g. LocalStack) by setting the Cognito endpoint and credentials in the context.
func NewClient(ctx context.Context, region awsconfig.Region) (*Client, error) {
	sdkClient, err := newCognitoClient(ctx, region)
	if err != nil {
		return nil, err
	}
	return &Client{client: sdkClient}, nil
}

// CognitoClient returns the underlying *cognitoidentity.Client for advanced usage.
func (c *Client) CognitoClient() *cognitoidentity.Client {
	return c.client
}

// GetCredentialsForIdentity calls the Cognito GetCredentialsForIdentity API.
// The region and endpoint are those the Client was created with via NewClient.
func (c *Client) GetCredentialsForIdentity(
	ctx context.Context, identityId string, logins map[string]string,
) (*cognitoidentity.GetCredentialsForIdentityOutput, error) {
	res, err := c.client.GetCredentialsForIdentity(
		ctx,
		&cognitoidentity.GetCredentialsForIdentityInput{
			IdentityId: aws.String(identityId),
			Logins:     logins,
		},
	)
	if err != nil {
		return nil, err
	}
	return res, nil
}

// GetCredentialsForIdentity calls the Cognito GetCredentialsForIdentity API
// using the package-level singleton client.
//
// Mocks: Using ctxawslocal.WithContext, you can make requests for local mocks.
func GetCredentialsForIdentity(
	ctx context.Context, region awsconfig.Region, identityId string, logins map[string]string,
) (*cognitoidentity.GetCredentialsForIdentityOutput, error) {
	var sdkClient *cognitoidentity.Client
	if v := cognitoidentityClientAtomic.Load(); v != nil {
		sdkClient = v
	} else {
		var err error
		sdkClient, err = newCognitoClient(ctx, region)
		if err != nil {
			return nil, err
		}
		cognitoidentityClientAtomic.Store(sdkClient)
	}
	return (&Client{client: sdkClient}).GetCredentialsForIdentity(ctx, identityId, logins)
}

// newCognitoClient creates a fresh *cognitoidentity.Client without touching the singleton.
func newCognitoClient(ctx context.Context, region awsconfig.Region) (*cognitoidentity.Client, error) {
	if localProfile, ok := getLocalEndpoint(ctx); ok {
		return getClientLocal(ctx, region, *localProfile)
	}
	awsCfg, err := awsConfig.LoadDefaultConfig(ctx, awsConfig.WithRegion(region.String()))
	if err != nil {
		return nil, fmt.Errorf("unable to load SDK config, %w", err)
	}
	return cognitoidentity.NewFromConfig(awsCfg), nil
}

func getClientLocal(ctx context.Context, region awsconfig.Region, localProfile LocalProfile) (
	*cognitoidentity.Client, error,
) {
	awsCfg, err := awsConfig.LoadDefaultConfig(ctx,
		awsConfig.WithRegion(region.String()),
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
	return cognitoidentity.NewFromConfig(awsCfg, func(o *cognitoidentity.Options) {
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
		p.Endpoint = c.CognitoEndpoint
		p.AccessKey = c.AccessKey
		p.SecretAccessKey = c.SecretAccessKey
		p.SessionToken = c.SessionToken
		return p, true
	}
	return nil, false
}
