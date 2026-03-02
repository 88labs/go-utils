package awscognito

import (
	"context"
	"fmt"
	"sync/atomic"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsConfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cognitoidentity"

	"github.com/88labs/go-utils/aws/awsconfig"
	"github.com/88labs/go-utils/aws/ctxawslocal"
)

var cognitoidentityClientAtomic atomic.Pointer[cognitoidentity.Client]

// Client wraps a *cognitoidentity.Client.
type Client struct {
	raw *cognitoidentity.Client
}

// NewClient creates a new, non-cached Cognito Identity client.
// Using ctxawslocal.WithContext, you can make requests for local mocks.
func NewClient(ctx context.Context, region awsconfig.Region) (*Client, error) {
	awsCfg, err := awsConfig.LoadDefaultConfig(ctx, awsConfig.WithRegion(region.String()))
	if err != nil {
		return nil, fmt.Errorf("unable to load SDK config, %w", err)
	}
	return &Client{raw: cognitoidentity.NewFromConfig(awsCfg)}, nil
}

// CognitoClient returns the underlying *cognitoidentity.Client.
func (c *Client) CognitoClient() *cognitoidentity.Client {
	return c.raw
}

// GetCredentialsForIdentity on Client
// aws-sdk-go v2 GetCredentialsForIdentity
//
// Mocks: Using ctxawslocal.WithContext, you can make requests for local mocks.
func (c *Client) GetCredentialsForIdentity(
	ctx context.Context, region awsconfig.Region, identityId string, logins map[string]string,
) (*cognitoidentity.GetCredentialsForIdentityOutput, error) {
	localProfile, _ := getLocalEndpoint(ctx)
	res, err := c.raw.GetCredentialsForIdentity(
		ctx,
		&cognitoidentity.GetCredentialsForIdentityInput{
			IdentityId:    aws.String(identityId),
			CustomRoleArn: nil,
			Logins:        logins,
		}, func(options *cognitoidentity.Options) {
			options.Region = region.String()
			if localProfile != nil {
				options.Credentials = aws.CredentialsProviderFunc(func(ctx context.Context) (aws.Credentials, error) {
					return aws.Credentials{
						AccessKeyID:     localProfile.AccessKey,
						SecretAccessKey: localProfile.SecretAccessKey,
						SessionToken:    localProfile.SessionToken,
					}, nil
				})
			}
		})
	if err != nil {
		return nil, err
	}
	return res, nil
}

// GetCredentialsForIdentity
// aws-sdk-go v2 GetCredentialsForIdentity
//
// Mocks: Using ctxawslocal.WithContext, you can make requests for local mocks.
func GetCredentialsForIdentity(
	ctx context.Context, region awsconfig.Region, identityId string, logins map[string]string,
) (*cognitoidentity.GetCredentialsForIdentityOutput, error) {
	var client *cognitoidentity.Client
	if v := cognitoidentityClientAtomic.Load(); v != nil {
		client = v
	} else {
		// Cognito Client
		awsCfg, err := awsConfig.LoadDefaultConfig(ctx, awsConfig.WithRegion(region.String()))
		if err != nil {
			return nil, fmt.Errorf("unable to load SDK config, %w", err)
		}
		client = cognitoidentity.NewFromConfig(awsCfg)
		cognitoidentityClientAtomic.Store(client)
	}
	return (&Client{raw: client}).GetCredentialsForIdentity(ctx, region, identityId, logins)
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
