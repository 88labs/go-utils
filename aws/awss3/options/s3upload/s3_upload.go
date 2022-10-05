package s3upload

import (
	"time"
)

type OptionS3Upload interface {
	Apply(*confS3Upload)
}

type confS3Upload struct {
	S3Expires time.Duration
}

// nolint:revive
func GetS3UploadConf(opts ...OptionS3Upload) confS3Upload {
	// default options
	c := confS3Upload{
		S3Expires: 24 * time.Hour,
	}
	for _, opt := range opts {
		opt.Apply(&c)
	}
	return c
}

type OptionS3Expires time.Duration

func (o OptionS3Expires) Apply(c *confS3Upload) {
	c.S3Expires = time.Duration(o)
}

func WithS3Expires(s3Expires time.Duration) OptionS3Expires {
	return OptionS3Expires(s3Expires)
}
