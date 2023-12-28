package cerrors

import (
	"errors"
	"fmt"
	"testing"

	"golang.org/x/xerrors"
)

func TestNew(t *testing.T) {
	type args struct {
		code   ErrorCode
		cause  error
		detail string
	}
	tests := []struct {
		name          string
		args          args
		wantErrString string
		wantErrLevel  ErrorLevel
	}{
		{
			name: "不具合とは限らないエラー",
			args: args{
				code:   NotFoundErr,
				cause:  errors.New("cause"),
				detail: "detail",
			},
			wantErrString: "NotFoundErr: detail",
			wantErrLevel:  ErrorLevelWarn,
		},
		{
			name: "不具合を表すエラー",
			args: args{
				code:   UnimplementedErr,
				cause:  errors.New("cause"),
				detail: "detail",
			},
			wantErrString: "UnimplementedErr: detail",
			wantErrLevel:  ErrorLevelError,
		},
		{
			name: "error codeだけ",
			args: args{
				code: NotFoundErr,
			},
			wantErrString: "NotFoundErr",
			wantErrLevel:  ErrorLevelWarn,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := New(tt.args.code, tt.args.cause, tt.args.detail)
			if err == nil {
				t.Errorf("cannot get err")
			}

			commonError, ok := err.(*CommonError)
			if !ok {
				t.Errorf("cannot get CommonError. err: %v", err)
			}

			actualErrString := commonError.Error()
			if actualErrString != tt.wantErrString {
				t.Errorf("error string is invalid. actual: %s, expected:%s", actualErrString, tt.wantErrString)
			}

			actualErrLevel := commonError.Level
			if commonError.Level != tt.wantErrLevel {
				t.Errorf("error string is invalid. actual: %d, expected:%d", actualErrLevel, tt.wantErrLevel)
			}

			actualCause := commonError.Unwrap()
			if actualCause != tt.args.cause {
				t.Errorf("error cause is invalid. actual: %s, expected:%s", actualCause, tt.args.cause)
			}
		})
	}
}

func TestNewf(t *testing.T) {
	type args struct {
		code       ErrorCode
		cause      error
		detail     string
		detailArgs []interface{}
	}
	tests := []struct {
		name          string
		args          args
		wantErrString string
		wantErrLevel  ErrorLevel
	}{
		{
			name: "不具合とは限らないエラー",
			args: args{
				code:       NotFoundErr,
				cause:      errors.New("cause"),
				detail:     "detail %s",
				detailArgs: []interface{}{"test"},
			},
			wantErrString: "NotFoundErr: detail test",
			wantErrLevel:  ErrorLevelWarn,
		},
		{
			name: "不具合を表すエラー",
			args: args{
				code:       UnimplementedErr,
				cause:      errors.New("cause"),
				detail:     "detail %s",
				detailArgs: []interface{}{"test"},
			},
			wantErrString: "UnimplementedErr: detail test",
			wantErrLevel:  ErrorLevelError,
		},
		{
			name: "causeなし",
			args: args{
				code:       NotFoundErr,
				detail:     "detail %s",
				detailArgs: []interface{}{"test"},
			},
			wantErrString: "NotFoundErr: detail test",
			wantErrLevel:  ErrorLevelWarn,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Newf(tt.args.code, tt.args.cause, tt.args.detail, tt.args.detailArgs...)
			if err == nil {
				t.Errorf("cannot get err")
			}

			commonError, ok := err.(*CommonError)
			if !ok {
				t.Errorf("cannot get CommonError. err: %v", err)
			}

			actualErrString := commonError.Error()
			if actualErrString != tt.wantErrString {
				t.Errorf("error string is invalid. actual: %s, expected:%s", actualErrString, tt.wantErrString)
			}

			actualErrLevel := commonError.Level
			if commonError.Level != tt.wantErrLevel {
				t.Errorf("error string is invalid. actual: %d, expected:%d", actualErrLevel, tt.wantErrLevel)
			}

			actualCause := commonError.Unwrap()
			if actualCause != tt.args.cause {
				t.Errorf("error cause is invalid. actual: %s, expected:%s", actualCause, tt.args.cause)
			}
		})
	}
}

