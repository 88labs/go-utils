package s3presigned

import "time"

type OptionS3Presigned interface {
	Apply(*confS3Presigned)
}

type confS3Presigned struct {
	// ContentDispositionType is the type of the Content-Disposition header.
	ContentDispositionType ContentDispositionType
	// PresignExpires is the duration that the presigned URL will be valid for.
	PresignExpires time.Duration
	// PresignFileName is the name of the file that will be returned in the Content-Disposition header.
	PresignFileName string
}
type ContentDispositionType int

const (
	// ContentDispositionTypeAttachment is the Content-Disposition header type for attachment.
	// default value, indicating it should be downloaded; most browsers presenting a 'Save as' dialog,
	// prefilled with the value of the filename parameters if present.
	ContentDispositionTypeAttachment ContentDispositionType = iota
	// ContentDispositionTypeInline is the Content-Disposition header type for inline.
	// indicating it can be displayed inside the Web page, or as the Web page.
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

// WithPresignFileName
// PresignFileName is the name of the file that will be returned in the Content-Disposition header.
// If set, the Content-Disposition header must be set to ContentDispositionTypeAttachment.
func WithPresignFileName(fileName string) OptionPresignFileName {
	return OptionPresignFileName(fileName)
}

type OptionPresignExpires time.Duration

func (o OptionPresignExpires) Apply(c *confS3Presigned) {
	c.PresignExpires = time.Duration(o)
}

// WithPresignExpires
// PresignExpires is the duration that the presigned URL will be valid for.
func WithPresignExpires(presignExpires time.Duration) OptionPresignExpires {
	return OptionPresignExpires(presignExpires)
}

type OptionContentDispositionType ContentDispositionType

func (o OptionContentDispositionType) Apply(c *confS3Presigned) {
	c.ContentDispositionType = ContentDispositionType(o)
}

// WithContentDispositionType
// ContentDispositionType is the type of the Content-Disposition header.
func WithContentDispositionType(tp ContentDispositionType) OptionContentDispositionType {
	return OptionContentDispositionType(tp)
}
