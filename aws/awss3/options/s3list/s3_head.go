package s3list

type OptionS3List interface {
	Apply(*confS3List)
}

type confS3List struct {
	// Limits the response to keys that begin with the specified prefix.
	Prefix *string
}

type OptionPrefix string

func (o OptionPrefix) Apply(c *confS3List) {
	v := string(o)
	c.Prefix = &v
}

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
