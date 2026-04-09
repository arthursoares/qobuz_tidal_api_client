// Package tidal provides a Go client for the Tidal music streaming API (v2).
//
// The Tidal API uses JSON:API format for all responses. This client handles
// the JSON:API envelope parsing automatically, exposing clean Go structs.
//
// Usage:
//
//	client := tidal.NewClient("access-token", "US", "user-id",
//	    tidal.WithRateLimit(1.0, 10),
//	)
//
//	// Favorites
//	albums, cursor, err := client.Favorites.GetAlbums(ctx, 50)
//
//	// Catalog
//	album, err := client.Catalog.GetAlbum(ctx, "album-id")
//
//	// Search
//	results, cursor, err := client.Search.Albums(ctx, "radiohead", 20)
package tidal

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"golang.org/x/time/rate"
)

// Client is the main Tidal API client.
type Client struct {
	Favorites *FavoritesService
	Playlists *PlaylistsService
	Catalog   *CatalogService
	Search    *SearchService
	transport *transport
	userID    string
}

// Option configures the Client.
type Option func(*Client)

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

// WithBaseURL overrides the default Tidal API base URL (useful for testing).
func WithBaseURL(baseURL string) Option {
	return func(c *Client) {
		c.transport.baseURL = baseURL
	}
}

// NewClient creates a new Tidal API client.
func NewClient(accessToken, countryCode, userID string, opts ...Option) *Client {
	t := newTransport(accessToken, countryCode)

	c := &Client{
		Favorites: &FavoritesService{t: t, userID: userID},
		Playlists: &PlaylistsService{t: t, userID: userID},
		Catalog:   &CatalogService{t: t},
		Search:    &SearchService{t: t},
		transport: t,
		userID:    userID,
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

// NewClientFromCredentials creates a Client from saved credentials.
// Automatically refreshes the token if expired.
func NewClientFromCredentials(opts ...Option) (*Client, error) {
	creds := LoadCredentials()
	if creds == nil {
		return nil, fmt.Errorf("no credentials found at %s — run: tidal login", CredentialsPath())
	}

	// Auto-refresh expired token
	if creds.IsExpired() {
		ctx := context.Background()
		if err := EnsureValidToken(ctx, creds); err != nil {
			return nil, fmt.Errorf("token expired: %w", err)
		}
	}

	return NewClient(creds.AccessToken, creds.CountryCode, creds.UserID, opts...), nil
}

// GetUser fetches the authenticated user's profile as raw JSON.
func (c *Client) GetUser(ctx context.Context) (json.RawMessage, error) {
	data, err := c.transport.get(ctx, fmt.Sprintf("users/%s", c.userID), map[string]string{})
	if err != nil {
		return nil, err
	}
	return json.RawMessage(data), nil
}
