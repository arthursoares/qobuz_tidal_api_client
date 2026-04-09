package tidal

import (
	"testing"
)

func TestParseOneSingleAlbum(t *testing.T) {
	doc := jsonAPISingleDoc(jsonAPIAlbum("12345", "4:44", 13, []string{"HIRES_LOSSLESS", "LOSSLESS"}))
	body := mustJSON(doc)

	album, _, err := ParseOne[Album](body, func(a *Album, id string) { a.ID = id })
	if err != nil {
		t.Fatalf("ParseOne: %v", err)
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
}

func TestParseManyAlbums(t *testing.T) {
	resources := []map[string]any{
		jsonAPIAlbum("1", "Album One", 10, []string{"LOSSLESS"}),
		jsonAPIAlbum("2", "Album Two", 8, []string{"HIRES_LOSSLESS"}),
	}
	doc := jsonAPIMultiDoc(resources, "/albums?page[cursor]=abc123")
	body := mustJSON(doc)

	albums, _, cursor, err := ParseMany[Album](body, func(a *Album, id string) { a.ID = id })
	if err != nil {
		t.Fatalf("ParseMany: %v", err)
	}

	if len(albums) != 2 {
		t.Fatalf("len = %d, want 2", len(albums))
	}
	if albums[0].Title != "Album One" {
		t.Errorf("albums[0].Title = %q, want %q", albums[0].Title, "Album One")
	}
	if albums[1].ID != "2" {
		t.Errorf("albums[1].ID = %q, want %q", albums[1].ID, "2")
	}
	if cursor != "abc123" {
		t.Errorf("cursor = %q, want %q", cursor, "abc123")
	}
}

func TestParseManyNoPagination(t *testing.T) {
	resources := []map[string]any{
		jsonAPIAlbum("1", "Solo Album", 5, nil),
	}
	doc := jsonAPIMultiDoc(resources, "")
	body := mustJSON(doc)

	albums, _, cursor, err := ParseMany[Album](body, func(a *Album, id string) { a.ID = id })
	if err != nil {
		t.Fatalf("ParseMany: %v", err)
	}

	if len(albums) != 1 {
		t.Fatalf("len = %d, want 1", len(albums))
	}
	if cursor != "" {
		t.Errorf("cursor = %q, want empty", cursor)
	}
}

func TestParseRelationshipWithIncluded(t *testing.T) {
	identifiers := []map[string]string{
		{"type": "albums", "id": "100"},
		{"type": "albums", "id": "200"},
	}
	included := []map[string]any{
		jsonAPIAlbum("100", "Included Album 1", 12, []string{"LOSSLESS"}),
		jsonAPIAlbum("200", "Included Album 2", 9, []string{"HIRES_LOSSLESS"}),
	}
	doc := jsonAPIRelationshipDoc(identifiers, included, "")
	body := mustJSON(doc)

	albums, cursor, err := ParseRelationship[Album](body, func(a *Album, id string) { a.ID = id })
	if err != nil {
		t.Fatalf("ParseRelationship: %v", err)
	}

	if len(albums) != 2 {
		t.Fatalf("len = %d, want 2", len(albums))
	}
	if albums[0].ID != "100" {
		t.Errorf("albums[0].ID = %q, want %q", albums[0].ID, "100")
	}
	if albums[0].Title != "Included Album 1" {
		t.Errorf("albums[0].Title = %q, want %q", albums[0].Title, "Included Album 1")
	}
	if albums[1].NumberOfItems != 9 {
		t.Errorf("albums[1].NumberOfItems = %d, want 9", albums[1].NumberOfItems)
	}
	if cursor != "" {
		t.Errorf("cursor = %q, want empty", cursor)
	}
}

func TestParseRelationshipWithPagination(t *testing.T) {
	identifiers := []map[string]string{
		{"type": "tracks", "id": "t1"},
	}
	included := []map[string]any{
		jsonAPITrack("t1", "Test Track", "USRC17607839", false),
	}
	doc := jsonAPIRelationshipDoc(identifiers, included, "/searchResults/test/relationships/tracks?page[cursor]=nextpage")
	body := mustJSON(doc)

	tracks, cursor, err := ParseRelationship[Track](body, func(t *Track, id string) { t.ID = id })
	if err != nil {
		t.Fatalf("ParseRelationship: %v", err)
	}

	if len(tracks) != 1 {
		t.Fatalf("len = %d, want 1", len(tracks))
	}
	if tracks[0].ISRC != "USRC17607839" {
		t.Errorf("ISRC = %q, want %q", tracks[0].ISRC, "USRC17607839")
	}
	if cursor != "nextpage" {
		t.Errorf("cursor = %q, want %q", cursor, "nextpage")
	}
}

func TestParseRelationshipMissingIncluded(t *testing.T) {
	// When included resources are missing, items should still be created with IDs
	identifiers := []map[string]string{
		{"type": "albums", "id": "orphan1"},
	}
	doc := jsonAPIRelationshipDoc(identifiers, nil, "")
	body := mustJSON(doc)

	albums, _, err := ParseRelationship[Album](body, func(a *Album, id string) { a.ID = id })
	if err != nil {
		t.Fatalf("ParseRelationship: %v", err)
	}

	if len(albums) != 1 {
		t.Fatalf("len = %d, want 1", len(albums))
	}
	if albums[0].ID != "orphan1" {
		t.Errorf("ID = %q, want %q", albums[0].ID, "orphan1")
	}
	if albums[0].Title != "" {
		t.Errorf("Title = %q, want empty (no included data)", albums[0].Title)
	}
}

func TestResourcePayload(t *testing.T) {
	payload := ResourcePayload("albums", []string{"123", "456"})
	data, ok := payload["data"].([]map[string]string)
	if !ok {
		t.Fatal("data should be []map[string]string")
	}
	if len(data) != 2 {
		t.Fatalf("len = %d, want 2", len(data))
	}
	if data[0]["type"] != "albums" || data[0]["id"] != "123" {
		t.Errorf("data[0] = %v, want type=albums id=123", data[0])
	}
	if data[1]["type"] != "albums" || data[1]["id"] != "456" {
		t.Errorf("data[1] = %v, want type=albums id=456", data[1])
	}
}

func TestCreateResourcePayload(t *testing.T) {
	payload := CreateResourcePayload("playlists", map[string]any{
		"name":       "Test",
		"accessType": "PUBLIC",
	})
	data, ok := payload["data"].(map[string]any)
	if !ok {
		t.Fatal("data should be map[string]any")
	}
	if data["type"] != "playlists" {
		t.Errorf("type = %v, want playlists", data["type"])
	}
	attrs, ok := data["attributes"].(map[string]any)
	if !ok {
		t.Fatal("attributes should be map[string]any")
	}
	if attrs["name"] != "Test" {
		t.Errorf("name = %v, want Test", attrs["name"])
	}
}

func TestExtractCursorFromURL(t *testing.T) {
	tests := []struct {
		name string
		links *Links
		want  string
	}{
		{"nil links", nil, ""},
		{"empty next", &Links{Self: "/test"}, ""},
		{"valid cursor", &Links{Self: "/test", Next: "/test?page[cursor]=xyz"}, "xyz"},
		{"no cursor param", &Links{Self: "/test", Next: "/test?other=val"}, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractCursor(tt.links)
			if got != tt.want {
				t.Errorf("extractCursor() = %q, want %q", got, tt.want)
			}
		})
	}
}
