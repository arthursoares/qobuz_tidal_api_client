package qobuz

import (
	"encoding/json"
	"testing"
)

func TestAlbumUnmarshal(t *testing.T) {
	raw := `{
		"id": "p0d55tt7gv3lc",
		"title": "Virgin Lake",
		"version": null,
		"maximum_bit_depth": 24,
		"maximum_sampling_rate": 44.1,
		"maximum_channel_count": 2,
		"duration": 3487,
		"tracks_count": 14,
		"streamable": true,
		"downloadable": true,
		"hires": true,
		"hires_streamable": true,
		"release_date_original": "2026-04-03",
		"upc": "0067003183055",
		"image": {
			"small": "https://static.qobuz.com/small.jpg",
			"thumbnail": "https://static.qobuz.com/thumb.jpg",
			"large": "https://static.qobuz.com/large.jpg"
		},
		"artist": {"id": 11162390, "name": "Philine Sonny"},
		"artists": [{"id": 11162390, "name": "Philine Sonny", "roles": ["main-artist"]}],
		"label": {"id": 2367808, "name": "Nettwerk Music Group"},
		"genre": {"id": 113, "name": "Alternative", "color": "#5eabc1", "path": [112], "slug": "alternative"},
		"description": "A great album",
		"awards": []
	}`

	var album Album
	if err := json.Unmarshal([]byte(raw), &album); err != nil {
		t.Fatalf("unmarshal album: %v", err)
	}

	if album.ID != "p0d55tt7gv3lc" {
		t.Errorf("ID = %q, want %q", album.ID, "p0d55tt7gv3lc")
	}
	if album.Title != "Virgin Lake" {
		t.Errorf("Title = %q, want %q", album.Title, "Virgin Lake")
	}
	if album.Version != nil {
		t.Errorf("Version = %v, want nil", album.Version)
	}
	if album.MaximumBitDepth != 24 {
		t.Errorf("MaximumBitDepth = %d, want 24", album.MaximumBitDepth)
	}
	if album.MaximumSamplingRate != 44.1 {
		t.Errorf("MaximumSamplingRate = %f, want 44.1", album.MaximumSamplingRate)
	}
	if album.TracksCount != 14 {
		t.Errorf("TracksCount = %d, want 14", album.TracksCount)
	}
	if !album.Streamable {
		t.Error("Streamable should be true")
	}
	if !album.Hires {
		t.Error("Hires should be true")
	}
	if album.Artist.Name != "Philine Sonny" {
		t.Errorf("Artist.Name = %q, want %q", album.Artist.Name, "Philine Sonny")
	}
	if album.Label == nil || album.Label.Name != "Nettwerk Music Group" {
		t.Error("Label should be Nettwerk Music Group")
	}
	if album.Genre == nil || album.Genre.Name != "Alternative" {
		t.Error("Genre should be Alternative")
	}
	if album.Description == nil || *album.Description != "A great album" {
		t.Error("Description should be 'A great album'")
	}
}

func TestAlbumUnmarshalNumericID(t *testing.T) {
	raw := `{
		"id": 12345,
		"title": "Numeric ID Album",
		"artist": {"id": 1, "name": "Test Artist"},
		"artists": [],
		"image": {}
	}`

	var album Album
	if err := json.Unmarshal([]byte(raw), &album); err != nil {
		t.Fatalf("unmarshal album: %v", err)
	}

	if album.ID != "12345" {
		t.Errorf("ID = %q, want %q", album.ID, "12345")
	}
}

func TestArtistSummaryStringName(t *testing.T) {
	raw := `{"id": 123, "name": "Radiohead"}`

	var artist ArtistSummary
	if err := json.Unmarshal([]byte(raw), &artist); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if artist.Name != "Radiohead" {
		t.Errorf("Name = %q, want %q", artist.Name, "Radiohead")
	}
}

func TestArtistSummaryDisplayName(t *testing.T) {
	raw := `{"id": 38895, "name": {"display": "Talk Talk"}}`

	var artist ArtistSummary
	if err := json.Unmarshal([]byte(raw), &artist); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if artist.Name != "Talk Talk" {
		t.Errorf("Name = %q, want %q", artist.Name, "Talk Talk")
	}
}

