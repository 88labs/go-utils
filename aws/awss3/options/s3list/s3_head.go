package s3list

type OptionS3List interface {
	Apply(*confS3List)
}

type confS3List struct {
	// https://github.com/aws/aws-sdk-go-v2/blob/v1.30.4/service/s3/api_op_ListObjectsV2.go#L201-L205
	// Limits the response to keys that begin with the specified prefix.
	//
	// Directory buckets - For directory buckets, only prefixes that end in a
	// delimiter ( / ) are supported.
	Prefix *string
}

type OptionPrefix string

func (o OptionPrefix) Apply(c *confS3List) {
	v := string(o)
	c.Prefix = &v
}

// WithPrefix
// https://github.com/aws/aws-sdk-go-v2/blob/v1.30.4/service/s3/api_op_ListObjectsV2.go#L201-L205
// Limits the response to keys that begin with the specified prefix.
//
// Directory buckets - For directory buckets, only prefixes that end in a
// delimiter ( / ) are supported.
func WithPrefix(prefix string) OptionPrefix {
	return OptionPrefix(prefix)
}

// nolint:revive
func GetS3ListConf(opts ...OptionS3List) confS3List {
	c := confS3List{}
	for _, opt := range opts {
		opt.Apply(&c)
	}
	return c
}
