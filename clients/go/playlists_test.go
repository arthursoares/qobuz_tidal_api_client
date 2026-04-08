package qobuz

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
)

func TestPlaylistsCreate(t *testing.T) {
	var capturedPath string
	var capturedName string
	var capturedIsPublic string

	server, client := testServerAndClient(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		r.ParseForm()
		capturedName = r.PostForm.Get("name")
		capturedIsPublic = r.PostForm.Get("is_public")

		resp := map[string]any{
			"id":               61997651,
			"name":             "My New Playlist",
			"description":      "A test playlist",
			"tracks_count":     0,
			"users_count":      0,
			"duration":         0,
			"is_public":        true,
			"is_collaborative": false,
			"public_at":        false,
			"created_at":       1775635602,
			"updated_at":       1775635602,
			"owner":            map[string]any{"id": 1, "name": "testuser"},
		}
		w.WriteHeader(200)
		json.NewEncoder(w).Encode(resp)
	})
	defer server.Close()

	p, err := client.Playlists.Create(context.Background(), "My New Playlist", "A test playlist", true, false)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if capturedPath != "/playlist/create" {
		t.Errorf("path = %q, want /playlist/create", capturedPath)
	}
	if capturedName != "My New Playlist" {
		t.Errorf("name = %q, want %q", capturedName, "My New Playlist")
	}
	if capturedIsPublic != "true" {
		t.Errorf("is_public = %q, want %q", capturedIsPublic, "true")
	}
	if p.ID != 61997651 {
		t.Errorf("ID = %d, want 61997651", p.ID)
	}
	if p.Name != "My New Playlist" {
		t.Errorf("Name = %q, want %q", p.Name, "My New Playlist")
	}
}

func TestPlaylistsAddTracksBatching(t *testing.T) {
	requestCount := 0
	var capturedTrackIDs []string

	server, client := testServerAndClient(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		r.ParseForm()
		capturedTrackIDs = append(capturedTrackIDs, r.PostForm.Get("track_ids"))
		w.WriteHeader(200)
		w.Write([]byte(`{"status": "success"}`))
	})
	defer server.Close()

	// Create 75 track IDs to test batching (should be 2 batches: 50 + 25)
	trackIDs := make([]string, 75)
	for i := range trackIDs {
		trackIDs[i] = "track-" + string(rune('A'+i%26))
	}

	err := client.Playlists.AddTracks(context.Background(), 123, trackIDs, true)
	if err != nil {
		t.Fatalf("AddTracks: %v", err)
	}

	if requestCount != 2 {
		t.Errorf("request count = %d, want 2", requestCount)
	}
}

func TestPlaylistsDelete(t *testing.T) {
	var capturedPath string
	var capturedPlaylistID string

	server, client := testServerAndClient(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		r.ParseForm()
		capturedPlaylistID = r.PostForm.Get("playlist_id")
		w.WriteHeader(200)
		w.Write([]byte(`{"status": "success"}`))
	})
	defer server.Close()

	err := client.Playlists.Delete(context.Background(), 12345)
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}

	if capturedPath != "/playlist/delete" {
		t.Errorf("path = %q, want /playlist/delete", capturedPath)
	}
	if capturedPlaylistID != "12345" {
		t.Errorf("playlist_id = %q, want %q", capturedPlaylistID, "12345")
	}
}

func TestPlaylistsUpdate(t *testing.T) {
	var capturedName string
	var capturedDesc string

	server, client := testServerAndClient(func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		capturedName = r.PostForm.Get("name")
		capturedDesc = r.PostForm.Get("description")

		resp := map[string]any{
			"id":               100,
			"name":             "Updated Name",
			"description":      "Updated Desc",
			"tracks_count":     5,
			"users_count":      1,
			"duration":         600,
			"is_public":        false,
			"is_collaborative": false,
			"public_at":        false,
			"created_at":       1000,
			"updated_at":       2000,
			"owner":            map[string]any{"id": 1, "name": "user"},
		}
		w.WriteHeader(200)
		json.NewEncoder(w).Encode(resp)
	})
	defer server.Close()

	name := "Updated Name"
	desc := "Updated Desc"
	p, err := client.Playlists.Update(context.Background(), 100, &PlaylistUpdateOptions{
		Name:        &name,
		Description: &desc,
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}

	if capturedName != "Updated Name" {
		t.Errorf("name = %q, want %q", capturedName, "Updated Name")
	}
	if capturedDesc != "Updated Desc" {
		t.Errorf("description = %q, want %q", capturedDesc, "Updated Desc")
	}
	if p.Name != "Updated Name" {
		t.Errorf("returned Name = %q, want %q", p.Name, "Updated Name")
	}
}

