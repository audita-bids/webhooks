package errors

import (
	"errors"
	"net/http"
	"strings"

	"github.com/newdesksoftwares/private-kit/decode"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type HTTPError struct {
	Status  int    `json:"-"`
	Code    string `json:"code,omitempty"`
	Message string `json:"message"`
}

func (e *HTTPError) Error() string {
	return e.Message
}

func New(status int, code, message string) error {
	return &HTTPError{
		Status:  status,
		Code:    code,
		Message: message,
	}
}

func BadRequest(code, message string) error {
	return New(http.StatusBadRequest, code, message)
}

func Unauthorized(message string) error {
	return New(http.StatusUnauthorized, "unauthorized", message)
}

func Forbidden(message string) error {
	return New(http.StatusForbidden, "forbidden", message)
}

func NotFound(resource string) error {
	return New(http.StatusNotFound, "not_found", resource+" not found")
}

func Conflict(message string) error {
	return New(http.StatusConflict, "conflict", message)
}

func Internal(message string) error {
	return New(http.StatusInternalServerError, "internal_error", message)
}

func FromGRPC(err error) *HTTPError {
	st, ok := status.FromError(err)
	if !ok {
		return nil
	}

	code := st.Code()
	message := cleanGRPCMessage(st.Message())
	msgLower := strings.ToLower(message)

	// Override status based on message content for Unknown/Internal errors
	if code == codes.Unknown || code == codes.Internal {
		switch {
		case containsAny(msgLower, []string{"not found", "does not exist"}):
			return &HTTPError{
				Status:  http.StatusNotFound,
				Code:    "not_found",
				Message: message,
			}
		case containsAny(msgLower, []string{"unauthorized", "invalid credentials"}):
			return &HTTPError{
				Status:  http.StatusUnauthorized,
				Code:    "unauthorized",
				Message: message,
			}
		case containsAny(msgLower, []string{"permission denied", "access denied"}):
			return &HTTPError{
				Status:  http.StatusForbidden,
				Code:    "forbidden",
				Message: message,
			}
		}
	}

	httpStatus := grpcToHTTPStatus(code)

	return &HTTPError{
		Status:  httpStatus,
		Code:    strings.ToLower(code.String()),
		Message: message,
	}
}

func ParseError(err error) *HTTPError {
	if err == nil {
		return nil
	}

	var httpErr *HTTPError
	if errors.As(err, &httpErr) {
		return httpErr
	}

	var svcErr *decode.SvcError
	if errors.As(err, &svcErr) {
		return &HTTPError{
			Status:  svcErr.Code,
			Code:    "validation_error",
			Message: svcErr.Message,
		}
	}

	if grpcErr := FromGRPC(err); grpcErr != nil {
		return grpcErr
	}

	msg := strings.ToLower(err.Error())
	switch {
	case containsAny(msg, []string{"not found"}):
		return &HTTPError{
			Status:  http.StatusNotFound,
			Code:    "not_found",
			Message: "Resource not found",
		}
	case containsAny(msg, []string{"unauthorized"}):
		return &HTTPError{
			Status:  http.StatusUnauthorized,
			Code:    "unauthorized",
			Message: "Unauthorized",
		}
	case containsAny(msg, []string{"insufficient"}):
		return &HTTPError{
			Status:  http.StatusUnauthorized,
			Code:    "insufficient_scopes",
			Message: "Insufficient scopes",
		}
	}

	return &HTTPError{
		Status:  http.StatusInternalServerError,
		Code:    "internal_error",
		Message: msg,
	}
}

func grpcToHTTPStatus(code codes.Code) int {
	switch code {
	case codes.OK:
		return http.StatusOK
	case codes.Canceled:
		return http.StatusRequestTimeout
	case codes.Unknown:
		return http.StatusInternalServerError
	case codes.InvalidArgument:
		return http.StatusBadRequest
	case codes.DeadlineExceeded:
		return http.StatusGatewayTimeout
	case codes.NotFound:
		return http.StatusNotFound
	case codes.AlreadyExists:
		return http.StatusConflict
	case codes.PermissionDenied:
		return http.StatusForbidden
	case codes.ResourceExhausted:
		return http.StatusTooManyRequests
	case codes.FailedPrecondition:
		return http.StatusBadRequest
	case codes.Aborted:
		return http.StatusConflict
	case codes.OutOfRange:
		return http.StatusBadRequest
	case codes.Unimplemented:
		return http.StatusNotImplemented
	case codes.Internal:
		return http.StatusInternalServerError
	case codes.Unavailable:
		return http.StatusServiceUnavailable
	case codes.DataLoss:
		return http.StatusInternalServerError
	case codes.Unauthenticated:
		return http.StatusUnauthorized
	default:
		return http.StatusInternalServerError
	}
}

func cleanGRPCMessage(msg string) string {
	if idx := strings.LastIndex(msg, "desc = "); idx != -1 {
		return msg[idx+7:]
	}
	return msg
}

func containsAny(s string, substrs []string) bool {
	for _, substr := range substrs {
		if strings.Contains(s, substr) {
			return true
		}
	}
	return false
}
