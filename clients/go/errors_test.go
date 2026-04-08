package qobuz

import (
	"errors"
	"testing"
)

func TestErrorMessage(t *testing.T) {
	err := &Error{Status: 401, Message: "Session authentication is required"}
	expected := "[401] Session authentication is required"
	if err.Error() != expected {
		t.Errorf("Error() = %q, want %q", err.Error(), expected)
	}
}

func TestIsAuthError(t *testing.T) {
	err := &Error{Status: 401, Message: "auth failed"}
	if !IsAuthError(err) {
		t.Error("expected IsAuthError to return true for 401")
	}
	if IsAuthError(&Error{Status: 403, Message: "forbidden"}) {
		t.Error("expected IsAuthError to return false for 403")
	}
}

func TestIsForbiddenError(t *testing.T) {
	if !IsForbiddenError(&Error{Status: 403, Message: "forbidden"}) {
		t.Error("expected IsForbiddenError to return true for 403")
	}
	if IsForbiddenError(&Error{Status: 401, Message: "auth"}) {
		t.Error("expected IsForbiddenError to return false for 401")
	}
}

func TestIsNotFoundError(t *testing.T) {
	if !IsNotFoundError(&Error{Status: 404, Message: "not found"}) {
		t.Error("expected IsNotFoundError to return true for 404")
	}
}

func TestIsRateLimitError(t *testing.T) {
	if !IsRateLimitError(&Error{Status: 429, Message: "rate limited"}) {
		t.Error("expected IsRateLimitError to return true for 429")
	}
}

func TestIsInvalidAppError(t *testing.T) {
	if !IsInvalidAppError(&Error{Status: 400, Message: "bad request"}) {
		t.Error("expected IsInvalidAppError to return true for 400")
	}
}

func TestErrorHelperWithNonQobuzError(t *testing.T) {
	err := errors.New("some other error")
	if IsAuthError(err) {
		t.Error("IsAuthError should return false for non-Qobuz error")
	}
	if IsForbiddenError(err) {
		t.Error("IsForbiddenError should return false for non-Qobuz error")
	}
	if IsNotFoundError(err) {
		t.Error("IsNotFoundError should return false for non-Qobuz error")
	}
	if IsRateLimitError(err) {
		t.Error("IsRateLimitError should return false for non-Qobuz error")
	}
}

func TestErrorHelperWithWrappedError(t *testing.T) {
	inner := &Error{Status: 401, Message: "auth"}
	wrapped := errors.Join(errors.New("context"), inner)
	if !IsAuthError(wrapped) {
		t.Error("IsAuthError should unwrap and find the Qobuz error")
	}
}
