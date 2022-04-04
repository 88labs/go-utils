package http

import (
	"net/http"

	"github.com/88labs/go-utils/cerrors"
)

func ToCommonErrorCode(statusCode int) cerrors.ErrorCode {
	switch statusCode {
	case http.StatusUnauthorized:
		return cerrors.UnauthenticatedErr
	case http.StatusForbidden:
		return cerrors.PermissionErr
	case http.StatusNotFound:
		return cerrors.NotFoundErr
	case http.StatusNotImplemented:
		return cerrors.UnimplementedErr
	case http.StatusBadRequest, http.StatusUnprocessableEntity:
		return cerrors.ParameterErr
	case http.StatusServiceUnavailable:
		return cerrors.UnavailableErr
	case http.StatusTooManyRequests:
		return cerrors.ResourceExhaustedErr
	default:
		return cerrors.UnknownErr
	}
}

func ToHttpStatusCode(errCode cerrors.ErrorCode) int {
	switch errCode {
	case cerrors.UnauthenticatedErr:
		return http.StatusUnauthorized
	case cerrors.PermissionErr:
		return http.StatusForbidden
	case cerrors.NotFoundErr:
		return http.StatusNotFound
	case cerrors.UnimplementedErr:
		return http.StatusNotImplemented
	case cerrors.ParameterErr:
		return http.StatusBadRequest
	case cerrors.UnavailableErr:
		return http.StatusServiceUnavailable
	case cerrors.ResourceExhaustedErr:
		return http.StatusTooManyRequests
	default:
		return http.StatusInternalServerError
	}
}
