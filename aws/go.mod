module github.com/88labs/go-utils/aws

go 1.23
toolchain go1.24.1

require (
	github.com/88labs/go-utils/ulid v0.8.0
	github.com/88labs/go-utils/utf8bom v0.5.0
	github.com/aws/aws-sdk-go-v2 v1.36.3
	github.com/aws/aws-sdk-go-v2/config v1.29.9
	github.com/aws/aws-sdk-go-v2/credentials v1.17.62
	github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue v1.18.8
	github.com/aws/aws-sdk-go-v2/feature/dynamodb/expression v1.7.74
	github.com/aws/aws-sdk-go-v2/feature/s3/manager v1.17.66
	github.com/aws/aws-sdk-go-v2/service/cognitoidentity v1.29.3
	github.com/aws/aws-sdk-go-v2/service/dynamodb v1.42.0
	github.com/aws/aws-sdk-go-v2/service/s3 v1.78.2
	github.com/aws/aws-sdk-go-v2/service/sqs v1.38.1
	github.com/aws/smithy-go v1.22.3
	github.com/cenkalti/backoff/v4 v4.3.0
	github.com/go-faker/faker/v4 v4.6.0
	github.com/stretchr/testify v1.10.0
	github.com/tomtwinkle/utfbomremover v0.1.1
	golang.org/x/sync v0.12.0
	golang.org/x/text v0.23.0
)

require (
	github.com/aws/aws-sdk-go-v2/aws/protocol/eventstream v1.6.10 // indirect
	github.com/aws/aws-sdk-go-v2/feature/ec2/imds v1.16.30 // indirect
	github.com/aws/aws-sdk-go-v2/internal/configsources v1.3.34 // indirect
	github.com/aws/aws-sdk-go-v2/internal/endpoints/v2 v2.6.34 // indirect
	github.com/aws/aws-sdk-go-v2/internal/ini v1.8.3 // indirect
	github.com/aws/aws-sdk-go-v2/internal/v4a v1.3.34 // indirect
	github.com/aws/aws-sdk-go-v2/service/dynamodbstreams v1.25.1 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/accept-encoding v1.12.3 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/checksum v1.7.0 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/endpoint-discovery v1.10.15 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/presigned-url v1.12.15 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/s3shared v1.18.15 // indirect
	github.com/aws/aws-sdk-go-v2/service/sso v1.25.1 // indirect
	github.com/aws/aws-sdk-go-v2/service/ssooidc v1.29.1 // indirect
	github.com/aws/aws-sdk-go-v2/service/sts v1.33.17 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/kr/pretty v0.3.0 // indirect
	github.com/oklog/ulid/v2 v2.1.0 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/rogpeppe/go-internal v1.8.0 // indirect
	gopkg.in/check.v1 v1.0.0-20201130134442-10cb98267c6c // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)
