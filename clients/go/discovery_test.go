package qobuz

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
)

func TestDiscoveryListGenres(t *testing.T) {
	server, client := testServerAndClient(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/genre/list" {
			t.Errorf("path = %q, want /genre/list", r.URL.Path)
		}

		resp := map[string]interface{}{
			"genres": map[string]interface{}{
				"items": []interface{}{
					map[string]interface{}{"id": 112, "color": "#5eabc1", "name": "Pop/Rock", "path": []int{112}, "slug": "pop-rock"},
					map[string]interface{}{"id": 113, "color": "#ff0000", "name": "Alternative", "path": []int{112, 113}, "slug": "alternative"},
				},
			},
		}
		w.WriteHeader(200)
		json.NewEncoder(w).Encode(resp)
	})
	defer server.Close()

	genres, err := client.Discovery.ListGenres(context.Background())
	if err != nil {
		t.Fatalf("ListGenres: %v", err)
	}

	if len(genres) != 2 {
		t.Fatalf("genres length = %d, want 2", len(genres))
	}
	if genres[0].Name != "Pop/Rock" {
		t.Errorf("genres[0].Name = %q, want %q", genres[0].Name, "Pop/Rock")
	}
	if genres[0].Slug != "pop-rock" {
		t.Errorf("genres[0].Slug = %q, want %q", genres[0].Slug, "pop-rock")
	}
}

func TestDiscoveryGetIndex(t *testing.T) {
	server, client := testServerAndClient(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/discover/index" {
			t.Errorf("path = %q, want /discover/index", r.URL.Path)
		}
		if r.URL.Query().Get("genre_ids") != "112" {
			t.Errorf("genre_ids = %q, want 112", r.URL.Query().Get("genre_ids"))
		}

		resp := map[string]interface{}{
			"containers": map[string]interface{}{
				"new_releases": map[string]interface{}{"id": "newReleases"},
				"playlists":    map[string]interface{}{"id": "playlists"},
			},
		}
		w.WriteHeader(200)
		json.NewEncoder(w).Encode(resp)
	})
	defer server.Close()

	raw, err := client.Discovery.GetIndex(context.Background(), []int{112})
	if err != nil {
		t.Fatalf("GetIndex: %v", err)
	}

	var containers map[string]interface{}
	if err := json.Unmarshal(raw, &containers); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if containers["new_releases"] == nil {
		t.Error("containers should contain new_releases")
	}
}

func TestDiscoveryNewReleases(t *testing.T) {
	server, client := testServerAndClient(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/discover/newReleases" {
			t.Errorf("path = %q, want /discover/newReleases", r.URL.Path)
		}

		resp := map[string]interface{}{
			"has_more": true,
			"items": []interface{}{
				map[string]interface{}{"id": "nr1", "title": "New Release 1"},
				map[string]interface{}{"id": "nr2", "title": "New Release 2"},
			},
		}
		w.WriteHeader(200)
		json.NewEncoder(w).Encode(resp)
	})
	defer server.Close()

	result, err := client.Discovery.NewReleases(context.Background(), nil, 0, 50)
	if err != nil {
		t.Fatalf("NewReleases: %v", err)
	}

	if len(result.Items) != 2 {
		t.Errorf("Items length = %d, want 2", len(result.Items))
	}
	if !result.HasMore {
		t.Error("HasMore should be true")
	}
}

func TestDiscoveryCuratedPlaylists(t *testing.T) {
	var capturedTags string

	server, client := testServerAndClient(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/discover/playlists" {
			t.Errorf("path = %q, want /discover/playlists", r.URL.Path)
		}
		capturedTags = r.URL.Query().Get("tags")

		resp := map[string]interface{}{
			"has_more": false,
			"items":    []interface{}{},
		}
		w.WriteHeader(200)
		json.NewEncoder(w).Encode(resp)
	})
	defer server.Close()

	result, err := client.Discovery.CuratedPlaylists(context.Background(), []int{112}, 0, 20)
	if err != nil {
		t.Fatalf("CuratedPlaylists: %v", err)
	}

	// Verify tags param is sent (empty string, matching Python behavior)
	if capturedTags != "" {
		t.Errorf("tags = %q, want empty string", capturedTags)
	}
	if result.HasMore {
		t.Error("HasMore should be false")
	}
}

func TestDiscoveryIdealDiscography(t *testing.T) {
	server, client := testServerAndClient(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/discover/idealDiscography" {
			t.Errorf("path = %q, want /discover/idealDiscography", r.URL.Path)
		}

		resp := map[string]interface{}{
			"has_more": false,
			"items":    []interface{}{map[string]interface{}{"id": "id1"}},
		}
		w.WriteHeader(200)
		json.NewEncoder(w).Encode(resp)
	})
	defer server.Close()

	result, err := client.Discovery.IdealDiscography(context.Background(), nil, 0, 48)
	if err != nil {
		t.Fatalf("IdealDiscography: %v", err)
	}

	if len(result.Items) != 1 {
		t.Errorf("Items length = %d, want 1", len(result.Items))
	}
}

func TestDiscoveryAlbumOfTheWeek(t *testing.T) {
	server, client := testServerAndClient(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/discover/albumOfTheWeek" {
			t.Errorf("path = %q, want /discover/albumOfTheWeek", r.URL.Path)
		}

		resp := map[string]interface{}{
			"has_more": false,
			"items":    []interface{}{},
		}
		w.WriteHeader(200)
		json.NewEncoder(w).Encode(resp)
	})
	defer server.Close()

	result, err := client.Discovery.AlbumOfTheWeek(context.Background(), nil, 0, 48)
	if err != nil {
		t.Fatalf("AlbumOfTheWeek: %v", err)
	}

	if result.HasMore {
		t.Error("HasMore should be false")
	}
}

func TestDiscoveryGenreIDsParam(t *testing.T) {
	var capturedGenreIDs string

	server, client := testServerAndClient(func(w http.ResponseWriter, r *http.Request) {
		capturedGenreIDs = r.URL.Query().Get("genre_ids")
		resp := map[string]interface{}{
			"has_more": false,
			"items":    []interface{}{},
		}
		w.WriteHeader(200)
		json.NewEncoder(w).Encode(resp)
	})
	defer server.Close()

	_, err := client.Discovery.NewReleases(context.Background(), []int{112, 113, 114}, 0, 50)
	if err != nil {
		t.Fatalf("NewReleases: %v", err)
	}

	if capturedGenreIDs != "112,113,114" {
		t.Errorf("genre_ids = %q, want %q", capturedGenreIDs, "112,113,114")
	}
}
