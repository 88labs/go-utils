package ctxawslocal

type OptionMock interface {
	Apply(*ConfMock)
}

type ConfMock struct {
	AccessKey       string
	SecretAccessKey string
	Endpoint        string
	S3Port          int
}

// nolint:revive
func getConf(opts ...OptionMock) ConfMock {
	// default options
	c := ConfMock{
		AccessKey:       "test", // localstack default AccessKey
		SecretAccessKey: "test", // localstack default SecretAccessKey
		Endpoint:        "http://127.0.0.1",
		S3Port:          4566, // localstack default port
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

type OptionEndpoint string

func (o OptionEndpoint) Apply(c *ConfMock) {
	c.Endpoint = string(o)
}

func WithEndpoint(endpoint string) OptionEndpoint {
	return OptionEndpoint(endpoint)
}

type OptionS3Port int

func (o OptionS3Port) Apply(c *ConfMock) {
	c.S3Port = int(o)
}

func WithS3Port(port int) OptionS3Port {
	return OptionS3Port(port)
}
