// Package qobuz provides a Go client for the Qobuz music streaming API.
//
// Usage:
//
//	client := qobuz.NewClient("app-id", "user-auth-token",
//	    qobuz.WithAppSecret("secret"),
//	)
//
//	// Favorites
//	albums, err := client.Favorites.GetAlbums(ctx, 50, 0)
//
//	// Catalog
//	album, err := client.Catalog.GetAlbum(ctx, "album-id")
//
//	// Streaming (requires app secret)
//	fileURL, err := client.Streaming.GetFileURL(ctx, trackID, 3)
package qobuz

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"golang.org/x/time/rate"
)

// Client is the main Qobuz API client.
type Client struct {
	Favorites *FavoritesService
	Playlists *PlaylistsService
	Catalog   *CatalogService
	Discovery *DiscoveryService
	Streaming *StreamingService
	transport *transport
}

// Option configures the Client.
type Option func(*Client)

// WithAppSecret sets the app secret for request signing (streaming endpoints).
func WithAppSecret(secret string) Option {
	return func(c *Client) {
		c.Streaming.appSecret = secret
	}
}

// WithHTTPClient sets a custom HTTP client.
func WithHTTPClient(httpClient *http.Client) Option {
	return func(c *Client) {
		c.transport.httpClient = httpClient
	}
}

// WithRateLimit sets the rate limiter (requests per second, burst size).
func WithRateLimit(rps float64, burst int) Option {
	return func(c *Client) {
		c.transport.limiter = rate.NewLimiter(rate.Limit(rps), burst)
	}
}

// WithBaseURL overrides the default Qobuz API base URL (useful for testing).
func WithBaseURL(baseURL string) Option {
	return func(c *Client) {
		c.transport.baseURL = baseURL
	}
}

// NewClient creates a new Qobuz API client.
func NewClient(appID, userAuthToken string, opts ...Option) *Client {
	t := newTransport(appID, userAuthToken)

	c := &Client{
		Favorites: &FavoritesService{t: t},
		Playlists: &PlaylistsService{t: t},
		Catalog:   &CatalogService{t: t},
		Discovery: &DiscoveryService{t: t},
		Streaming: &StreamingService{t: t},
		transport: t,
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

// LastUpdate polls for library changes and returns timestamps for each section.
func (c *Client) LastUpdate(ctx context.Context) (*LastUpdate, error) {
	data, err := c.transport.get(ctx, "user/lastUpdate", map[string]string{})
	if err != nil {
		return nil, err
	}

	// The API wraps the result in {"last_update": {...}}
	var wrapper lastUpdateWrapper
	if err := json.Unmarshal(data, &wrapper); err != nil {
		return nil, fmt.Errorf("parse last update: %w", err)
	}
	return &wrapper.LastUpdate, nil
}

// Login validates the current token and returns the user profile as raw JSON.
func (c *Client) Login(ctx context.Context) (json.RawMessage, error) {
	data, err := c.transport.postForm(ctx, "user/login", map[string]string{
		"extra": "partner",
	})
	if err != nil {
		return nil, err
	}
	return json.RawMessage(data), nil
}