func TestPlaylistsList(t *testing.T) {
	server, client := testServerAndClient(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/playlist/getUserPlaylists" {
			t.Errorf("path = %q, want /playlist/getUserPlaylists", r.URL.Path)
		}
		resp := map[string]any{
			"playlists": map[string]any{
				"items":  []any{map[string]any{"id": 1, "name": "pl1"}},
				"total":  1,
				"limit":  500,
				"offset": 0,
			},
		}
		w.WriteHeader(200)
		json.NewEncoder(w).Encode(resp)
	})
	defer server.Close()

	result, err := client.Playlists.List(context.Background(), 500)
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	if len(result.Items) != 1 {
		t.Errorf("Items length = %d, want 1", len(result.Items))
	}
}

func TestPlaylistsGetWithTracks(t *testing.T) {
	server, client := testServerAndClient(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/playlist/get" {
			t.Errorf("path = %q, want /playlist/get", r.URL.Path)
		}
		if r.URL.Query().Get("extra") != "tracks" {
			t.Errorf("extra = %q, want tracks", r.URL.Query().Get("extra"))
		}

		resp := map[string]any{
			"id":               42,
			"name":             "My Playlist",
			"description":      "desc",
			"tracks_count":     2,
			"users_count":      1,
			"duration":         500,
			"is_public":        true,
			"is_collaborative": false,
			"public_at":        false,
			"created_at":       1000,
			"updated_at":       2000,
			"owner":            map[string]any{"id": 1, "name": "user"},
			"tracks": map[string]any{
				"items": []any{
					map[string]any{
						"id":         100,
						"title":      "Track A",
						"duration":   200,
						"performer":  map[string]any{"id": 1, "name": "Artist A"},
						"album":      map[string]any{"id": "x", "title": "Album X"},
						"audio_info": map[string]any{},
						"rights":     map[string]any{},
					},
					map[string]any{
						"id":         200,
						"title":      "Track B",
						"duration":   300,
						"performer":  map[string]any{"id": 2, "name": "Artist B"},
						"album":      map[string]any{"id": "y", "title": "Album Y"},
						"audio_info": map[string]any{},
						"rights":     map[string]any{},
					},
				},
				"total":  2,
				"limit":  50,
				"offset": 0,
			},
		}
		w.WriteHeader(200)
		json.NewEncoder(w).Encode(resp)
	})
	defer server.Close()

	p, err := client.Playlists.Get(context.Background(), 42, nil)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}

	if p.Name != "My Playlist" {
		t.Errorf("Name = %q, want %q", p.Name, "My Playlist")
	}
	if p.Tracks == nil {
		t.Fatal("Tracks should not be nil")
	}
	if len(p.Tracks.Items) != 2 {
		t.Fatalf("Tracks.Items length = %d, want 2", len(p.Tracks.Items))
	}
	if p.Tracks.Items[0].Title != "Track A" {
		t.Errorf("Tracks.Items[0].Title = %q, want %q", p.Tracks.Items[0].Title, "Track A")
	}
	if p.Tracks.Total != 2 {
		t.Errorf("Tracks.Total = %d, want 2", p.Tracks.Total)
	}
}

func TestPlaylistsGetWithOptions(t *testing.T) {
	server, client := testServerAndClient(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("extra") != "track_ids,getSimilarPlaylists" {
			t.Errorf("extra = %q, want track_ids,getSimilarPlaylists", r.URL.Query().Get("extra"))
		}
		if r.URL.Query().Get("offset") != "10" {
			t.Errorf("offset = %q, want 10", r.URL.Query().Get("offset"))
		}
		if r.URL.Query().Get("limit") != "25" {
			t.Errorf("limit = %q, want 25", r.URL.Query().Get("limit"))
		}

		resp := map[string]any{
			"id":               42,
			"name":             "Test",
			"description":      "",
			"tracks_count":     0,
			"users_count":      0,
			"duration":         0,
			"is_public":        false,
			"is_collaborative": false,
			"public_at":        false,
			"created_at":       1000,
			"updated_at":       2000,
			"owner":            map[string]any{"id": 1, "name": "user"},
		}
		w.WriteHeader(200)
		json.NewEncoder(w).Encode(resp)
	})
	defer server.Close()

	_, err := client.Playlists.Get(context.Background(), 42, &PlaylistGetOptions{
		Extra:  "track_ids,getSimilarPlaylists",
		Offset: 10,
		Limit:  25,
	})
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
}

func TestPlaylistsSearch(t *testing.T) {
	server, client := testServerAndClient(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/playlist/search" {
			t.Errorf("path = %q, want /playlist/search", r.URL.Path)
		}
		if r.URL.Query().Get("query") != "jazz" {
			t.Errorf("query = %q, want jazz", r.URL.Query().Get("query"))
		}
		resp := map[string]any{
			"playlists": map[string]any{
				"items":  []any{},
				"total":  0,
				"limit":  50,
				"offset": 0,
			},
		}
		w.WriteHeader(200)
		json.NewEncoder(w).Encode(resp)
	})
	defer server.Close()

	result, err := client.Playlists.Search(context.Background(), "jazz", 50, 0)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}

	if len(result.Items) != 0 {
		t.Errorf("Items length = %d, want 0", len(result.Items))
	}
}
