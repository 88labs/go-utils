package s3presigned

import (
	"time"
)

type OptionS3Presigned interface {
	Apply(*confS3Presigned)
}

type confS3Presigned struct {
	ContentDispositionType ContentDispositionType
	PresignExpires         time.Duration
	PresignFileName        string
}
type ContentDispositionType int

const (
	ContentDispositionTypeAttachment ContentDispositionType = iota
	ContentDispositionTypeInline
)

// nolint:revive
func GetS3PresignedConf(opts ...OptionS3Presigned) confS3Presigned {
	// default options
	c := confS3Presigned{
		PresignExpires: 15 * time.Minute,
	}
	for _, opt := range opts {
		opt.Apply(&c)
	}
	return c
}

type OptionPresignFileName string

func (o OptionPresignFileName) Apply(c *confS3Presigned) {
	c.PresignFileName = string(o)
}

func WithPresignFileName(fileName string) OptionPresignFileName {
	return OptionPresignFileName(fileName)
}

type OptionPresignExpires time.Duration

func (o OptionPresignExpires) Apply(c *confS3Presigned) {
	c.PresignExpires = time.Duration(o)
}

func WithPresignExpires(presignExpires time.Duration) OptionPresignExpires {
	return OptionPresignExpires(presignExpires)
}

type OptionContentDispositionType ContentDispositionType

func (o OptionContentDispositionType) Apply(c *confS3Presigned) {
	c.ContentDispositionType = ContentDispositionType(o)
}

func WithContentDispositionType(tp ContentDispositionType) OptionContentDispositionType {
	return OptionContentDispositionType(tp)
}