func Test_ErrorCode_String(t *testing.T) {
	type args struct {
		code ErrorCode
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "PermissionErr",
			args: args{
				code: PermissionErr,
			},
			want: "PermissionErr",
		},
		{
			name: "UnauthenticatedErr",
			args: args{
				code: UnauthenticatedErr,
			},
			want: "UnauthenticatedErr",
		},
		{
			name: "NotFoundErr",
			args: args{
				code: NotFoundErr,
			},
			want: "NotFoundErr",
		},
		{
			name: "ParameterErr",
			args: args{
				code: ParameterErr,
			},
			want: "ParameterErr",
		},
		{
			name: "UnimplementedErr",
			args: args{
				code: UnimplementedErr,
			},
			want: "UnimplementedErr",
		},
		{
			name: "UnknownErr",
			args: args{
				code: UnknownErr,
			},
			want: "UnknownErr",
		},
		{
			name: "UnavailableErr",
			args: args{
				code: UnavailableErr,
			},
			want: "UnavailableErr",
		},
		{
			name: "ResourceExhaustedErr",
			args: args{
				code: ResourceExhaustedErr,
			},
			want: "ResourceExhaustedErr",
		},
		{
			name: "FailedPreconditionErr",
			args: args{
				code: FailedPreconditionErr,
			},
			want: "FailedPreconditionErr",
		},
		{
			name: "Canceled",
			args: args{
				code: Canceled,
			},
			want: "Canceled",
		},
		{
			name: "AlreadyExists",
			args: args{
				code: AlreadyExists,
			},
			want: "AlreadyExists",
		},
		{
			name: "不正値",
			args: args{
				code: ErrorCode(-1),
			},
			want: "UnknownErr",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.args.code.String(); got != tt.want {
				t.Errorf("ErrorCode.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_CommonError_Format(t *testing.T) {
	type fields struct {
		Code    ErrorCode
		summary string
		detail  string
		cause   error
		frame   xerrors.Frame
	}
	type args struct {
		p xerrors.Printer
	}
	tests := []struct {
		name   string
		fields fields
		args   args
	}{
		{
			name: "causeあり",
			fields: fields{
				Code:   PermissionErr,
				cause:  errors.New("cause"),
				detail: "detail",
			},
		},
		{
			name: "causeなし",
			fields: fields{
				Code:   PermissionErr,
				detail: "detail",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := New(tt.fields.Code, tt.fields.cause, "")
			// 期待動作はxerrorsに依存するので、深く踏み込まない
			_ = fmt.Sprintf("%+v", e)
		})
	}
}

func TestNewOp(t *testing.T) {
	type args struct {
		code ErrorCode
		opts []Option
	}
	type wants struct {
		level     ErrorLevel
		cause     error
		errString string
	}

	cause := errors.New("cause")
	tests := []struct {
		name  string
		args  args
		wants wants
	}{
		{
			name: "WithOptions",
			args: args{
				code: PermissionErr,
				opts: []Option{
					Cause(cause),
					Detail("detail"),
					Level(ErrorLevelError),
				},
			},
			wants: wants{
				level:     ErrorLevelError,
				cause:     cause,
				errString: "PermissionErr: detail",
			},
		},
		{
			name: "WithOptions(Detailf)",
			args: args{
				code: PermissionErr,
				opts: []Option{
					Cause(cause),
					Detail("detail:%d", 1),
					Level(ErrorLevelError),
				},
			},
			wants: wants{
				level:     ErrorLevelError,
				cause:     cause,
				errString: "PermissionErr: detail:1",
			},
		},
		{
			name: "Default",
			args: args{
				code: PermissionErr,
				opts: []Option{},
			},
			wants: wants{
				level:     ErrorLevelWarn,
				cause:     nil,
				errString: "PermissionErr",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NewOp(tt.args.code, tt.args.opts...)
			if err == nil {
				t.Errorf("cannot get err")
			}

			commonError, ok := err.(*CommonError)
			if !ok {
				t.Errorf("cannot get CommonError. err: %v", err)
			}

			actualErrString := commonError.Error()
			if actualErrString != tt.wants.errString {
				t.Errorf("error string is invalid. actual: %s, expected:%s", actualErrString, tt.wants.errString)
			}

			actualCause := commonError.Unwrap()
			if actualCause != tt.wants.cause {
				t.Errorf("error cause is invalid. actual: %s, expected:%s", actualCause, tt.wants.cause)
			}
		})
	}
}

func Test_defaultErrorLevel(t *testing.T) {
	type args struct {
		code ErrorCode
	}
	tests := []struct {
		name string
		args args
		want ErrorLevel
	}{
		{
			name: "PermissionErr",
			args: args{
				code: PermissionErr,
			},
			want: ErrorLevelWarn,
		},
		{
			name: "UnauthenticatedErr",
			args: args{
				code: UnauthenticatedErr,
			},
			want: ErrorLevelWarn,
		},
		{
			name: "NotFoundErr",
			args: args{
				code: NotFoundErr,
			},
			want: ErrorLevelWarn,
		},
		{
			name: "ParameterErr",
			args: args{
				code: ParameterErr,
			},
			want: ErrorLevelWarn,
		},
		{
			name: "UnimplementedErr",
			args: args{
				code: UnimplementedErr,
			},
			want: ErrorLevelError,
		},
		{
			name: "UnknownErr",
			args: args{
				code: UnknownErr,
			},
			want: ErrorLevelError,
		},
		{
			name: "UnavailableErr",
			args: args{
				code: UnavailableErr,
			},
			want: ErrorLevelError,
		},
		{
			name: "ResourceExhaustedErr",
			args: args{
				code: ResourceExhaustedErr,
			},
			want: ErrorLevelWarn,
		},
		{
			name: "FailedPreconditionErr",
			args: args{
				code: FailedPreconditionErr,
			},
			want: ErrorLevelWarn,
		},
		{
			name: "不正値",
			args: args{
				code: ErrorCode(-1),
			},
			want: ErrorLevelError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := defaultErrorLevel(tt.args.code); got != tt.want {
				t.Errorf("defaultErrorLevel() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCode(t *testing.T) {
	type args struct {
		err error
	}
	tests := []struct {
		name string
		args args
		want ErrorCode
	}{
		{
			name: "CommonErrorが指定されたらそのエラーコードを返すこと",
			args: args{
				err: New(NotFoundErr, nil, ""),
			},
			want: NotFoundErr,
		},
		{
			name: "エラーでないならOKを返すこと",
			args: args{
				err: nil,
			},
			want: OK,
		},
		{
			name: "CommonErrorでないエラーが指定されたらUnknownErrを返すこと",
			args: args{
				err: errors.New("unknown"),
			},
			want: UnknownErr,
		},
		{
			name: "CommonErrorをWrapしたエラーは、WrapされたCommonErrorのエラーコードを返すこと",
			args: args{
				err: xerrors.Errorf(": %w", New(ParameterErr, nil, "")),
			},
			want: ParameterErr,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Code(tt.args.err); got != tt.want {
				t.Errorf("Code() = %v, want %v", got, tt.want)
			}
		})
	}
}
