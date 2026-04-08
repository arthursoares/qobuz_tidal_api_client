package qobuz

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"

	"golang.org/x/time/rate"
)

// testServerAndClient creates an httptest.Server and a Client wired to it.
// The handler receives the real HTTP requests from the client.
// The returned server should be closed after the test.
func testServerAndClient(handler http.HandlerFunc) (*httptest.Server, *Client) {
	server := httptest.NewServer(handler)

	t := &transport{
		baseURL:       server.URL,
		appID:         "test-app-id",
		userAuthToken: "test-token",
		httpClient:    server.Client(),
		limiter:       rate.NewLimiter(rate.Inf, 1), // no rate limiting in tests
	}

	c := &Client{
		Favorites: &FavoritesService{t: t},
		Playlists: &PlaylistsService{t: t},
		Catalog:   &CatalogService{t: t},
		Discovery: &DiscoveryService{t: t},
		Streaming: &StreamingService{t: t, appSecret: "test-secret"},
		transport: t,
	}

	return server, c
}

// mustJSON marshals v to JSON bytes. Panics on error.
func mustJSON(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}