func TestTrackUnmarshal(t *testing.T) {
	raw := `{
		"id": 33967376,
		"title": "Everything but the Girl",
		"version": null,
		"duration": 245,
		"track_number": 3,
		"media_number": 1,
		"parental_warning": false,
		"performer": {"id": 123, "name": "Some Artist"},
		"album": {"id": "abc123", "title": "Some Album", "image": {}},
		"audio_info": {"maximum_bit_depth": 24, "maximum_channel_count": 2, "maximum_sampling_rate": 96.0},
		"rights": {"streamable": true, "downloadable": true, "hires_streamable": true, "purchasable": true},
		"isrc": "USRC12345678"
	}`

	var track Track
	if err := json.Unmarshal([]byte(raw), &track); err != nil {
		t.Fatalf("unmarshal track: %v", err)
	}

	if track.ID != 33967376 {
		t.Errorf("ID = %d, want 33967376", track.ID)
	}
	if track.Title != "Everything but the Girl" {
		t.Errorf("Title = %q, want %q", track.Title, "Everything but the Girl")
	}
	if track.TrackNumber != 3 {
		t.Errorf("TrackNumber = %d, want 3", track.TrackNumber)
	}
	if track.DiscNumber != 1 {
		t.Errorf("DiscNumber = %d, want 1", track.DiscNumber)
	}
	if track.AudioInfo.MaximumBitDepth != 24 {
		t.Errorf("AudioInfo.MaximumBitDepth = %d, want 24", track.AudioInfo.MaximumBitDepth)
	}
	if !track.Rights.Streamable {
		t.Error("Rights.Streamable should be true")
	}
	if track.ISRC == nil || *track.ISRC != "USRC12345678" {
		t.Error("ISRC should be USRC12345678")
	}
}

func TestPlaylistUnmarshal(t *testing.T) {
	raw := `{
		"id": 61997651,
		"name": "My Playlist",
		"description": "Test description",
		"tracks_count": 10,
		"users_count": 1,
		"duration": 3600,
		"is_public": true,
		"is_collaborative": false,
		"public_at": false,
		"created_at": 1775635602,
		"updated_at": 1775635602,
		"owner": {"id": 2113276, "name": "arthursoares"}
	}`

	var playlist Playlist
	if err := json.Unmarshal([]byte(raw), &playlist); err != nil {
		t.Fatalf("unmarshal playlist: %v", err)
	}

	if playlist.ID != 61997651 {
		t.Errorf("ID = %d, want 61997651", playlist.ID)
	}
	if playlist.Name != "My Playlist" {
		t.Errorf("Name = %q, want %q", playlist.Name, "My Playlist")
	}
	if !playlist.IsPublic {
		t.Error("IsPublic should be true")
	}
	if playlist.Owner.Name != "arthursoares" {
		t.Errorf("Owner.Name = %q, want %q", playlist.Owner.Name, "arthursoares")
	}
}

func TestFavoriteIdsUnmarshal(t *testing.T) {
	raw := `{
		"albums": ["abc123", "def456"],
		"tracks": [100, 200, 300],
		"artists": [1, 2],
		"labels": [10],
		"awards": []
	}`

	var ids FavoriteIds
	if err := json.Unmarshal([]byte(raw), &ids); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if len(ids.Albums) != 2 || ids.Albums[0] != "abc123" {
		t.Errorf("Albums = %v, want [abc123 def456]", ids.Albums)
	}
	if len(ids.Tracks) != 3 {
		t.Errorf("Tracks length = %d, want 3", len(ids.Tracks))
	}
	if len(ids.Artists) != 2 {
		t.Errorf("Artists length = %d, want 2", len(ids.Artists))
	}
}

func TestFavoriteIdsUnmarshalNumericAlbums(t *testing.T) {
	raw := `{
		"albums": [12345, 67890],
		"tracks": [],
		"artists": [],
		"labels": [],
		"awards": []
	}`

	var ids FavoriteIds
	if err := json.Unmarshal([]byte(raw), &ids); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if len(ids.Albums) != 2 || ids.Albums[0] != "12345" {
		t.Errorf("Albums = %v, want [12345 67890]", ids.Albums)
	}
}

func TestParsePaginatedWithKey(t *testing.T) {
	raw := `{
		"albums": {
			"items": [{"id": "a1"}, {"id": "a2"}],
			"total": 100,
			"limit": 50,
			"offset": 0
		}
	}`

	result, err := ParsePaginated([]byte(raw), "albums")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if len(result.Items) != 2 {
		t.Errorf("Items length = %d, want 2", len(result.Items))
	}
	if result.Total == nil || *result.Total != 100 {
		t.Error("Total should be 100")
	}
	if result.Limit != 50 {
		t.Errorf("Limit = %d, want 50", result.Limit)
	}
	if !result.HasMore {
		t.Error("HasMore should be true (offset 0 + limit 50 < total 100)")
	}
}

