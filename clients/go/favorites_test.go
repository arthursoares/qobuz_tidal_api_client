package qobuz

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func TestFavoritesAddAlbum(t *testing.T) {
	var capturedPath string
	var capturedBody string
	var capturedAppID string

	server, client := testServerAndClient(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		capturedAppID = r.Header.Get("X-App-Id")
		r.ParseForm()
		capturedBody = r.PostForm.Encode()
		w.WriteHeader(200)
		w.Write([]byte(`{"status": "success"}`))
	})
	defer server.Close()

	err := client.Favorites.AddAlbum(context.Background(), "album-123")
	if err != nil {
		t.Fatalf("AddAlbum: %v", err)
	}

	if capturedPath != "/favorite/create" {
		t.Errorf("path = %q, want /favorite/create", capturedPath)
	}
	if capturedAppID != "test-app-id" {
		t.Errorf("X-App-Id = %q, want test-app-id", capturedAppID)
	}
	if !strings.Contains(capturedBody, "album_ids=album-123") {
		t.Errorf("body should contain album_ids=album-123, got %q", capturedBody)
	}
	if !strings.Contains(capturedBody, "artist_ids=") {
		t.Errorf("body should contain artist_ids=, got %q", capturedBody)
	}
	if !strings.Contains(capturedBody, "track_ids=") {
		t.Errorf("body should contain track_ids=, got %q", capturedBody)
	}
}

func TestFavoritesAddAlbums(t *testing.T) {
	var capturedBody string

	server, client := testServerAndClient(func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		capturedBody = r.PostForm.Get("album_ids")
		w.WriteHeader(200)
		w.Write([]byte(`{"status": "success"}`))
	})
	defer server.Close()

	err := client.Favorites.AddAlbums(context.Background(), []string{"a1", "a2", "a3"})
	if err != nil {
		t.Fatalf("AddAlbums: %v", err)
	}

	if capturedBody != "a1,a2,a3" {
		t.Errorf("album_ids = %q, want %q", capturedBody, "a1,a2,a3")
	}
}

func TestFavoritesRemoveAlbum(t *testing.T) {
	var capturedPath string

	server, client := testServerAndClient(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		w.WriteHeader(200)
		w.Write([]byte(`{"status": "success"}`))
	})
	defer server.Close()

	err := client.Favorites.RemoveAlbum(context.Background(), "album-456")
	if err != nil {
		t.Fatalf("RemoveAlbum: %v", err)
	}

	if capturedPath != "/favorite/delete" {
		t.Errorf("path = %q, want /favorite/delete", capturedPath)
	}
}

func TestFavoritesGetIDs(t *testing.T) {
	server, client := testServerAndClient(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/favorite/getUserFavoriteIds" {
			t.Errorf("path = %q, want /favorite/getUserFavoriteIds", r.URL.Path)
		}
		resp := map[string]any{
			"albums":  []string{"a1", "a2"},
			"tracks":  []int{100, 200},
			"artists": []int{10},
			"labels":  []int{},
			"awards":  []int{},
		}
		w.WriteHeader(200)
		w.Write(mustJSON(resp))
	})
	defer server.Close()

	ids, err := client.Favorites.GetIDs(context.Background(), 5000)
	if err != nil {
		t.Fatalf("GetIDs: %v", err)
	}

	if len(ids.Albums) != 2 || ids.Albums[0] != "a1" {
		t.Errorf("Albums = %v, want [a1 a2]", ids.Albums)
	}
	if len(ids.Tracks) != 2 || ids.Tracks[0] != 100 {
		t.Errorf("Tracks = %v, want [100 200]", ids.Tracks)
	}
}

func TestFavoritesGetAlbums(t *testing.T) {
	server, client := testServerAndClient(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/favorite/getUserFavorites" {
			t.Errorf("path = %q, want /favorite/getUserFavorites", r.URL.Path)
		}
		if r.URL.Query().Get("type") != "albums" {
			t.Errorf("type = %q, want albums", r.URL.Query().Get("type"))
		}

		resp := map[string]any{
			"albums": map[string]any{
				"items": []map[string]any{
					{
						"id":                    "album-1",
						"title":                 "Test Album",
						"artist":                map[string]any{"id": 1, "name": "Artist 1"},
						"artists":               []any{},
						"image":                 map[string]any{},
						"duration":              3600,
						"tracks_count":          10,
						"maximum_bit_depth":     24,
						"maximum_sampling_rate": 96.0,
						"maximum_channel_count": 2,
						"streamable":            true,
						"downloadable":          true,
						"hires":                 true,
						"hires_streamable":      true,
					},
				},
				"total":  1,
				"limit":  50,
				"offset": 0,
			},
		}
		w.WriteHeader(200)
		json.NewEncoder(w).Encode(resp)
	})
	defer server.Close()

	albums, err := client.Favorites.GetAlbums(context.Background(), 50, 0)
	if err != nil {
		t.Fatalf("GetAlbums: %v", err)
	}

	if len(albums.Items) != 1 {
		t.Fatalf("Items length = %d, want 1", len(albums.Items))
	}
	if albums.Items[0].ID != "album-1" {
		t.Errorf("Albums[0].ID = %q, want %q", albums.Items[0].ID, "album-1")
	}
	if albums.Total != 1 {
		t.Errorf("Total = %d, want 1", albums.Total)
	}
}

func TestFavoritesAddTrack(t *testing.T) {
	var capturedTrackIDs string

	server, client := testServerAndClient(func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		capturedTrackIDs = r.PostForm.Get("track_ids")
		w.WriteHeader(200)
		w.Write([]byte(`{"status": "success"}`))
	})
	defer server.Close()

	err := client.Favorites.AddTrack(context.Background(), "12345")
	if err != nil {
		t.Fatalf("AddTrack: %v", err)
	}

	if capturedTrackIDs != "12345" {
		t.Errorf("track_ids = %q, want %q", capturedTrackIDs, "12345")
	}
}

func TestFavoritesHTTPError(t *testing.T) {
	server, client := testServerAndClient(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(401)
		w.Write([]byte(`{"message": "Session authentication is required"}`))
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
