package qobuz

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"
)

func TestCatalogGetAlbum(t *testing.T) {
	server, client := testServerAndClient(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/album/get" {
			t.Errorf("path = %q, want /album/get", r.URL.Path)
		}
		if r.URL.Query().Get("album_id") != "p0d55tt7gv3lc" {
			t.Errorf("album_id = %q, want p0d55tt7gv3lc", r.URL.Query().Get("album_id"))
		}

		resp := map[string]interface{}{
			"id":                    "p0d55tt7gv3lc",
			"title":                 "Virgin Lake",
			"version":              nil,
			"maximum_bit_depth":     24,
			"maximum_sampling_rate": 44.1,
			"maximum_channel_count": 2,
			"duration":              3487,
			"tracks_count":          14,
			"streamable":            true,
			"downloadable":          true,
			"hires":                 true,
			"hires_streamable":      true,
			"artist":                map[string]interface{}{"id": 11162390, "name": "Philine Sonny"},
			"artists":               []interface{}{},
			"image":                 map[string]interface{}{},
		}
		w.WriteHeader(200)
		json.NewEncoder(w).Encode(resp)
	})
	defer server.Close()

	album, err := client.Catalog.GetAlbum(context.Background(), "p0d55tt7gv3lc")
	if err != nil {
		t.Fatalf("GetAlbum: %v", err)
	}

	if album.ID != "p0d55tt7gv3lc" {
		t.Errorf("ID = %q, want %q", album.ID, "p0d55tt7gv3lc")
	}
	if album.Title != "Virgin Lake" {
		t.Errorf("Title = %q, want %q", album.Title, "Virgin Lake")
	}
	if album.MaximumBitDepth != 24 {
		t.Errorf("MaximumBitDepth = %d, want 24", album.MaximumBitDepth)
	}
}

func TestCatalogSearchAlbums(t *testing.T) {
	server, client := testServerAndClient(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/album/search" {
			t.Errorf("path = %q, want /album/search", r.URL.Path)
		}
		if r.URL.Query().Get("query") != "radiohead" {
			t.Errorf("query = %q, want radiohead", r.URL.Query().Get("query"))
		}

		resp := map[string]interface{}{
			"albums": map[string]interface{}{
				"items": []interface{}{
					map[string]interface{}{"id": "a1", "title": "OK Computer"},
					map[string]interface{}{"id": "a2", "title": "Kid A"},
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

	result, err := client.Catalog.SearchAlbums(context.Background(), "radiohead", 50, 0)
	if err != nil {
		t.Fatalf("SearchAlbums: %v", err)
	}

	if len(result.Items) != 2 {
		t.Errorf("Items length = %d, want 2", len(result.Items))
	}
	if result.Total == nil || *result.Total != 2 {
		t.Error("Total should be 2")
	}
}

func TestCatalogGetTrack(t *testing.T) {
	server, client := testServerAndClient(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/track/get" {
			t.Errorf("path = %q, want /track/get", r.URL.Path)
		}

		resp := map[string]interface{}{
			"id":               33967376,
			"title":            "Test Track",
			"duration":         245,
			"track_number":     3,
			"media_number":     1,
			"parental_warning": false,
			"performer":        map[string]interface{}{"id": 1, "name": "Artist"},
			"album":            map[string]interface{}{"id": "abc", "title": "Album"},
			"audio_info":       map[string]interface{}{"maximum_bit_depth": 24, "maximum_channel_count": 2, "maximum_sampling_rate": 96.0},
			"rights":           map[string]interface{}{"streamable": true, "downloadable": true},
		}
		w.WriteHeader(200)
		json.NewEncoder(w).Encode(resp)
	})
	defer server.Close()

	track, err := client.Catalog.GetTrack(context.Background(), 33967376)
	if err != nil {
		t.Fatalf("GetTrack: %v", err)
	}

	if track.ID != 33967376 {
		t.Errorf("ID = %d, want 33967376", track.ID)
	}
	if track.Title != "Test Track" {
		t.Errorf("Title = %q, want %q", track.Title, "Test Track")
	}
}

func TestCatalogGetTracks(t *testing.T) {
	var capturedMethod string
	var capturedBody []byte

	server, client := testServerAndClient(func(w http.ResponseWriter, r *http.Request) {
		capturedMethod = r.Method
		if r.URL.Path != "/track/getList" {
			t.Errorf("path = %q, want /track/getList", r.URL.Path)
		}

		var err error
		capturedBody, err = io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}

		resp := map[string]interface{}{
			"tracks": map[string]interface{}{
				"items": []interface{}{
					map[string]interface{}{
						"id":         100,
						"title":      "Track 1",
						"duration":   200,
						"performer":  map[string]interface{}{"id": 1, "name": "A"},
						"album":      map[string]interface{}{"id": "x", "title": "Y"},
						"audio_info": map[string]interface{}{},
						"rights":     map[string]interface{}{},
					},
					map[string]interface{}{
						"id":         200,
						"title":      "Track 2",
						"duration":   300,
						"performer":  map[string]interface{}{"id": 2, "name": "B"},
						"album":      map[string]interface{}{"id": "x", "title": "Y"},
						"audio_info": map[string]interface{}{},
						"rights":     map[string]interface{}{},
					},
				},
			},
		}
		w.WriteHeader(200)
		json.NewEncoder(w).Encode(resp)
	})
	defer server.Close()

	tracks, err := client.Catalog.GetTracks(context.Background(), []int{100, 200})
	if err != nil {
		t.Fatalf("GetTracks: %v", err)
	}

	if capturedMethod != "POST" {
		t.Errorf("method = %q, want POST", capturedMethod)
	}

	// Verify JSON body contains tracks_id
	var body map[string]interface{}
	if err := json.Unmarshal(capturedBody, &body); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	if _, ok := body["tracks_id"]; !ok {
		t.Error("body should contain tracks_id")
	}

	if len(tracks) != 2 {
		t.Fatalf("tracks length = %d, want 2", len(tracks))
	}
	if tracks[0].ID != 100 {
		t.Errorf("tracks[0].ID = %d, want 100", tracks[0].ID)
	}
}

