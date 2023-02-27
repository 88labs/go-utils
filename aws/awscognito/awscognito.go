package awscognito

import (
	"context"
	"fmt"

	"github.com/88labs/go-utils/aws/ctxawslocal"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsConfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cognitoidentity"

	"github.com/88labs/go-utils/aws/awsconfig"
)

// GetCredentialsForIdentity
// aws-sdk-go v2 GetCredentialsForIdentity
//
// Mocks: Using ctxawslocal.WithContext, you can make requests for local mocks.
func GetCredentialsForIdentity(ctx context.Context, region awsconfig.Region, identityId string, logins map[string]string) (*cognitoidentity.GetCredentialsForIdentityOutput, error) {
	localProfile, _ := getLocalEndpoint(ctx)
	// Cognito Client
	awsCfg, err := awsConfig.LoadDefaultConfig(ctx, awsConfig.WithRegion(region.String()))
	if err != nil {
		return nil, fmt.Errorf("unable to load SDK config, %w", err)
	}
	client := cognitoidentity.NewFromConfig(awsCfg)
	if err != nil {
		return nil, err
	}

	res, err := client.GetCredentialsForIdentity(
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
