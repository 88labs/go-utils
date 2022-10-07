package s3selectcsv

import "github.com/aws/aws-sdk-go-v2/service/s3/types"

type OptionS3SelectCSV interface {
	Apply(*confS3SelectCSV)
}

type confS3SelectCSV struct {
	SkipByteSize    int64
	FileHeaderInfo  types.FileHeaderInfo
	CompressionType types.CompressionType
}

// nolint:revive
func GetS3SelectCSVConf(opts ...OptionS3SelectCSV) confS3SelectCSV {
	// default options
	c := confS3SelectCSV{
		FileHeaderInfo:  types.FileHeaderInfoUse,
		CompressionType: types.CompressionTypeNone,
	}
	for _, opt := range opts {
		opt.Apply(&c)
	}
	return c
}

type OptionSkipByteSize int64

func (o OptionSkipByteSize) Apply(c *confS3SelectCSV) {
	c.SkipByteSize = int64(o)
}

// WithSkipByteSize
// It means scan from that point to the end of the file. For example, 50 means scan
// from byte 50 until the end of the file.
func WithSkipByteSize(skipByteSize int64) OptionSkipByteSize {
	return OptionSkipByteSize(skipByteSize)
}

type OptionFileHeaderInfo types.FileHeaderInfo

func (o OptionFileHeaderInfo) Apply(c *confS3SelectCSV) {
	c.FileHeaderInfo = types.FileHeaderInfo(o)
}

// WithFileHeaderInfo
// Describes the first line of input. Valid values are:
//
// * NONE: First line is not a header.
//
// * IGNORE: First line is a header, but you can't use the header values
// to indicate the column in an expression. You can use column position (such as
// _1, _2, …) to indicate the column (SELECT s._1 FROM OBJECT s).
//
// * Use: Default. First line is a header, and you can use the header value to identify a column in an
// expression (SELECT "name" FROM S3Object).
func WithFileHeaderInfo(headerInfo types.FileHeaderInfo) OptionFileHeaderInfo {
	return OptionFileHeaderInfo(headerInfo)
}

type OptionCompressionType types.CompressionType

func (o OptionCompressionType) Apply(c *confS3SelectCSV) {
	c.CompressionType = types.CompressionType(o)
}

// WithCompressionType
// Specifies object's compression format. Valid values: NONE, GZIP, BZIP2. Default
// Value: NONE.
func WithCompressionType(compressionType types.CompressionType) OptionCompressionType {
	return OptionCompressionType(compressionType)
}
