package tidal

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"
)

func TestPlaylistsCreate(t *testing.T) {
	var capturedMethod string
	var capturedPath string
	var capturedBody map[string]any

	server, client := testServerAndClient(func(w http.ResponseWriter, r *http.Request) {
		capturedMethod = r.Method
		capturedPath = r.URL.Path
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &capturedBody)

		// Return created playlist
		doc := jsonAPISingleDoc(jsonAPIPlaylist("pl-new", "My Playlist", "A test playlist", 0))
		w.WriteHeader(201)
		w.Write(mustJSON(doc))
	})
	defer server.Close()

	pl, err := client.Playlists.Create(context.Background(), "My Playlist", "A test playlist", true)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if capturedMethod != "POST" {
		t.Errorf("method = %q, want POST", capturedMethod)
	}
	if capturedPath != "/playlists" {
		t.Errorf("path = %q, want /playlists", capturedPath)
	}
	if pl.ID != "pl-new" {
		t.Errorf("ID = %q, want %q", pl.ID, "pl-new")
	}
	if pl.Name != "My Playlist" {
		t.Errorf("Name = %q, want %q", pl.Name, "My Playlist")
	}

	// Check JSON:API payload structure
	data := capturedBody["data"].(map[string]any)
	if data["type"] != "playlists" {
		t.Errorf("type = %v, want playlists", data["type"])
	}
	attrs := data["attributes"].(map[string]any)
	if attrs["name"] != "My Playlist" {
		t.Errorf("name = %v, want %q", attrs["name"], "My Playlist")
	}
	if attrs["accessType"] != "PUBLIC" {
		t.Errorf("accessType = %v, want PUBLIC", attrs["accessType"])
	}
}

func TestPlaylistsDelete(t *testing.T) {
	var capturedMethod string
	var capturedPath string

	server, client := testServerAndClient(func(w http.ResponseWriter, r *http.Request) {
		capturedMethod = r.Method
		capturedPath = r.URL.Path
		w.WriteHeader(204)
	})
	defer server.Close()

	err := client.Playlists.Delete(context.Background(), "pl-123")
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}

	if capturedMethod != "DELETE" {
		t.Errorf("method = %q, want DELETE", capturedMethod)
	}
	if capturedPath != "/playlists/pl-123" {
		t.Errorf("path = %q, want /playlists/pl-123", capturedPath)
	}
}

func TestPlaylistsUpdate(t *testing.T) {
	var capturedBody map[string]any

	server, client := testServerAndClient(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PATCH" {
			t.Errorf("method = %q, want PATCH", r.Method)
		}
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &capturedBody)

		doc := jsonAPISingleDoc(jsonAPIPlaylist("pl-123", "Renamed Playlist", "", 10))
		w.WriteHeader(200)
		w.Write(mustJSON(doc))
	})
	defer server.Close()

	name := "Renamed Playlist"
	pl, err := client.Playlists.Update(context.Background(), "pl-123", &PlaylistUpdateOptions{Name: &name})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}

	if pl.Name != "Renamed Playlist" {
		t.Errorf("Name = %q, want %q", pl.Name, "Renamed Playlist")
	}

	data := capturedBody["data"].(map[string]any)
	if data["id"] != "pl-123" {
		t.Errorf("id = %v, want pl-123", data["id"])
	}
	attrs := data["attributes"].(map[string]any)
	if attrs["name"] != "Renamed Playlist" {
		t.Errorf("name = %v, want Renamed Playlist", attrs["name"])
	}
}

func TestPlaylistsGet(t *testing.T) {
	server, client := testServerAndClient(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/playlists/pl-123" {
			t.Errorf("path = %q, want /playlists/pl-123", r.URL.Path)
		}

		doc := jsonAPISingleDoc(jsonAPIPlaylist("pl-123", "My Playlist", "Great tunes", 42))
		w.WriteHeader(200)
		w.Write(mustJSON(doc))
	})
	defer server.Close()

	pl, err := client.Playlists.Get(context.Background(), "pl-123")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}

	if pl.ID != "pl-123" {
		t.Errorf("ID = %q, want %q", pl.ID, "pl-123")
	}
	if pl.Name != "My Playlist" {
		t.Errorf("Name = %q, want %q", pl.Name, "My Playlist")
	}
	if pl.NumberOfItems != 42 {
		t.Errorf("NumberOfItems = %d, want 42", pl.NumberOfItems)
	}
}

