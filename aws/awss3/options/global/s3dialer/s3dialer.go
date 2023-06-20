package s3dialer

import (
	"time"
)

type ConfGlobalDialer struct {
	// Timeout is the maximum amount of time a dial will wait for
	// a connect to complete. If Deadline is also set, it may fail
	// earlier.
	//
	// The default is no timeout.
	//
	// When using TCP and dialing a host name with multiple IP
	// addresses, the timeout may be divided between them.
	//
	// With or without a timeout, the operating system may impose
	// its own earlier timeout. For instance, TCP timeouts are
	// often around 3 minutes.
	Timeout time.Duration

	// Deadline is the absolute point in time after which dials
	// will fail. If Timeout is set, it may fail earlier.
	// Zero means no deadline, or dependent on the operating system
	// as with the Timeout option.
	Deadline *time.Time

	// KeepAlive specifies the interval between keep-alive
	// probes for an active network connection.
	// If zero, keep-alive probes are sent with a default value
	// (currently 15 seconds), if supported by the protocol and operating
	// system. Network protocols or operating systems that do
	// not support keep-alives ignore this field.
	// If negative, keep-alive probes are disabled.
	KeepAlive time.Duration
}

func NewConfGlobalDialer() *ConfGlobalDialer {
	return &ConfGlobalDialer{}
}

func (c *ConfGlobalDialer) WithTimeout(timeout time.Duration) {
	c.Timeout = timeout
}

func (c *ConfGlobalDialer) WithDeadline(deadline time.Time) {
	c.Deadline = &deadline
}

func (c *ConfGlobalDialer) WithKeepAlive(keepAlive time.Duration) {
	c.KeepAlive = keepAlive
}
