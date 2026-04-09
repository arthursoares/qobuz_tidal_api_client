package tidal

import (
	"context"
	"net/http"
	"testing"
)

func TestCatalogGetAlbum(t *testing.T) {
	server, client := testServerAndClient(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/albums/12345" {
			t.Errorf("path = %q, want /albums/12345", r.URL.Path)
		}
		if r.URL.Query().Get("countryCode") != "US" {
			t.Errorf("countryCode = %q, want US", r.URL.Query().Get("countryCode"))
		}

		doc := jsonAPISingleDoc(jsonAPIAlbum("12345", "4:44", 13, []string{"HIRES_LOSSLESS", "LOSSLESS"}))
		w.WriteHeader(200)
		w.Write(mustJSON(doc))
	})
	defer server.Close()

	album, err := client.Catalog.GetAlbum(context.Background(), "12345")
	if err != nil {
		t.Fatalf("GetAlbum: %v", err)
	}

	if album.ID != "12345" {
		t.Errorf("ID = %q, want %q", album.ID, "12345")
	}
	if album.Title != "4:44" {
		t.Errorf("Title = %q, want %q", album.Title, "4:44")
	}
	if album.NumberOfItems != 13 {
		t.Errorf("NumberOfItems = %d, want 13", album.NumberOfItems)
	}
	if !album.IsHiRes() {
		t.Error("IsHiRes() should be true")
	}
	if album.ReleaseDate != "2017-06-30" {
		t.Errorf("ReleaseDate = %q, want %q", album.ReleaseDate, "2017-06-30")
	}
}

func TestCatalogGetAlbumItems(t *testing.T) {
	server, client := testServerAndClient(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/albums/12345/relationships/items" {
			t.Errorf("path = %q, want /albums/12345/relationships/items", r.URL.Path)
		}

		identifiers := []map[string]string{
			{"type": "tracks", "id": "t1"},
			{"type": "tracks", "id": "t2"},
		}
		included := []map[string]any{
			jsonAPITrack("t1", "Kill Jay Z", "USRC17607839", false),
			jsonAPITrack("t2", "The Story of O.J.", "USRC17607840", true),
		}
		doc := jsonAPIRelationshipDoc(identifiers, included, "")
		w.WriteHeader(200)
		w.Write(mustJSON(doc))
	})
	defer server.Close()

	tracks, cursor, err := client.Catalog.GetAlbumItems(context.Background(), "12345")
	if err != nil {
		t.Fatalf("GetAlbumItems: %v", err)
	}

	if len(tracks) != 2 {
		t.Fatalf("len = %d, want 2", len(tracks))
	}
	if tracks[0].Title != "Kill Jay Z" {
		t.Errorf("tracks[0].Title = %q, want %q", tracks[0].Title, "Kill Jay Z")
	}
	if tracks[1].Explicit != true {
		t.Error("tracks[1].Explicit should be true")
	}
	if cursor != "" {
		t.Errorf("cursor = %q, want empty", cursor)
	}
}

func TestCatalogGetArtist(t *testing.T) {
	server, client := testServerAndClient(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/artists/7804" {
			t.Errorf("path = %q, want /artists/7804", r.URL.Path)
		}

		doc := jsonAPISingleDoc(jsonAPIArtist("7804", "JAY Z", 0.95))
		w.WriteHeader(200)
		w.Write(mustJSON(doc))
	})
	defer server.Close()

	artist, err := client.Catalog.GetArtist(context.Background(), "7804")
	if err != nil {
		t.Fatalf("GetArtist: %v", err)
	}

	if artist.ID != "7804" {
		t.Errorf("ID = %q, want %q", artist.ID, "7804")
	}
	if artist.Name != "JAY Z" {
		t.Errorf("Name = %q, want %q", artist.Name, "JAY Z")
	}
}

func TestCatalogGetArtistAlbums(t *testing.T) {
	server, client := testServerAndClient(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/artists/7804/relationships/albums" {
			t.Errorf("path = %q, want /artists/7804/relationships/albums", r.URL.Path)
		}

		identifiers := []map[string]string{
			{"type": "albums", "id": "a1"},
		}
		included := []map[string]any{
			jsonAPIAlbum("a1", "4:44", 13, []string{"HIRES_LOSSLESS"}),
		}
		doc := jsonAPIRelationshipDoc(identifiers, included, "")
		w.WriteHeader(200)
		w.Write(mustJSON(doc))
	})
	defer server.Close()

	albums, _, err := client.Catalog.GetArtistAlbums(context.Background(), "7804")
	if err != nil {
		t.Fatalf("GetArtistAlbums: %v", err)
	}

	if len(albums) != 1 {
		t.Fatalf("len = %d, want 1", len(albums))
	}
	if albums[0].Title != "4:44" {
		t.Errorf("Title = %q, want %q", albums[0].Title, "4:44")
	}
}

func TestCatalogGetTrack(t *testing.T) {
	server, client := testServerAndClient(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/tracks/t1" {
			t.Errorf("path = %q, want /tracks/t1", r.URL.Path)
		}

		doc := jsonAPISingleDoc(jsonAPITrack("t1", "Kill Jay Z", "USRC17607839", false))
		w.WriteHeader(200)
		w.Write(mustJSON(doc))
	})
	defer server.Close()

	track, err := client.Catalog.GetTrack(context.Background(), "t1")
	if err != nil {
		t.Fatalf("GetTrack: %v", err)
	}

	if track.ID != "t1" {
		t.Errorf("ID = %q, want %q", track.ID, "t1")
	}
	if track.Title != "Kill Jay Z" {
		t.Errorf("Title = %q, want %q", track.Title, "Kill Jay Z")
	}
	if track.ISRC != "USRC17607839" {
		t.Errorf("ISRC = %q, want %q", track.ISRC, "USRC17607839")
	}
}

func TestCatalogGetGenres(t *testing.T) {
	server, client := testServerAndClient(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/genres" {
			t.Errorf("path = %q, want /genres", r.URL.Path)
		}

		resources := []map[string]any{
			jsonAPIGenre("pop", "Pop"),
			jsonAPIGenre("rock", "Rock"),
			jsonAPIGenre("hip-hop", "Hip-Hop"),
		}
		doc := jsonAPIMultiDoc(resources, "")
		w.WriteHeader(200)
		w.Write(mustJSON(doc))
	})
	defer server.Close()

	genres, err := client.Catalog.GetGenres(context.Background())
	if err != nil {
		t.Fatalf("GetGenres: %v", err)
	}

	if len(genres) != 3 {
		t.Fatalf("len = %d, want 3", len(genres))
	}
	if genres[0].ID != "pop" {
		t.Errorf("genres[0].ID = %q, want %q", genres[0].ID, "pop")
	}
	if genres[0].Name != "Pop" {
		t.Errorf("genres[0].Name = %q, want %q", genres[0].Name, "Pop")
	}
	if genres[2].Name != "Hip-Hop" {
		t.Errorf("genres[2].Name = %q, want %q", genres[2].Name, "Hip-Hop")
	}
}

func TestCatalogNotFound(t *testing.T) {
	server, client := testServerAndClient(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
		w.Write([]byte(`{"errors": [{"title": "Not Found", "detail": "Album does not exist"}]}`))
	})
	defer server.Close()

	_, err := client.Catalog.GetAlbum(context.Background(), "nonexistent")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !IsNotFoundError(err) {
		t.Errorf("expected not-found error, got: %v", err)
	}
}
