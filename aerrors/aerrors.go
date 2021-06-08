package aerrors

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
)

type AndpadError struct {
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
	default:
		return "UnknownErr"
	}
}

func New(code ErrorCode, cause error, detail string) error {
	return &AndpadError{
		Code:    code,
		summary: toSummary(code),
		detail:  detail,
		cause:   cause,
		frame:   xerrors.Caller(1),
	}
}

func Newf(code ErrorCode, cause error, detail string, args ...interface{}) error {
	return &AndpadError{
		Code:    code,
		summary: toSummary(code),
		detail:  fmt.Sprintf(detail, args...),
		cause:   cause,
		frame:   xerrors.Caller(1),
	}
}

func (err *AndpadError) Error() string {
	if err.detail == "" {
		return err.summary
	} else {
		return err.summary + ": " + err.detail
	}
}

func (err *AndpadError) Unwrap() error { return err.cause }

func (e *AndpadError) Format(s fmt.State, v rune) { xerrors.FormatError(e, s, v) }

func (e *AndpadError) FormatError(p xerrors.Printer) (next error) {
	p.Print(e.Error())
	e.frame.Format(p)
	return e.cause
}
