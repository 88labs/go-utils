package s3donwload

type OptionS3Download interface {
	Apply(*confS3Download)
}

type confS3Download struct {
	FileNameReplacer FileNameReplacerFunc
}

type FileNameReplacerFunc func(S3Key string, baseFileName string) string
type OptionFileNameReplacer FileNameReplacerFunc

func (o OptionFileNameReplacer) Apply(c *confS3Download) {
	c.FileNameReplacer = FileNameReplacerFunc(o)
}

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
