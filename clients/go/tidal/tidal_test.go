package tidal

import (
	"context"
	"net/http"
	"testing"
)

func TestNewClient(t *testing.T) {
	c := NewClient("token", "US", "user-123")

	if c.Favorites == nil {
		t.Error("Favorites should not be nil")
	}
	if c.Playlists == nil {
		t.Error("Playlists should not be nil")
	}
	if c.Catalog == nil {
		t.Error("Catalog should not be nil")
	}
	if c.Search == nil {
		t.Error("Search should not be nil")
	}
	if c.userID != "user-123" {
		t.Errorf("userID = %q, want %q", c.userID, "user-123")
	}
}

func TestNewClientWithOptions(t *testing.T) {
	c := NewClient("token", "GB", "user-456",
		WithRateLimit(10.0, 20),
	)

	if c.transport.countryCode != "GB" {
		t.Errorf("countryCode = %q, want GB", c.transport.countryCode)
	}
}

func TestGetUser(t *testing.T) {
	server, client := testServerAndClient(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/users/test-user-123" {
			t.Errorf("path = %q, want /users/test-user-123", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Errorf("Authorization = %q, want Bearer test-token", r.Header.Get("Authorization"))
		}
		if r.Header.Get("Accept") != jsonAPIMediaType {
			t.Errorf("Accept = %q, want %q", r.Header.Get("Accept"), jsonAPIMediaType)
		}

		doc := map[string]any{
			"data": map[string]any{
				"type": "users",
				"id":   "test-user-123",
				"attributes": map[string]any{
					"country": "US",
				},
			},
			"links": map[string]any{
				"self": "/users/test-user-123",
			},
		}
		w.WriteHeader(200)
		w.Write(mustJSON(doc))
	})
	defer server.Close()

	data, err := client.GetUser(context.Background())
	if err != nil {
		t.Fatalf("GetUser: %v", err)
	}

	if data == nil {
		t.Fatal("data should not be nil")
	}
}

func TestHTTPHeaders(t *testing.T) {
	server, client := testServerAndClient(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Errorf("Authorization = %q, want Bearer test-token", r.Header.Get("Authorization"))
		}
		if r.Header.Get("Accept") != jsonAPIMediaType {
			t.Errorf("Accept = %q, want %q", r.Header.Get("Accept"), jsonAPIMediaType)
		}
		if r.Header.Get("User-Agent") != defaultUserAgent {
			t.Errorf("User-Agent = %q, want %q", r.Header.Get("User-Agent"), defaultUserAgent)
		}

		doc := jsonAPISingleDoc(jsonAPIAlbum("1", "Test", 1, nil))
		w.WriteHeader(200)
		w.Write(mustJSON(doc))
	})
	defer server.Close()

	_, err := client.Catalog.GetAlbum(context.Background(), "1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCountryCodeParam(t *testing.T) {
	server, client := testServerAndClient(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("countryCode") != "US" {
			t.Errorf("countryCode = %q, want US", r.URL.Query().Get("countryCode"))
		}

		doc := jsonAPISingleDoc(jsonAPIAlbum("1", "Test", 1, nil))
		w.WriteHeader(200)
		w.Write(mustJSON(doc))
	})
	defer server.Close()

	_, err := client.Catalog.GetAlbum(context.Background(), "1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