func TestCatalogGetTracksEmpty(t *testing.T) {
	// GetTracks with empty slice should return nil without making a request
	requestMade := false
	server, client := testServerAndClient(func(w http.ResponseWriter, r *http.Request) {
		requestMade = true
		w.WriteHeader(200)
	})
	defer server.Close()

	tracks, err := client.Catalog.GetTracks(context.Background(), []int{})
	if err != nil {
		t.Fatalf("GetTracks: %v", err)
	}
	if tracks != nil {
		t.Errorf("expected nil, got %v", tracks)
	}
	if requestMade {
		t.Error("no HTTP request should be made for empty track IDs")
	}
}

func TestCatalogSuggestAlbum(t *testing.T) {
	server, client := testServerAndClient(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/album/suggest" {
			t.Errorf("path = %q, want /album/suggest", r.URL.Path)
		}

		resp := map[string]interface{}{
			"albums": map[string]interface{}{
				"items": []interface{}{
					map[string]interface{}{
						"id":                    "s1",
						"title":                 "Similar Album",
						"artist":                map[string]interface{}{"id": 5, "name": "Similar Artist"},
						"artists":               []interface{}{},
						"image":                 map[string]interface{}{},
						"duration":              1800,
						"tracks_count":          8,
						"maximum_bit_depth":     16,
						"maximum_sampling_rate": 44.1,
						"maximum_channel_count": 2,
						"streamable":            true,
						"downloadable":          true,
						"hires":                 false,
						"hires_streamable":      false,
					},
				},
			},
		}
		w.WriteHeader(200)
		json.NewEncoder(w).Encode(resp)
	})
	defer server.Close()

	albums, err := client.Catalog.SuggestAlbum(context.Background(), "test-id")
	if err != nil {
		t.Fatalf("SuggestAlbum: %v", err)
	}

	if len(albums) != 1 {
		t.Fatalf("albums length = %d, want 1", len(albums))
	}
	if albums[0].Title != "Similar Album" {
		t.Errorf("Title = %q, want %q", albums[0].Title, "Similar Album")
	}
}

