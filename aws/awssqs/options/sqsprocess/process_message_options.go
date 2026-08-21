package sqsprocess

// ProcessMessageOption configures one ProcessMessage call.
type ProcessMessageOption interface {
	Apply(*confProcessMessage)
}

type confProcessMessage struct {
	OperationName string
}

// OptionOperationName changes the operation portion of the process span name.
// The queue name is always appended by the SQS client.
type OptionOperationName string

// Apply configures the operation portion of a process span name.
func (o OptionOperationName) Apply(c *confProcessMessage) {
	c.OperationName = string(o)
}

// WithOperationName changes the operation portion of the process span name
// for this API call. The default is "process" and the final name includes the
// queue, for example "consume orders".
func WithOperationName(operationName string) OptionOperationName {
	return OptionOperationName(operationName)
}

func GetConf(opts ...ProcessMessageOption) confProcessMessage {
	var c confProcessMessage
	for _, opt := range opts {
		if opt != nil {
			opt.Apply(&c)
		}
	}
	return c
}
