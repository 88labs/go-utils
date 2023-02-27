package awscognito

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cognitoidentity"

	"github.com/88labs/go-utils/aws/awsconfig"
)

// GetCredentialsForIdentity
// aws-sdk-go v2 GetCredentialsForIdentity
//
// Mocks: Using ctxawslocal.WithContext, you can make requests for local mocks.
func GetCredentialsForIdentity(ctx context.Context, region awsconfig.Region, identityId string, logins map[string]string) (*cognitoidentity.GetCredentialsForIdentityOutput, error) {
	client, err := GetClient(ctx, region) // nolint:typecheck
	if err != nil {
		return nil, err
	}

	res, err := client.GetCredentialsForIdentity(
		ctx,
		&cognitoidentity.GetCredentialsForIdentityInput{
			IdentityId:    aws.String(identityId),
			CustomRoleArn: nil,
			Logins:        logins,
		})
	if err != nil {
		return nil, err
	}
	return res, nil
}
