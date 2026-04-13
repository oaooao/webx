package types

import "fmt"

type ErrorCode string

const (
	ErrAntiBot            ErrorCode = "ANTI_BOT"
	ErrLoginRequired      ErrorCode = "LOGIN_REQUIRED"
	ErrRateLimited        ErrorCode = "RATE_LIMITED"
	ErrContentEmpty       ErrorCode = "CONTENT_EMPTY"
	ErrBackendUnavailable ErrorCode = "BACKEND_UNAVAILABLE"
	ErrTLSBlocked         ErrorCode = "TLS_BLOCKED"
	ErrNoMatch            ErrorCode = "NO_MATCH"
	ErrUnsupportedKind    ErrorCode = "UNSUPPORTED_KIND"
	ErrPartialContent     ErrorCode = "PARTIAL_CONTENT"
	ErrFetchFailed        ErrorCode = "FETCH_FAILED"
	ErrFetchTimeout       ErrorCode = "FETCH_TIMEOUT"
	ErrBackendFailed      ErrorCode = "BACKEND_FAILED"
)

type WebxError struct {
	Code    ErrorCode
	Message string
}

func (e *WebxError) Error() string {
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

func NewWebxError(code ErrorCode, message string) *WebxError {
	return &WebxError{Code: code, Message: message}
}

func NewNoMatchError(url string) *WebxError {
	return &WebxError{Code: ErrNoMatch, Message: fmt.Sprintf("No adapter matched URL: %s", url)}
}

func NewNotImplementedError(feature string) *WebxError {
	return &WebxError{Code: ErrBackendUnavailable, Message: fmt.Sprintf("%s is not implemented yet", feature)}
}

// EnvelopeError is the JSON-serializable error in the envelope.
type EnvelopeError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}
