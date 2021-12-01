package http

import (
	"net/http"
	"testing"

	"github.com/88labs/go-utils/cerrors"
)

func TestToCommonErrorCode(t *testing.T) {
	type args struct {
		statusCode int
	}
	tests := []struct {
		name string
		args args
		want cerrors.ErrorCode
	}{
		{
			name: "StatusUnauthorized",
			args: args{
				http.StatusUnauthorized,
			},
			want: cerrors.UnauthenticatedErr,
		},
		{
			name: "StatusForbidden",
			args: args{
				http.StatusForbidden,
			},
			want: cerrors.PermissionErr,
		},
		{
			name: "StatusNotFound",
			args: args{
				http.StatusNotFound,
			},
			want: cerrors.NotFoundErr,
		},
		{
			name: "StatusNotImplemented",
			args: args{
				http.StatusNotImplemented,
			},
			want: cerrors.UnimplementedErr,
		},
		{
			name: "StatusBadRequest",
			args: args{
				http.StatusBadRequest,
			},
			want: cerrors.ParameterErr,
		},
		{
			name: "StatusServiceUnavailable",
			args: args{
				http.StatusServiceUnavailable,
			},
			want: cerrors.UnavailableErr,
		},
		{
			name: "StatusTooManyRequests",
			args: args{
				http.StatusTooManyRequests,
			},
			want: cerrors.ResourceExhaustedErr,
		},
		{
			name: "StatusInternalServerError",
			args: args{
				http.StatusInternalServerError,
			},
			want: cerrors.UnknownErr,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ToCommonErrorCode(tt.args.statusCode); got != tt.want {
				t.Errorf("ToCommonErrorCode() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestToHttpStatusCode(t *testing.T) {
	type args struct {
		errCode cerrors.ErrorCode
	}
	tests := []struct {
		name string
		args args
		want int
	}{
		{
			name: "UnauthenticatedErr",
			args: args{cerrors.UnauthenticatedErr},
			want: http.StatusUnauthorized,
		},
		{
			name: "PermissionErr",
			args: args{cerrors.PermissionErr},
			want: http.StatusForbidden,
		},
		{
			name: "NotFoundErr",
			args: args{cerrors.NotFoundErr},
			want: http.StatusNotFound,
		},
		{
			name: "UnimplementedErr",
			args: args{cerrors.UnimplementedErr},
			want: http.StatusNotImplemented,
		},
		{
			name: "ParameterErr",
			args: args{cerrors.ParameterErr},
			want: http.StatusBadRequest,
		},
		{
			name: "UnavailableErr",
			args: args{cerrors.UnavailableErr},
			want: http.StatusServiceUnavailable,
		},
		{
			name: "ResourceExhaustedErr",
			args: args{cerrors.ResourceExhaustedErr},
			want: http.StatusTooManyRequests,
		},
		{
			name: "UnknownErr",
			args: args{cerrors.UnknownErr},
			want: http.StatusInternalServerError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ToHttpStatusCode(tt.args.errCode); got != tt.want {
				t.Errorf("ToHttpStatusCode() = %v, want %v", got, tt.want)
			}
		})
	}
}
