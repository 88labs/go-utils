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
	}{
		{
			name: "一般的なケース",
			args: args{
				code:   NotFoundErr,
				cause:  errors.New("cause"),
				detail: "detail",
			},
			wantErrString: "NotFoundErr: detail",
		},
		{
			name: "error codeだけ",
			args: args{
				code: NotFoundErr,
			},
			wantErrString: "NotFoundErr",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := New(tt.args.code, tt.args.cause, tt.args.detail)
			if err == nil {
				t.Errorf("cannot get err")
			}

			andpadError, ok := err.(*CommonError)
			if !ok {
				t.Errorf("cannot get CommonError. err: %v", err)
			}

			actualErrString := andpadError.Error()
			if actualErrString != tt.wantErrString {
				t.Errorf("error string is invalid. actual: %s, expected:%s", actualErrString, tt.wantErrString)
			}

			actualCause := andpadError.Unwrap()
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
	}{
		{
			name: "一般的なケース",
			args: args{
				code:       NotFoundErr,
				cause:      errors.New("cause"),
				detail:     "detail %s",
				detailArgs: []interface{}{"test"},
			},
			wantErrString: "NotFoundErr: detail test",
		},
		{
			name: "causeなし",
			args: args{
				code:       NotFoundErr,
				detail:     "detail %s",
				detailArgs: []interface{}{"test"},
			},
			wantErrString: "NotFoundErr: detail test",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Newf(tt.args.code, tt.args.cause, tt.args.detail, tt.args.detailArgs...)
			if err == nil {
				t.Errorf("cannot get err")
			}

			andpadError, ok := err.(*CommonError)
			if !ok {
				t.Errorf("cannot get CommonError. err: %v", err)
			}

			actualErrString := andpadError.Error()
			if actualErrString != tt.wantErrString {
				t.Errorf("error string is invalid. actual: %s, expected:%s", actualErrString, tt.wantErrString)
			}

			actualCause := andpadError.Unwrap()
			if actualCause != tt.args.cause {
				t.Errorf("error cause is invalid. actual: %s, expected:%s", actualCause, tt.args.cause)
			}
		})
	}
}

func Test_toSummary(t *testing.T) {
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
			name: "不正値",
			args: args{
				code: ErrorCode(-1),
			},
			want: "UnknownErr",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := toSummary(tt.args.code); got != tt.want {
				t.Errorf("toSummary() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAndpadError_Format(t *testing.T) {
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
