package sqsreceive

type ReceiveMessageOption interface {
	Apply(*confReceiveMessage)
}

type confReceiveMessage struct {
	// ReceiveMessage
	MaxNumberOfMessages int32
	WaitTimeSeconds     int32
	VisibilityTimeout   int32
}

type OptionMaxNumberOfMessages int32

func (o OptionMaxNumberOfMessages) Apply(c *confReceiveMessage) {
	c.MaxNumberOfMessages = int32(o)
}

// WithMaxNumberOfMessages min:1, max:10
func WithMaxNumberOfMessages(maxNumberOfMessages int32) OptionMaxNumberOfMessages {
	return OptionMaxNumberOfMessages(maxNumberOfMessages)
}

type OptionWaitTimeSeconds int32

func (o OptionWaitTimeSeconds) Apply(c *confReceiveMessage) {
	c.WaitTimeSeconds = int32(o)
}

func WithWaitTimeSeconds(waitTimeSeconds int32) OptionWaitTimeSeconds {
	return OptionWaitTimeSeconds(waitTimeSeconds)
}

type OptionVisibilityTimeout int32

func (o OptionVisibilityTimeout) Apply(c *confReceiveMessage) {
	c.VisibilityTimeout = int32(o)
}

func WithVisibilityTimeout(visibilityTimeout int32) OptionVisibilityTimeout {
	return OptionVisibilityTimeout(visibilityTimeout)
}

func GetConf(opts ...ReceiveMessageOption) confReceiveMessage {
	// default options
	c := confReceiveMessage{
		MaxNumberOfMessages: 1,
		WaitTimeSeconds:     20,
		VisibilityTimeout:   30,
	}
	for _, opt := range opts {
		opt.Apply(&c)
	}
	return c
}
