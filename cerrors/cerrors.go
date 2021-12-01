package cerrors

import (
	"fmt"

	"golang.org/x/xerrors"
)

type ErrorCode int32

const (
	PermissionErr ErrorCode = iota
	UnauthenticatedErr
	NotFoundErr
	ParameterErr
	UnimplementedErr
	UnknownErr
	FailedPreconditionErr
	UnavailableErr
	ResourceExhaustedErr
)

type CommonError struct {
	Code    ErrorCode
	summary string
	detail  string
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

func New(code ErrorCode, cause error, detail string) error {
	return &CommonError{
		Code:    code,
		summary: toSummary(code),
		detail:  detail,
		cause:   cause,
		frame:   xerrors.Caller(1),
	}
}

func Newf(code ErrorCode, cause error, detail string, args ...interface{}) error {
	return &CommonError{
		Code:    code,
		summary: toSummary(code),
		detail:  fmt.Sprintf(detail, args...),
		cause:   cause,
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