func TestPlaylistsList(t *testing.T) {
	server, client := testServerAndClient(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/playlists" {
			t.Errorf("path = %q, want /playlists", r.URL.Path)
		}
		if r.URL.Query().Get("filter[owners.id]") != "me" {
			t.Errorf("filter[owners.id] = %q, want me", r.URL.Query().Get("filter[owners.id]"))
		}

		resources := []map[string]any{
			jsonAPIPlaylist("pl-1", "Playlist 1", "", 10),
			jsonAPIPlaylist("pl-2", "Playlist 2", "Desc", 20),
		}
		doc := jsonAPIMultiDoc(resources, "")
		w.WriteHeader(200)
		w.Write(mustJSON(doc))
	})
	defer server.Close()

	playlists, _, err := client.Playlists.List(context.Background(), 100)
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	if len(playlists) != 2 {
		t.Fatalf("len = %d, want 2", len(playlists))
	}
	if playlists[0].Name != "Playlist 1" {
		t.Errorf("Name = %q, want %q", playlists[0].Name, "Playlist 1")
	}
}

func TestPlaylistsAddTracks(t *testing.T) {
	var capturedPath string
	var capturedBody map[string]any

	server, client := testServerAndClient(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &capturedBody)
		w.WriteHeader(204)
	})
	defer server.Close()

	err := client.Playlists.AddTracks(context.Background(), "pl-123", []string{"t1", "t2"})
	if err != nil {
		t.Fatalf("AddTracks: %v", err)
	}

	if capturedPath != "/playlists/pl-123/relationships/items" {
		t.Errorf("path = %q, want /playlists/pl-123/relationships/items", capturedPath)
	}

	data := capturedBody["data"].([]any)
	if len(data) != 2 {
		t.Fatalf("data len = %d, want 2", len(data))
	}
	item := data[0].(map[string]any)
	if item["type"] != "tracks" || item["id"] != "t1" {
		t.Errorf("data[0] = %v, want type=tracks id=t1", item)
	}

	// Verify required meta.positionBefore is present
	meta, ok := capturedBody["meta"].(map[string]any)
	if !ok {
		t.Fatal("meta is missing or not an object")
	}
	if pos, _ := meta["positionBefore"].(string); pos != "-" {
		t.Errorf("meta.positionBefore = %q, want %q (append)", pos, "-")
	}
}

func TestPlaylistsAddTracksAt(t *testing.T) {
	var capturedBody map[string]any
	server, client := testServerAndClient(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &capturedBody)
		w.WriteHeader(204)
	})
	defer server.Close()

	err := client.Playlists.AddTracksAt(context.Background(), "pl-123", []string{"t1"}, "item-99")
	if err != nil {
		t.Fatalf("AddTracksAt: %v", err)
	}
	meta := capturedBody["meta"].(map[string]any)
	if meta["positionBefore"] != "item-99" {
		t.Errorf("positionBefore = %v, want item-99", meta["positionBefore"])
	}
}

func TestPlaylistsRemoveTracks(t *testing.T) {
	var capturedMethod string
	var capturedPath string
	var capturedBody map[string]any

	server, client := testServerAndClient(func(w http.ResponseWriter, r *http.Request) {
		capturedMethod = r.Method
		capturedPath = r.URL.Path
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &capturedBody)
		w.WriteHeader(204)
	})
	defer server.Close()

	err := client.Playlists.RemoveTracks(context.Background(), "pl-123", []string{"item-1", "item-2"})
	if err != nil {
		t.Fatalf("RemoveTracks: %v", err)
	}

	if capturedMethod != "DELETE" {
		t.Errorf("method = %q, want DELETE", capturedMethod)
	}
	if capturedPath != "/playlists/pl-123/relationships/items" {
		t.Errorf("path = %q, want correct path", capturedPath)
	}

	// Each entry must include meta.itemId per the spec
	data := capturedBody["data"].([]any)
	if len(data) != 2 {
		t.Fatalf("data len = %d, want 2", len(data))
	}
	for i, raw := range data {
		entry := raw.(map[string]any)
		if entry["type"] != "tracks" {
			t.Errorf("data[%d].type = %v, want tracks", i, entry["type"])
		}
		meta, ok := entry["meta"].(map[string]any)
		if !ok {
			t.Fatalf("data[%d].meta is missing", i)
		}
		if meta["itemId"] == nil || meta["itemId"] == "" {
			t.Errorf("data[%d].meta.itemId is missing", i)
		}
	}
}