func TestCatalogGetArtistReleases(t *testing.T) {
	server, client := testServerAndClient(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/artist/getReleasesList" {
			t.Errorf("path = %q, want /artist/getReleasesList", r.URL.Path)
		}
		if r.URL.Query().Get("artist_id") != "12345" {
			t.Errorf("artist_id = %q, want 12345", r.URL.Query().Get("artist_id"))
		}

		resp := map[string]interface{}{
			"has_more": true,
			"items":    []interface{}{map[string]interface{}{"id": "r1", "title": "Release 1"}},
		}
		w.WriteHeader(200)
		json.NewEncoder(w).Encode(resp)
	})
	defer server.Close()

	result, err := client.Catalog.GetArtistReleases(context.Background(), 12345, nil)
	if err != nil {
		t.Fatalf("GetArtistReleases: %v", err)
	}

	if len(result.Items) != 1 {
		t.Errorf("Items length = %d, want 1", len(result.Items))
	}
	if !result.HasMore {
		t.Error("HasMore should be true")
	}
}

func TestCatalogSearchArtists(t *testing.T) {
	server, client := testServerAndClient(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/artist/search" {
			t.Errorf("path = %q, want /artist/search", r.URL.Path)
		}

		resp := map[string]interface{}{
			"artists": map[string]interface{}{
				"items":  []interface{}{map[string]interface{}{"id": 1, "name": "Test Artist"}},
				"total":  1,
				"limit":  50,
				"offset": 0,
			},
		}
		w.WriteHeader(200)
		json.NewEncoder(w).Encode(resp)
	})
	defer server.Close()

	result, err := client.Catalog.SearchArtists(context.Background(), "test", 50, 0)
	if err != nil {
		t.Fatalf("SearchArtists: %v", err)
	}

	if len(result.Items) != 1 {
		t.Errorf("Items length = %d, want 1", len(result.Items))
	}
}

func TestCatalogGetAlbumStory(t *testing.T) {
	server, client := testServerAndClient(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/album/story" {
			t.Errorf("path = %q, want /album/story", r.URL.Path)
		}
		if r.URL.Query().Get("album_id") != "p0d55tt7gv3lc" {
			t.Errorf("album_id = %q, want p0d55tt7gv3lc", r.URL.Query().Get("album_id"))
		}
		if r.URL.Query().Get("offset") != "0" {
			t.Errorf("offset = %q, want 0", r.URL.Query().Get("offset"))
		}
		if r.URL.Query().Get("limit") != "10" {
			t.Errorf("limit = %q, want 10", r.URL.Query().Get("limit"))
		}

		resp := map[string]interface{}{
			"story": "Editorial content about this album.",
		}
		w.WriteHeader(200)
		json.NewEncoder(w).Encode(resp)
	})
	defer server.Close()

	data, err := client.Catalog.GetAlbumStory(context.Background(), "p0d55tt7gv3lc")
	if err != nil {
		t.Fatalf("GetAlbumStory: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if result["story"] != "Editorial content about this album." {
		t.Errorf("story = %v, want editorial content", result["story"])
	}
}

func TestCatalogSearchTracks(t *testing.T) {
	server, client := testServerAndClient(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/track/search" {
			t.Errorf("path = %q, want /track/search", r.URL.Path)
		}

		resp := map[string]interface{}{
			"tracks": map[string]interface{}{
				"items":  []interface{}{},
				"total":  0,
				"limit":  50,
				"offset": 0,
			},
		}
		w.WriteHeader(200)
		json.NewEncoder(w).Encode(resp)
	})
	defer server.Close()

	result, err := client.Catalog.SearchTracks(context.Background(), "nonexistent", 50, 0)
	if err != nil {
		t.Fatalf("SearchTracks: %v", err)
	}

	if len(result.Items) != 0 {
		t.Errorf("Items length = %d, want 0", len(result.Items))
	}
}
