package s3upload

import (
	"time"
)

type OptionS3Upload interface {
	Apply(*confS3Upload)
}

type confS3Upload struct {
	S3Expires *time.Duration
}

type OptionS3Expires time.Duration

func (o OptionS3Expires) Apply(c *confS3Upload) {
	d := time.Duration(o)
	c.S3Expires = &d
}

func WithS3Expires(s3Expires time.Duration) OptionS3Expires {
	return OptionS3Expires(s3Expires)
}

// nolint:revive
func GetS3UploadConf(opts ...OptionS3Upload) confS3Upload {
	var c confS3Upload
	for _, opt := range opts {
		opt.Apply(&c)
	}
	return c
}