func TestParsePaginatedHasMoreStyle(t *testing.T) {
	raw := `{
		"has_more": true,
		"items": [{"id": 1}, {"id": 2}, {"id": 3}]
	}`

	result, err := ParsePaginated([]byte(raw), "")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if len(result.Items) != 3 {
		t.Errorf("Items length = %d, want 3", len(result.Items))
	}
	if result.Total != nil {
		t.Error("Total should be nil for has_more style")
	}
	if !result.HasMore {
		t.Error("HasMore should be true")
	}
}

func TestParsePaginatedNoMore(t *testing.T) {
	raw := `{
		"albums": {
			"items": [{"id": "a1"}],
			"total": 1,
			"limit": 50,
			"offset": 0
		}
	}`

	result, err := ParsePaginated([]byte(raw), "albums")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if result.HasMore {
		t.Error("HasMore should be false (offset 0 + limit 50 >= total 1)")
	}
}

func TestLastUpdateUnmarshal(t *testing.T) {
	raw := `{
		"last_update": {
			"favorite": 1000,
			"favorite_album": 2000,
			"favorite_artist": 3000,
			"favorite_track": 4000,
			"favorite_label": 5000,
			"playlist": 6000,
			"purchase": 7000
		}
	}`

	var wrapper lastUpdateWrapper
	if err := json.Unmarshal([]byte(raw), &wrapper); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	lu := wrapper.LastUpdate
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

func TestFileUrlUnmarshal(t *testing.T) {
	raw := `{
		"track_id": 33967376,
		"format_id": 7,
		"mime_type": "audio/mp4; codecs=\"flac\"",
		"sampling_rate": 96000,
		"bits_depth": 24,
		"duration": 133.29,
		"url_template": "https://streaming.example.com/file?s=$SEGMENT$",
		"n_segments": 14,
		"key_id": "bfff4e0a-b8d9-6de0-81d8-833f326f3082",
		"key": "qbz-1.encrypted",
		"blob": "opaque_blob",
		"restrictions": [{"code": "FormatRestrictedByFormatAvailability"}]
	}`

	var f FileUrl
	if err := json.Unmarshal([]byte(raw), &f); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if f.TrackID != 33967376 {
		t.Errorf("TrackID = %d, want 33967376", f.TrackID)
	}
	if f.FormatID != 7 {
		t.Errorf("FormatID = %d, want 7", f.FormatID)
	}
	if f.NSegments != 14 {
		t.Errorf("NSegments = %d, want 14", f.NSegments)
	}
	if f.KeyID == nil || *f.KeyID != "bfff4e0a-b8d9-6de0-81d8-833f326f3082" {
		t.Error("KeyID mismatch")
	}
	if f.Blob == nil || *f.Blob != "opaque_blob" {
		t.Error("Blob mismatch")
	}
}

func TestSessionUnmarshal(t *testing.T) {
	raw := `{
		"session_id": "sess-123",
		"profile": "qbz-1",
		"expires_at": 1775700000
	}`

	var s Session
	if err := json.Unmarshal([]byte(raw), &s); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if s.SessionID != "sess-123" {
		t.Errorf("SessionID = %q, want %q", s.SessionID, "sess-123")
	}
	if s.Profile != "qbz-1" {
		t.Errorf("Profile = %q, want %q", s.Profile, "qbz-1")
	}
	if s.ExpiresAt != 1775700000 {
		t.Errorf("ExpiresAt = %d, want 1775700000", s.ExpiresAt)
	}
}

func TestUserSummaryDisplayName(t *testing.T) {
	raw := `{"id": 42, "display_name": "User42"}`

	var u UserSummary
	if err := json.Unmarshal([]byte(raw), &u); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if u.Name != "User42" {
		t.Errorf("Name = %q, want %q", u.Name, "User42")
	}
}

func TestAlbumSummaryNumericID(t *testing.T) {
	raw := `{"id": 99999, "title": "Test Album", "image": {}}`

	var a AlbumSummary
	if err := json.Unmarshal([]byte(raw), &a); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if a.ID != "99999" {
		t.Errorf("ID = %q, want %q", a.ID, "99999")
	}
}
