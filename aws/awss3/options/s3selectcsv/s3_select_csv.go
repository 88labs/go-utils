package s3selectcsv

import "github.com/aws/aws-sdk-go-v2/service/s3/types"

type OptionS3SelectCSV interface {
	Apply(*confS3SelectCSV)
}

type confS3SelectCSV struct {
	SkipByteSize    int64
	CSVInput        types.CSVInput
	CompressionType types.CompressionType
	CSVOutput       types.CSVOutput
}

// nolint:revive
func GetS3SelectCSVConf(opts ...OptionS3SelectCSV) confS3SelectCSV {
	// default options
	c := confS3SelectCSV{
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

type OptionCSVInput types.CSVInput

func (o OptionCSVInput) Apply(c *confS3SelectCSV) {
	c.CSVInput = types.CSVInput(o)
}

// WithCSVInput
// Describes how an uncompressed comma-separated values (CSV)-formatted input object is
// formatted.
// https://pkg.go.dev/github.com/aws/aws-sdk-go-v2/service/s3/types#CSVInput
func WithCSVInput(csvInput types.CSVInput) OptionCSVInput {
	return OptionCSVInput(csvInput)
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

type OptionCSVOutput types.CSVOutput

func (o OptionCSVOutput) Apply(c *confS3SelectCSV) {
	c.CSVOutput = types.CSVOutput(o)
}

// WithCSVOutput
// Describes how uncompressed comma-separated values (CSV)-formatted results are
// formatted.
// https://pkg.go.dev/github.com/aws/aws-sdk-go-v2/service/s3/types#CSVOutput
func WithCSVOutput(csvOutput types.CSVOutput) OptionCSVOutput {
	return OptionCSVOutput(csvOutput)
}
