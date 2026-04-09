package tidal

import (
	"testing"
)

func TestErrorString(t *testing.T) {
	e := &Error{Status: 404, Message: "Not Found"}
	if e.Error() != "[404] Not Found" {
		t.Errorf("Error() = %q, want %q", e.Error(), "[404] Not Found")
	}

	e2 := &Error{Status: 400, Message: "Bad Request", Detail: "Invalid parameter"}
	if e2.Error() != "[400] Bad Request: Invalid parameter" {
		t.Errorf("Error() = %q, want %q", e2.Error(), "[400] Bad Request: Invalid parameter")
	}
}

func TestIsAuthError(t *testing.T) {
	if !IsAuthError(&Error{Status: 401, Message: "Unauthorized"}) {
		t.Error("expected IsAuthError to be true for 401")
	}
	if IsAuthError(&Error{Status: 403, Message: "Forbidden"}) {
		t.Error("expected IsAuthError to be false for 403")
	}
}

func TestIsForbiddenError(t *testing.T) {
	if !IsForbiddenError(&Error{Status: 403, Message: "Forbidden"}) {
		t.Error("expected IsForbiddenError to be true for 403")
	}
}

func TestIsNotFoundError(t *testing.T) {
	if !IsNotFoundError(&Error{Status: 404, Message: "Not Found"}) {
		t.Error("expected IsNotFoundError to be true for 404")
	}
}

func TestIsRateLimitError(t *testing.T) {
	if !IsRateLimitError(&Error{Status: 429, Message: "Too Many Requests"}) {
		t.Error("expected IsRateLimitError to be true for 429")
	}
}

func TestRaiseForStatusJSONAPI(t *testing.T) {
	body := []byte(`{"errors": [{"title": "Not Found", "detail": "Album does not exist", "status": "404"}]}`)
	err := raiseForStatus(404, body)

	e, ok := err.(*Error)
	if !ok {
		t.Fatalf("expected *Error, got %T", err)
	}
	if e.Status != 404 {
		t.Errorf("Status = %d, want 404", e.Status)
	}
	if e.Message != "Not Found" {
		t.Errorf("Message = %q, want %q", e.Message, "Not Found")
	}
	if e.Detail != "Album does not exist" {
		t.Errorf("Detail = %q, want %q", e.Detail, "Album does not exist")
	}
}

func TestRaiseForStatusInvalidJSON(t *testing.T) {
	err := raiseForStatus(500, []byte("not json"))
	e, ok := err.(*Error)
	if !ok {
		t.Fatalf("expected *Error, got %T", err)
	}
	if e.Status != 500 {
		t.Errorf("Status = %d, want 500", e.Status)
	}
	if e.Message != "HTTP 500" {
		t.Errorf("Message = %q, want %q", e.Message, "HTTP 500")
	}
}
