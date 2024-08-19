package s3download

type OptionS3Download interface {
	Apply(*confS3Download)
}

type confS3Download struct {
	// Sets the function used to replace file names in downloaded files.
	FileNameReplacer FileNameReplacerFunc
}

type FileNameReplacerFunc func(S3Key string, baseFileName string) string
type OptionFileNameReplacer FileNameReplacerFunc

func (o OptionFileNameReplacer) Apply(c *confS3Download) {
	c.FileNameReplacer = FileNameReplacerFunc(o)
}

// WithFileNameReplacerFunc
// Sets the function used to replace file names in downloaded files.
func WithFileNameReplacerFunc(fileNameReplacerFunc FileNameReplacerFunc) OptionFileNameReplacer {
	return OptionFileNameReplacer(fileNameReplacerFunc)
}

// nolint:revive
func GetS3DownloadConf(opts ...OptionS3Download) confS3Download {
	var c confS3Download
	for _, opt := range opts {
		opt.Apply(&c)
	}
	return c
}
