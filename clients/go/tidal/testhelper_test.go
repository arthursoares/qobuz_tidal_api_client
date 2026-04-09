package tidal

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"

	"golang.org/x/time/rate"
)

// testServerAndClient creates an httptest.Server and a Client wired to it.
func testServerAndClient(handler http.HandlerFunc) (*httptest.Server, *Client) {
	server := httptest.NewServer(handler)

	t := &transport{
		baseURL:     server.URL,
		accessToken: "test-token",
		countryCode: "US",
		httpClient:  server.Client(),
		limiter:     rate.NewLimiter(rate.Inf, 1), // no rate limiting in tests
	}

	c := &Client{
		Favorites: &FavoritesService{t: t, userID: "test-user-123"},
		Playlists: &PlaylistsService{t: t, userID: "test-user-123"},
		Catalog:   &CatalogService{t: t},
		Search:    &SearchService{t: t},
		transport: t,
		userID:    "test-user-123",
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

// jsonAPIAlbum builds a JSON:API resource object for an album.
func jsonAPIAlbum(id, title string, numberOfItems int, mediaTags []string) map[string]any {
	return map[string]any{
		"type": "albums",
		"id":   id,
		"attributes": map[string]any{
			"title":           title,
			"albumType":       "ALBUM",
			"duration":        "PT46M17S",
			"numberOfItems":   numberOfItems,
			"numberOfVolumes": 1,
			"explicit":        false,
			"releaseDate":     "2017-06-30",
			"barcodeId":       "0123456789012",
			"popularity":      0.56,
			"mediaTags":       mediaTags,
		},
	}
}

// jsonAPITrack builds a JSON:API resource object for a track.
func jsonAPITrack(id, title, isrc string, explicit bool) map[string]any {
	return map[string]any{
		"type": "tracks",
		"id":   id,
		"attributes": map[string]any{
			"title":      title,
			"duration":   "PT2M58S",
			"isrc":       isrc,
			"explicit":   explicit,
			"popularity": 0.42,
			"mediaTags":  []string{"LOSSLESS"},
			"key":        "C",
			"keyScale":   "MAJOR",
		},
	}
}

// jsonAPIArtist builds a JSON:API resource object for an artist.
func jsonAPIArtist(id, name string, popularity float64) map[string]any {
	return map[string]any{
		"type": "artists",
		"id":   id,
		"attributes": map[string]any{
			"name":       name,
			"popularity": popularity,
		},
	}
}

// jsonAPIPlaylist builds a JSON:API resource object for a playlist.
func jsonAPIPlaylist(id, name, desc string, numberOfItems int) map[string]any {
	return map[string]any{
		"type": "playlists",
		"id":   id,
		"attributes": map[string]any{
			"name":              name,
			"description":       desc,
			"accessType":        "PUBLIC",
			"bounded":           true,
			"createdAt":         "2024-01-01T00:00:00Z",
			"lastModifiedAt":    "2024-01-02T00:00:00Z",
			"numberOfFollowers": 42,
			"numberOfItems":     numberOfItems,
			"playlistType":      "USER",
			"externalLinks":     []any{},
		},
	}
}

// jsonAPIGenre builds a JSON:API resource object for a genre.
func jsonAPIGenre(id, name string) map[string]any {
	return map[string]any{
		"type": "genres",
		"id":   id,
		"attributes": map[string]any{
			"genreName": name,
		},
	}
}

// jsonAPISingleDoc wraps a resource in a single-resource document.
func jsonAPISingleDoc(resource map[string]any) map[string]any {
	return map[string]any{
		"data": resource,
		"links": map[string]any{
			"self": "/test",
		},
	}
}

// jsonAPIMultiDoc wraps resources in a multi-resource document.
func jsonAPIMultiDoc(resources []map[string]any, nextURL string) map[string]any {
	doc := map[string]any{
		"data": resources,
		"links": map[string]any{
			"self": "/test",
		},
	}
	if nextURL != "" {
		doc["links"] = map[string]any{
			"self": "/test",
			"next": nextURL,
		}
	}
	return doc
}

// jsonAPIRelationshipDoc builds a relationship document with included resources.
func jsonAPIRelationshipDoc(identifiers []map[string]string, included []map[string]any, nextURL string) map[string]any {
	doc := map[string]any{
		"data":     identifiers,
		"included": included,
		"links": map[string]any{
			"self": "/test",
		},
	}
	if nextURL != "" {
		doc["links"] = map[string]any{
			"self": "/test",
			"next": nextURL,
		}
	}
	return doc
}
