package types

import "errors"

// TraceReasonFromError maps a WebxError to the appropriate TraceReason.
// Falls back to TraceBackendFailed for unknown or non-WebxError errors.
func TraceReasonFromError(err error) TraceReason {
	var wxErr *WebxError
	if !errors.As(err, &wxErr) {
		return TraceBackendFailed
	}
	switch wxErr.Code {
	case ErrContentEmpty:
		return TraceEmptyContent
	case ErrAntiBot:
		return TraceAntiBot
	case ErrRateLimited:
		return TraceRateLimited
	case ErrLoginRequired:
		return TraceLoginRequired
	case ErrTLSBlocked:
		return TraceBackendFailed
	case ErrPartialContent:
		return TracePartialContent
	default:
		return TraceBackendFailed
	}
}
