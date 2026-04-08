package qobuz

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
)

func TestNewClientHasAllServices(t *testing.T) {
	c := NewClient("app-id", "token")

	if c.Favorites == nil {
		t.Error("Favorites should not be nil")
	}
	if c.Playlists == nil {
		t.Error("Playlists should not be nil")
	}
	if c.Catalog == nil {
		t.Error("Catalog should not be nil")
	}
	if c.Discovery == nil {
		t.Error("Discovery should not be nil")
	}
	if c.Streaming == nil {
		t.Error("Streaming should not be nil")
	}
	if c.transport == nil {
		t.Error("transport should not be nil")
	}
}

func TestNewClientWithOptions(t *testing.T) {
	customHTTPClient := &http.Client{}

	c := NewClient("app-id", "token",
		WithAppSecret("my-secret"),
		WithHTTPClient(customHTTPClient),
		WithRateLimit(10.0, 5),
	)

	if c.Streaming.appSecret != "my-secret" {
		t.Errorf("appSecret = %q, want %q", c.Streaming.appSecret, "my-secret")
	}
	if c.transport.httpClient != customHTTPClient {
		t.Error("httpClient should be the custom one")
	}
}

func TestNewClientDefaultBaseURL(t *testing.T) {
	c := NewClient("app-id", "token")
	if c.transport.baseURL != defaultBaseURL {
		t.Errorf("baseURL = %q, want %q", c.transport.baseURL, defaultBaseURL)
	}
}

func TestNewClientWithBaseURL(t *testing.T) {
	c := NewClient("app-id", "token", WithBaseURL("http://localhost:8080"))
	if c.transport.baseURL != "http://localhost:8080" {
		t.Errorf("baseURL = %q, want %q", c.transport.baseURL, "http://localhost:8080")
	}
}

func TestClientLastUpdate(t *testing.T) {
	server, client := testServerAndClient(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/user/lastUpdate" {
			t.Errorf("path = %q, want /user/lastUpdate", r.URL.Path)
		}
		resp := map[string]any{
			"last_update": map[string]any{
				"favorite":        1000,
				"favorite_album":  2000,
				"favorite_artist": 3000,
				"favorite_track":  4000,
				"favorite_label":  5000,
				"playlist":        6000,
				"purchase":        7000,
			},
		}
		w.WriteHeader(200)
		json.NewEncoder(w).Encode(resp)
	})
	defer server.Close()

	lu, err := client.LastUpdate(context.Background())
	if err != nil {
		t.Fatalf("LastUpdate: %v", err)
	}

	if lu.Favorite != 1000 {
		t.Errorf("Favorite = %d, want 1000", lu.Favorite)
	}
	if lu.FavoriteAlbum != 2000 {
		t.Errorf("FavoriteAlbum = %d, want 2000", lu.FavoriteAlbum)
	}
	if lu.Playlist != 6000 {
		t.Errorf("Playlist = %d, want 6000", lu.Playlist)
	}
}

func TestClientLogin(t *testing.T) {
	server, client := testServerAndClient(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/user/login" {
			t.Errorf("path = %q, want /user/login", r.URL.Path)
		}
		if r.Method != "POST" {
			t.Errorf("method = %q, want POST", r.Method)
		}

		r.ParseForm()
		if r.PostForm.Get("extra") != "partner" {
			t.Errorf("extra = %q, want partner", r.PostForm.Get("extra"))
		}

		resp := map[string]any{
			"user": map[string]any{
				"id":   2113276,
				"name": "testuser",
			},
		}
		w.WriteHeader(200)
		json.NewEncoder(w).Encode(resp)
	})
	defer server.Close()

	data, err := client.Login(context.Background())
	if err != nil {
		t.Fatalf("Login: %v", err)
	}

	if data == nil {
		t.Fatal("data should not be nil")
	}

	var body map[string]any
	if err := json.Unmarshal(data, &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if body["user"] == nil {
		t.Error("response should contain user")
	}
}

func TestClientHeaders(t *testing.T) {
	var capturedAppID string
	var capturedToken string

	server, client := testServerAndClient(func(w http.ResponseWriter, r *http.Request) {
		capturedAppID = r.Header.Get("X-App-Id")
		capturedToken = r.Header.Get("X-User-Auth-Token")
		resp := map[string]any{
			"last_update": map[string]any{},
		}
		w.WriteHeader(200)
		json.NewEncoder(w).Encode(resp)
	})
	defer server.Close()

	_, err := client.LastUpdate(context.Background())
	if err != nil {
		t.Fatalf("LastUpdate: %v", err)
	}

	if capturedAppID != "test-app-id" {
		t.Errorf("X-App-Id = %q, want test-app-id", capturedAppID)
	}
	if capturedToken != "test-token" {
		t.Errorf("X-User-Auth-Token = %q, want test-token", capturedToken)
	}
}

func TestClientIntegrationWorkflow(t *testing.T) {
	// Simulates a real workflow: login -> get favorites -> search albums -> get file URL
	step := 0
	server, client := testServerAndClient(func(w http.ResponseWriter, r *http.Request) {
		step++
		switch r.URL.Path {
		case "/user/login":
			w.Write([]byte(`{"user": {"id": 1}}`))
		case "/favorite/getUserFavoriteIds":
			resp := map[string]any{
				"albums":  []string{"a1"},
				"tracks":  []int{100},
				"artists": []int{},
				"labels":  []int{},
				"awards":  []int{},
			}
			json.NewEncoder(w).Encode(resp)
		case "/album/search":
			resp := map[string]any{
				"albums": map[string]any{
					"items":  []any{map[string]any{"id": "found-1", "title": "Found Album"}},
					"total":  1,
					"limit":  50,
					"offset": 0,
				},
			}
			json.NewEncoder(w).Encode(resp)
		case "/file/url":
			resp := map[string]any{
				"track_id":     100,
				"format_id":    7,
				"url_template": "https://streaming.example.com/$SEGMENT$",
				"n_segments":   10,
			}
			json.NewEncoder(w).Encode(resp)
		default:
			w.WriteHeader(404)
			w.Write([]byte(`{"message": "not found"}`))
		}
	})
	defer server.Close()

	ctx := context.Background()

	// Step 1: Login
	_, err := client.Login(ctx)
	if err != nil {
		t.Fatalf("Login: %v", err)
	}

	// Step 2: Get favorite IDs
	ids, err := client.Favorites.GetIDs(ctx, 5000)
	if err != nil {
		t.Fatalf("GetIDs: %v", err)
	}
	if len(ids.Albums) != 1 {
		t.Errorf("favorite albums = %d, want 1", len(ids.Albums))
	}

	// Step 3: Search albums
	results, err := client.Catalog.SearchAlbums(ctx, "test", 50, 0)
	if err != nil {
		t.Fatalf("SearchAlbums: %v", err)
	}
	if len(results.Items) != 1 {
		t.Errorf("search results = %d, want 1", len(results.Items))
	}

	// Step 4: Get file URL
	fu, err := client.Streaming.GetFileURL(ctx, 100, 3)
	if err != nil {
		t.Fatalf("GetFileURL: %v", err)
	}
	if fu.TrackID != 100 {
		t.Errorf("TrackID = %d, want 100", fu.TrackID)
	}

	if step != 4 {
		t.Errorf("expected 4 requests, got %d", step)
	}
}
