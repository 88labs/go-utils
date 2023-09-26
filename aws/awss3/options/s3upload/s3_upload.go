package s3upload

import "time"

type OptionS3Upload interface {
	Apply(*confS3Upload)
}

type confS3Upload struct {
	S3Expires *time.Duration
}

// nolint:revive
func GetS3UploadConf(opts ...OptionS3Upload) confS3Upload {
	// default options
	c := confS3Upload{}
	for _, opt := range opts {
		opt.Apply(&c)
	}
	return c
}

type OptionS3Expires time.Duration

func (o OptionS3Expires) Apply(c *confS3Upload) {
	d := time.Duration(o)
	c.S3Expires = &d
}

// WithS3Expires
// Set the expires time of an object
// https://docs.aws.amazon.com/AmazonS3/latest/userguide/lifecycle-expire-general-considerations.html
func WithS3Expires(s3Expires time.Duration) OptionS3Expires {
	return OptionS3Expires(s3Expires)
}
