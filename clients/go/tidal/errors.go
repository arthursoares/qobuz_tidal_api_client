package tidal

import (
	"encoding/json"
	"errors"
	"fmt"
)

// Error represents an error returned by the Tidal API.
type Error struct {
	Status  int
	Message string
	Detail  string
}

func (e *Error) Error() string {
	if e.Detail != "" {
		return fmt.Sprintf("[%d] %s: %s", e.Status, e.Message, e.Detail)
	}
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

// raiseForStatus converts an HTTP error response into a *Error.
// Tidal uses JSON:API error format: {"errors": [{"title": "...", "detail": "..."}]}
func raiseForStatus(status int, body []byte) error {
	var errResp struct {
		Errors []struct {
			Title  string `json:"title"`
			Detail string `json:"detail"`
			Status string `json:"status"`
		} `json:"errors"`
	}

	msg := fmt.Sprintf("HTTP %d", status)
	detail := ""

	if err := json.Unmarshal(body, &errResp); err == nil && len(errResp.Errors) > 0 {
		msg = errResp.Errors[0].Title
		detail = errResp.Errors[0].Detail
		if msg == "" {
			msg = fmt.Sprintf("HTTP %d", status)
		}
	}

	return &Error{Status: status, Message: msg, Detail: detail}
}
