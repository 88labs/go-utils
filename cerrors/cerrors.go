package cerrors

import (
	"errors"
	"fmt"

	"golang.org/x/xerrors"
)

type ErrorCode int32

const (
	OK ErrorCode = iota
	PermissionErr
	UnauthenticatedErr
	NotFoundErr
	ParameterErr
	UnimplementedErr
	UnknownErr
	FailedPreconditionErr
	UnavailableErr
	ResourceExhaustedErr
)

type ErrorLevel int

const (
	ErrorLevelFatal ErrorLevel = 1
	ErrorLevelError ErrorLevel = 2
	ErrorLevelWarn  ErrorLevel = 3
)

type CommonError struct {
	Code    ErrorCode
	summary string
	detail  string
	Level   ErrorLevel
	cause   error
	frame   xerrors.Frame
}

func toSummary(code ErrorCode) string {
	// Reflect使うか?
	switch code {
	case PermissionErr:
		return "PermissionErr"
	case UnauthenticatedErr:
		return "UnauthenticatedErr"
	case NotFoundErr:
		return "NotFoundErr"
	case ParameterErr:
		return "ParameterErr"
	case UnknownErr:
		return "UnknownErr"
	case UnimplementedErr:
		return "UnimplementedErr"
	case FailedPreconditionErr:
		return "FailedPreconditionErr"
	case UnavailableErr:
		return "UnavailableErr"
	case ResourceExhaustedErr:
		return "ResourceExhaustedErr"
	default:
		return "UnknownErr"
	}
}

func defaultErrorLevel(code ErrorCode) ErrorLevel {
	switch code {
	case PermissionErr,
		UnauthenticatedErr,
		NotFoundErr,
		ParameterErr,
		ResourceExhaustedErr,
		FailedPreconditionErr:
		return ErrorLevelWarn
	case UnknownErr,
		UnimplementedErr,
		UnavailableErr:
		return ErrorLevelError
	default:
		return ErrorLevelError
	}
}

type Option func(commonError *CommonError)

func Cause(cause error) Option {
	return func(c *CommonError) {
		c.cause = cause
	}
}

func Detail(detail string, args ...interface{}) Option {
	if len(args) == 0 {
		return func(c *CommonError) {
			c.detail = detail
		}
	} else {
		return func(c *CommonError) {
			c.detail = fmt.Sprintf(detail, args...)
		}
	}
}

func Level(level ErrorLevel) Option {
	return func(c *CommonError) {
		c.Level = level
	}
}

func NewOp(code ErrorCode, opts ...Option) error {
	c := &CommonError{
		Code:    code,
		summary: toSummary(code),
		detail:  "",
		Level:   defaultErrorLevel(code),
		frame:   xerrors.Caller(1),
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

func New(code ErrorCode, cause error, detail string) error {
	return &CommonError{
		Code:    code,
		summary: toSummary(code),
		detail:  detail,
		cause:   cause,
		Level:   defaultErrorLevel(code),
		frame:   xerrors.Caller(1),
	}
}

func Newf(code ErrorCode, cause error, detail string, args ...interface{}) error {
	return &CommonError{
		Code:    code,
		summary: toSummary(code),
		detail:  fmt.Sprintf(detail, args...),
		cause:   cause,
		Level:   defaultErrorLevel(code),
		frame:   xerrors.Caller(1),
	}
}

func (c *CommonError) Error() string {
	if c.detail == "" {
		return c.summary
	} else {
		return c.summary + ": " + c.detail
	}
}

func (c *CommonError) Unwrap() error { return c.cause }

func (c *CommonError) Format(s fmt.State, v rune) { xerrors.FormatError(c, s, v) }

func (c *CommonError) FormatError(p xerrors.Printer) (next error) {
	p.Print(c.Error())
	c.frame.Format(p)
	return c.cause
}

func Code(err error) ErrorCode {
	if err == nil {
		return OK
	}

	var e *CommonError
	if errors.As(err, &e) {
		return e.Code
	} else {
		return UnknownErr
	}
}
