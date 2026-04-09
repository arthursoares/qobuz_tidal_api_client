package tidal

import (
	"context"
	"net/http"
	"testing"
)

func TestSearchAlbums(t *testing.T) {
	server, client := testServerAndClient(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("method = %q, want GET", r.Method)
		}
		// The query is URL-encoded in the path
		expectedPath := "/searchResults/radiohead/relationships/albums"
		if r.URL.Path != expectedPath {
			t.Errorf("path = %q, want %q", r.URL.Path, expectedPath)
		}
		if r.URL.Query().Get("include") != "albums" {
			t.Errorf("include = %q, want albums", r.URL.Query().Get("include"))
		}

		identifiers := []map[string]string{
			{"type": "albums", "id": "a1"},
			{"type": "albums", "id": "a2"},
		}
		included := []map[string]any{
			jsonAPIAlbum("a1", "OK Computer", 12, []string{"LOSSLESS"}),
			jsonAPIAlbum("a2", "Kid A", 10, []string{"HIRES_LOSSLESS"}),
		}
		doc := jsonAPIRelationshipDoc(identifiers, included, "")
		w.WriteHeader(200)
		w.Write(mustJSON(doc))
	})
	defer server.Close()

	albums, cursor, err := client.Search.Albums(context.Background(), "radiohead", 20)
	if err != nil {
		t.Fatalf("Search.Albums: %v", err)
	}

	if len(albums) != 2 {
		t.Fatalf("len = %d, want 2", len(albums))
	}
	if albums[0].Title != "OK Computer" {
		t.Errorf("albums[0].Title = %q, want %q", albums[0].Title, "OK Computer")
	}
	if albums[1].Title != "Kid A" {
		t.Errorf("albums[1].Title = %q, want %q", albums[1].Title, "Kid A")
	}
	if cursor != "" {
		t.Errorf("cursor = %q, want empty", cursor)
	}
}

func TestSearchTracks(t *testing.T) {
	server, client := testServerAndClient(func(w http.ResponseWriter, r *http.Request) {
		identifiers := []map[string]string{
			{"type": "tracks", "id": "t1"},
		}
		included := []map[string]any{
			jsonAPITrack("t1", "Paranoid Android", "GBAYE9700100", false),
		}
		doc := jsonAPIRelationshipDoc(identifiers, included, "/searchResults/test/relationships/tracks?page[cursor]=next123")
		w.WriteHeader(200)
		w.Write(mustJSON(doc))
	})
	defer server.Close()

	tracks, cursor, err := client.Search.Tracks(context.Background(), "paranoid android", 20)
	if err != nil {
		t.Fatalf("Search.Tracks: %v", err)
	}

	if len(tracks) != 1 {
		t.Fatalf("len = %d, want 1", len(tracks))
	}
	if tracks[0].Title != "Paranoid Android" {
		t.Errorf("Title = %q, want %q", tracks[0].Title, "Paranoid Android")
	}
	if cursor != "next123" {
		t.Errorf("cursor = %q, want %q", cursor, "next123")
	}
}

func TestSearchArtists(t *testing.T) {
	server, client := testServerAndClient(func(w http.ResponseWriter, r *http.Request) {
		identifiers := []map[string]string{
			{"type": "artists", "id": "3175"},
		}
		included := []map[string]any{
			jsonAPIArtist("3175", "Radiohead", 0.92),
		}
		doc := jsonAPIRelationshipDoc(identifiers, included, "")
		w.WriteHeader(200)
		w.Write(mustJSON(doc))
	})
	defer server.Close()

	artists, cursor, err := client.Search.Artists(context.Background(), "radiohead", 10)
	if err != nil {
		t.Fatalf("Search.Artists: %v", err)
	}

	if len(artists) != 1 {
		t.Fatalf("len = %d, want 1", len(artists))
	}
	if artists[0].Name != "Radiohead" {
		t.Errorf("Name = %q, want %q", artists[0].Name, "Radiohead")
	}
	if artists[0].Popularity != 0.92 {
		t.Errorf("Popularity = %f, want 0.92", artists[0].Popularity)
	}
	if cursor != "" {
		t.Errorf("cursor = %q, want empty", cursor)
	}
}

func TestSearchEmpty(t *testing.T) {
	server, client := testServerAndClient(func(w http.ResponseWriter, r *http.Request) {
		doc := jsonAPIRelationshipDoc([]map[string]string{}, nil, "")
		w.WriteHeader(200)
		w.Write(mustJSON(doc))
	})
	defer server.Close()

	albums, _, err := client.Search.Albums(context.Background(), "xyznonexistent", 20)
	if err != nil {
		t.Fatalf("Search.Albums: %v", err)
	}

	if len(albums) != 0 {
		t.Errorf("len = %d, want 0", len(albums))
	}
}
