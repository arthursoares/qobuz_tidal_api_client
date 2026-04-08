package qobuz

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/time/rate"
)

const (
	defaultBaseURL   = "https://www.qobuz.com/api.json/0.2"
	defaultUserAgent = "qobuz-go-sdk/0.1.0"
)

// transport is the low-level HTTP client for the Qobuz REST API.
type transport struct {
	baseURL       string
	appID         string
	userAuthToken string
	httpClient    *http.Client
	limiter       *rate.Limiter
}

// newTransport creates a new transport with the given configuration.
func newTransport(appID, userAuthToken string) *transport {
	return &transport{
		baseURL:       defaultBaseURL,
		appID:         appID,
		userAuthToken: userAuthToken,
		httpClient:    &http.Client{Timeout: 30 * time.Second},
		limiter:       rate.NewLimiter(rate.Every(2*time.Second), 5), // 30 req/min, burst of 5
	}
}

// headers returns the standard request headers.
func (t *transport) headers() http.Header {
	h := http.Header{}
	h.Set("User-Agent", defaultUserAgent)
	h.Set("X-App-Id", t.appID)
	if t.userAuthToken != "" {
		h.Set("X-User-Auth-Token", t.userAuthToken)
	}
	return h
}

// get performs a GET request and returns the raw JSON response bytes.
func (t *transport) get(ctx context.Context, endpoint string, params map[string]string) ([]byte, error) {
	if err := t.limiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("rate limiter: %w", err)
	}

	u, err := url.Parse(t.baseURL + "/" + endpoint)
	if err != nil {
		return nil, fmt.Errorf("parse url: %w", err)
	}

	q := u.Query()
	for k, v := range params {
		q.Set(k, v)
	}
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header = t.headers()

	return t.doRequest(req)
}

// postForm performs a POST request with form-encoded body.
func (t *transport) postForm(ctx context.Context, endpoint string, data map[string]string) ([]byte, error) {
	if err := t.limiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("rate limiter: %w", err)
	}

	form := url.Values{}
	for k, v := range data {
		form.Set(k, v)
	}

	u := t.baseURL + "/" + endpoint
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header = t.headers()
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	return t.doRequest(req)
}

// postJSON performs a POST request with a JSON body.
func (t *transport) postJSON(ctx context.Context, endpoint string, body any) ([]byte, error) {
	if err := t.limiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("rate limiter: %w", err)
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal json: %w", err)
	}

	u := t.baseURL + "/" + endpoint
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header = t.headers()
	req.Header.Set("Content-Type", "application/json")

	return t.doRequest(req)
}

// doRequest executes the HTTP request and checks for errors.
func (t *transport) doRequest(req *http.Request) ([]byte, error) {
	resp, err := t.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, raiseForStatus(resp.StatusCode, body)
	}

	return body, nil
}

// raiseForStatus converts an HTTP error response into an *Error.
func raiseForStatus(status int, body []byte) error {
	var errBody struct {
		Message string `json:"message"`
		Code    int    `json:"code"`
	}
	if err := json.Unmarshal(body, &errBody); err != nil {
		errBody.Message = fmt.Sprintf("HTTP %d", status)
	}
	if errBody.Message == "" {
		errBody.Message = fmt.Sprintf("HTTP %d", status)
	}
	return &Error{Status: status, Message: errBody.Message, Code: errBody.Code}
}
