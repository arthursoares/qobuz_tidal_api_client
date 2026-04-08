package qobuz

import (
	"errors"
	"fmt"
)

// Error represents an error returned by the Qobuz API.
type Error struct {
	Status  int
	Message string
	Code    int
}

func (e *Error) Error() string {
	return fmt.Sprintf("[%d] %s", e.Status, e.Message)
}

// IsAuthError reports whether the error is a 401 authentication error.
func IsAuthError(err error) bool {
	var e *Error
	if errors.As(err, &e) {
		return e.Status == 401
	}
	return false
}

// IsForbiddenError reports whether the error is a 403 forbidden error.
func IsForbiddenError(err error) bool {
	var e *Error
	if errors.As(err, &e) {
		return e.Status == 403
	}
	return false
}

// IsNotFoundError reports whether the error is a 404 not-found error.
func IsNotFoundError(err error) bool {
	var e *Error
	if errors.As(err, &e) {
		return e.Status == 404
	}
	return false
}

// IsRateLimitError reports whether the error is a 429 rate-limit error.
func IsRateLimitError(err error) bool {
	var e *Error
	if errors.As(err, &e) {
		return e.Status == 429
	}
	return false
}

// IsInvalidAppError reports whether the error is a 400 bad-request error.
func IsInvalidAppError(err error) bool {
	var e *Error
	if errors.As(err, &e) {
		return e.Status == 400
	}
	return false
}
