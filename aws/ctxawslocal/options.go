package ctxawslocal

type OptionMock interface {
	Apply(*ConfMock)
}

type ConfMock struct {
	AccessKey       string
	SecretAccessKey string
	S3Endpoint      string
}

// nolint:revive
func getConf(opts ...OptionMock) ConfMock {
	// default options
	c := ConfMock{
		AccessKey:       "test",                  // localstack default AccessKey
		SecretAccessKey: "test",                  // localstack default SecretAccessKey
		S3Endpoint:      "http://127.0.0.1:4566", // localstack default endpoint
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

type OptionS3Endpoint string

func (o OptionS3Endpoint) Apply(c *ConfMock) {
	c.S3Endpoint = string(o)
}

func WithS3Endpoint(endpoint string) OptionS3Endpoint {
	return OptionS3Endpoint(endpoint)
}
