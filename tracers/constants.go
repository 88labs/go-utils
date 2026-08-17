package tracers

// TraceIDFormat controls the representation returned by TraceInfo.GetTraceID.
//
// OpenTelemetry represents trace IDs as 16-byte values and exposes them as
// lowercase 32-character hexadecimal strings. FormatInt64 provides the
// decimal representation of the lower 64 bits for integrations that require
// a 64-bit numeric form.
type TraceIDFormat uint8

const (
	// FormatString returns TraceID as a 128-bit, 32-character lowercase
	// hexadecimal string. It is the default format and matches the
	// representation defined by the OpenTelemetry Trace API.
	FormatString TraceIDFormat = iota
	// FormatInt64 returns the lower 64 bits of TraceID as a decimal string.
	FormatInt64
)
