package tidal

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestFavoritesAddAlbum(t *testing.T) {
	var capturedPath string
	var capturedMethod string
	var capturedBody map[string]any
	var capturedAuth string

	server, client := testServerAndClient(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		capturedMethod = r.Method
		capturedAuth = r.Header.Get("Authorization")
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &capturedBody)
		w.WriteHeader(204)
	})
	defer server.Close()

	err := client.Favorites.AddAlbum(context.Background(), "album-123")
	if err != nil {
		t.Fatalf("AddAlbum: %v", err)
	}

	if capturedMethod != "POST" {
		t.Errorf("method = %q, want POST", capturedMethod)
	}
	if capturedPath != "/userCollections/test-user-123/relationships/albums" {
		t.Errorf("path = %q, want /userCollections/test-user-123/relationships/albums", capturedPath)
	}
	if capturedAuth != "Bearer test-token" {
		t.Errorf("Authorization = %q, want %q", capturedAuth, "Bearer test-token")
	}

	data, ok := capturedBody["data"].([]any)
	if !ok || len(data) != 1 {
		t.Fatalf("data should be array of 1, got %v", capturedBody["data"])
	}
	item := data[0].(map[string]any)
	if item["type"] != "albums" || item["id"] != "album-123" {
		t.Errorf("payload item = %v, want type=albums id=album-123", item)
	}
}

func TestFavoritesRemoveAlbum(t *testing.T) {
	var capturedMethod string
	var capturedPath string

	server, client := testServerAndClient(func(w http.ResponseWriter, r *http.Request) {
		capturedMethod = r.Method
		capturedPath = r.URL.Path
		w.WriteHeader(204)
	})
	defer server.Close()

	err := client.Favorites.RemoveAlbum(context.Background(), "album-456")
	if err != nil {
		t.Fatalf("RemoveAlbum: %v", err)
	}

	if capturedMethod != "DELETE" {
		t.Errorf("method = %q, want DELETE", capturedMethod)
	}
	if capturedPath != "/userCollections/test-user-123/relationships/albums" {
		t.Errorf("path = %q, want correct path", capturedPath)
	}
}

func TestFavoritesGetAlbums(t *testing.T) {
	server, client := testServerAndClient(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("method = %q, want GET", r.Method)
		}
		if !strings.Contains(r.URL.Path, "userCollections/test-user-123/relationships/albums") {
			t.Errorf("path = %q, missing expected segments", r.URL.Path)
		}
		if r.URL.Query().Get("include") != "albums" {
			t.Errorf("include = %q, want albums", r.URL.Query().Get("include"))
		}

		identifiers := []map[string]string{
			{"type": "albums", "id": "100"},
			{"type": "albums", "id": "200"},
		}
		included := []map[string]any{
			jsonAPIAlbum("100", "Test Album 1", 10, []string{"HIRES_LOSSLESS"}),
			jsonAPIAlbum("200", "Test Album 2", 8, []string{"LOSSLESS"}),
		}
		doc := jsonAPIRelationshipDoc(identifiers, included, "")
		w.WriteHeader(200)
		w.Write(mustJSON(doc))
	})
	defer server.Close()

	albums, cursor, err := client.Favorites.GetAlbums(context.Background(), 50)
	if err != nil {
		t.Fatalf("GetAlbums: %v", err)
	}

	if len(albums) != 2 {
		t.Fatalf("len = %d, want 2", len(albums))
	}
	if albums[0].ID != "100" {
		t.Errorf("albums[0].ID = %q, want %q", albums[0].ID, "100")
	}
	if albums[0].Title != "Test Album 1" {
		t.Errorf("albums[0].Title = %q, want %q", albums[0].Title, "Test Album 1")
	}
	if !albums[0].IsHiRes() {
		t.Error("albums[0] should be Hi-Res")
	}
	if albums[1].Title != "Test Album 2" {
		t.Errorf("albums[1].Title = %q, want %q", albums[1].Title, "Test Album 2")
	}
	if cursor != "" {
		t.Errorf("cursor = %q, want empty", cursor)
	}
}

func TestFavoritesAddTrack(t *testing.T) {
	var capturedPath string
	var capturedBody map[string]any

	server, client := testServerAndClient(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &capturedBody)
		w.WriteHeader(204)
	})
	defer server.Close()

	err := client.Favorites.AddTrack(context.Background(), "track-789")
	if err != nil {
		t.Fatalf("AddTrack: %v", err)
	}

	if !strings.Contains(capturedPath, "/relationships/tracks") {
		t.Errorf("path = %q, should contain /relationships/tracks", capturedPath)
	}

	data := capturedBody["data"].([]any)
	item := data[0].(map[string]any)
	if item["type"] != "tracks" || item["id"] != "track-789" {
		t.Errorf("payload = %v, want type=tracks id=track-789", item)
	}
}

func TestFavoritesAddArtist(t *testing.T) {
	var capturedPath string

	server, client := testServerAndClient(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		w.WriteHeader(204)
	})
	defer server.Close()

	err := client.Favorites.AddArtist(context.Background(), "artist-42")
	if err != nil {
		t.Fatalf("AddArtist: %v", err)
	}

	if !strings.Contains(capturedPath, "/relationships/artists") {
		t.Errorf("path = %q, should contain /relationships/artists", capturedPath)
	}
}

func TestFavoritesGetArtists(t *testing.T) {
	server, client := testServerAndClient(func(w http.ResponseWriter, r *http.Request) {
		identifiers := []map[string]string{
			{"type": "artists", "id": "a1"},
		}
		included := []map[string]any{
			jsonAPIArtist("a1", "JAY Z", 0.95),
		}
		doc := jsonAPIRelationshipDoc(identifiers, included, "")
		w.WriteHeader(200)
		w.Write(mustJSON(doc))
	})
	defer server.Close()

	artists, _, err := client.Favorites.GetArtists(context.Background(), 50)
	if err != nil {
		t.Fatalf("GetArtists: %v", err)
	}

	if len(artists) != 1 {
		t.Fatalf("len = %d, want 1", len(artists))
	}
	if artists[0].Name != "JAY Z" {
		t.Errorf("Name = %q, want %q", artists[0].Name, "JAY Z")
	}
}

func TestFavoritesHTTPError(t *testing.T) {
	server, client := testServerAndClient(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(401)
		w.Write([]byte(`{"errors": [{"title": "Unauthorized", "detail": "Invalid token"}]}`))
	})
	defer server.Close()

	err := client.Favorites.AddAlbum(context.Background(), "test")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !IsAuthError(err) {
		t.Errorf("expected auth error, got: %v", err)
	}
}
