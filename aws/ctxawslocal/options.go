package ctxawslocal

type OptionMock interface {
	Apply(*ConfMock)
}

type ConfMock struct {
	AccessKey       string
	SecretAccessKey string
	SessionToken    string
	S3Endpoint      string
	SQSEndpoint     string
	CognitoEndpoint string
	DynamoEndpoint  string
}

// nolint:revive
func getConf(opts ...OptionMock) ConfMock {
	// default options
	c := ConfMock{
		AccessKey:       "test", // localstack default AccessKey
		SecretAccessKey: "test", // localstack default SecretAccessKey
		SessionToken:    "",
		S3Endpoint:      "http://127.0.0.1:4566", // localstack default endpoint
		SQSEndpoint:     "http://127.0.0.1:4566", // localstack default endpoint
		DynamoEndpoint:  "http://127.0.0.1:4566", // localstack default endpoint
	}
	for _, opt := range opts {
		opt.Apply(&c)
	}
	return c
}

type OptionAccessKey string

func (o OptionAccessKey) Apply(c *ConfMock) {
	c.AccessKey = string(o)
}

func WithAccessKey(accessKey string) OptionAccessKey {
	return OptionAccessKey(accessKey)
}

type OptionSecretAccessKey string

func (o OptionSecretAccessKey) Apply(c *ConfMock) {
	c.SecretAccessKey = string(o)
}

func WithSecretAccessKey(secretAccessKey string) OptionSecretAccessKey {
	return OptionSecretAccessKey(secretAccessKey)
}

type OptionSessionToken string

func (o OptionSessionToken) Apply(c *ConfMock) {
	c.SessionToken = string(o)
}

func WithSessionToken(sessionToken string) OptionSessionToken {
	return OptionSessionToken(sessionToken)
}

type OptionS3Endpoint string

func (o OptionS3Endpoint) Apply(c *ConfMock) {
	c.S3Endpoint = string(o)
}

func WithS3Endpoint(endpoint string) OptionS3Endpoint {
	return OptionS3Endpoint(endpoint)
}

type OptionSQSEndpoint string

func (o OptionSQSEndpoint) Apply(c *ConfMock) {
	c.SQSEndpoint = string(o)
}

func WithSQSEndpoint(endpoint string) OptionSQSEndpoint {
	return OptionSQSEndpoint(endpoint)
}

type OptionCognitoEndpoint string

func (o OptionCognitoEndpoint) Apply(c *ConfMock) {
	c.CognitoEndpoint = string(o)
}

func WithCognitoEndpoint(endpoint string) OptionCognitoEndpoint {
	return OptionCognitoEndpoint(endpoint)
}

type OptionDynamoEndpoint string

func (o OptionDynamoEndpoint) Apply(c *ConfMock) {
	c.DynamoEndpoint = string(o)
}

func WithDynamoEndpoint(endpoint string) OptionDynamoEndpoint {
	return OptionDynamoEndpoint(endpoint)
}
