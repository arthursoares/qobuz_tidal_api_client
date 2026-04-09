package tidal

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"golang.org/x/time/rate"
)

const (
	defaultBaseURL   = "https://openapi.tidal.com/v2"
	defaultUserAgent = "tidal-go-sdk/0.1.0"
	jsonAPIMediaType = "application/vnd.api+json"
)

// transport is the low-level HTTP client for the Tidal REST API.
type transport struct {
	baseURL     string
	accessToken string
	countryCode string
	httpClient  *http.Client
	limiter     *rate.Limiter
}

// newTransport creates a new transport with the given access token.
func newTransport(accessToken, countryCode string) *transport {
	return &transport{
		baseURL:     defaultBaseURL,
		accessToken: accessToken,
		countryCode: countryCode,
		httpClient:  &http.Client{Timeout: 30 * time.Second},
		limiter:     rate.NewLimiter(rate.Every(2*time.Second), 5),
	}
}

// headers returns the standard request headers.
func (t *transport) headers() http.Header {
	h := http.Header{}
	h.Set("User-Agent", defaultUserAgent)
	h.Set("Accept", jsonAPIMediaType)
	if t.accessToken != "" {
		h.Set("Authorization", "Bearer "+t.accessToken)
	}
	return h
}

// get performs a GET request and returns the raw response bytes.
func (t *transport) get(ctx context.Context, endpoint string, params map[string]string) ([]byte, error) {
	if err := t.limiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("rate limiter: %w", err)
	}

	u, err := url.Parse(t.baseURL + "/" + endpoint)
	if err != nil {
		return nil, fmt.Errorf("parse url: %w", err)
	}

	q := u.Query()
	// Add countryCode by default if not explicitly provided
	if _, has := params["countryCode"]; !has && t.countryCode != "" {
		q.Set("countryCode", t.countryCode)
	}
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

// postJSON performs a POST request with a JSON:API body.
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
	req.Header.Set("Content-Type", jsonAPIMediaType)

	return t.doRequest(req)
}

// patchJSON performs a PATCH request with a JSON:API body.
func (t *transport) patchJSON(ctx context.Context, endpoint string, body any) ([]byte, error) {
	if err := t.limiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("rate limiter: %w", err)
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal json: %w", err)
	}

	u := t.baseURL + "/" + endpoint
	req, err := http.NewRequestWithContext(ctx, http.MethodPatch, u, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header = t.headers()
	req.Header.Set("Content-Type", jsonAPIMediaType)

	return t.doRequest(req)
}

// deleteJSON performs a DELETE request, optionally with a JSON:API body.
func (t *transport) deleteJSON(ctx context.Context, endpoint string, body any) ([]byte, error) {
	if err := t.limiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("rate limiter: %w", err)
	}

	var bodyReader io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal json: %w", err)
		}
		bodyReader = bytes.NewReader(jsonBody)
	}

	u := t.baseURL + "/" + endpoint
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, u, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header = t.headers()
	if body != nil {
		req.Header.Set("Content-Type", jsonAPIMediaType)
	}

	return t.doRequest(req)
}

// delete performs a DELETE request without a body.
func (t *transport) delete(ctx context.Context, endpoint string) ([]byte, error) {
	return t.deleteJSON(ctx, endpoint, nil)
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

	// 204 No Content is success with no body
	if resp.StatusCode == 204 {
		return nil, nil
	}

	return body, nil
}
