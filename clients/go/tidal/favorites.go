package tidal

import (
	"context"
	"fmt"
)

// FavoritesService provides methods for the Tidal userCollections endpoints.
type FavoritesService struct {
	t      *transport
	userID string
}

// --- Albums ---

// AddAlbum adds an album to the user's collection.
func (s *FavoritesService) AddAlbum(ctx context.Context, albumID string) error {
	endpoint := fmt.Sprintf("userCollections/%s/relationships/albums", s.userID)
	payload := ResourcePayload("albums", []string{albumID})
	_, err := s.t.postJSON(ctx, endpoint, payload)
	return err
}

// RemoveAlbum removes an album from the user's collection.
func (s *FavoritesService) RemoveAlbum(ctx context.Context, albumID string) error {
	endpoint := fmt.Sprintf("userCollections/%s/relationships/albums", s.userID)
	payload := ResourcePayload("albums", []string{albumID})
	_, err := s.t.deleteJSON(ctx, endpoint, payload)
	return err
}

// GetAlbums returns the user's favorite albums.
// Uses the include parameter to get full album attributes in the response.
func (s *FavoritesService) GetAlbums(ctx context.Context, limit int) ([]Album, string, error) {
	endpoint := fmt.Sprintf("userCollections/%s/relationships/albums", s.userID)
	params := map[string]string{
		"include": "albums",
	}
	if limit > 0 {
		params["page[cursor]"] = ""
	}
	data, err := s.t.get(ctx, endpoint, params)
	if err != nil {
		return nil, "", err
	}
	return ParseRelationship[Album](data, func(a *Album, id string) { a.ID = id })
}

// GetAlbumsPage returns a specific page of favorite albums using a cursor.
func (s *FavoritesService) GetAlbumsPage(ctx context.Context, cursor string) ([]Album, string, error) {
	endpoint := fmt.Sprintf("userCollections/%s/relationships/albums", s.userID)
	params := map[string]string{
		"include": "albums",
	}
	if cursor != "" {
		params["page[cursor]"] = cursor
	}
	data, err := s.t.get(ctx, endpoint, params)
	if err != nil {
		return nil, "", err
	}
	return ParseRelationship[Album](data, func(a *Album, id string) { a.ID = id })
}

// --- Tracks ---

// AddTrack adds a track to the user's collection.
func (s *FavoritesService) AddTrack(ctx context.Context, trackID string) error {
	endpoint := fmt.Sprintf("userCollections/%s/relationships/tracks", s.userID)
	payload := ResourcePayload("tracks", []string{trackID})
	_, err := s.t.postJSON(ctx, endpoint, payload)
	return err
}

// RemoveTrack removes a track from the user's collection.
func (s *FavoritesService) RemoveTrack(ctx context.Context, trackID string) error {
	endpoint := fmt.Sprintf("userCollections/%s/relationships/tracks", s.userID)
	payload := ResourcePayload("tracks", []string{trackID})
	_, err := s.t.deleteJSON(ctx, endpoint, payload)
	return err
}

// GetTracks returns the user's favorite tracks.
func (s *FavoritesService) GetTracks(ctx context.Context, limit int) ([]Track, string, error) {
	endpoint := fmt.Sprintf("userCollections/%s/relationships/tracks", s.userID)
	params := map[string]string{
		"include": "tracks",
	}
	data, err := s.t.get(ctx, endpoint, params)
	if err != nil {
		return nil, "", err
	}
	return ParseRelationship[Track](data, func(t *Track, id string) { t.ID = id })
}

// --- Artists ---

// AddArtist adds an artist to the user's collection.
func (s *FavoritesService) AddArtist(ctx context.Context, artistID string) error {
	endpoint := fmt.Sprintf("userCollections/%s/relationships/artists", s.userID)
	payload := ResourcePayload("artists", []string{artistID})
	_, err := s.t.postJSON(ctx, endpoint, payload)
	return err
}

// RemoveArtist removes an artist from the user's collection.
func (s *FavoritesService) RemoveArtist(ctx context.Context, artistID string) error {
	endpoint := fmt.Sprintf("userCollections/%s/relationships/artists", s.userID)
	payload := ResourcePayload("artists", []string{artistID})
	_, err := s.t.deleteJSON(ctx, endpoint, payload)
	return err
}

// GetArtists returns the user's favorite artists.
func (s *FavoritesService) GetArtists(ctx context.Context, limit int) ([]Artist, string, error) {
	endpoint := fmt.Sprintf("userCollections/%s/relationships/artists", s.userID)
	params := map[string]string{
		"include": "artists",
	}
	data, err := s.t.get(ctx, endpoint, params)
	if err != nil {
		return nil, "", err
	}
	return ParseRelationship[Artist](data, func(a *Artist, id string) { a.ID = id })
}
