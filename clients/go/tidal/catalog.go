package tidal

import (
	"context"
	"fmt"
)

// CatalogService provides methods for album, artist, track, and genre endpoints.
type CatalogService struct {
	t *transport
}

// --- Albums ---

// GetAlbum fetches a single album by ID.
func (s *CatalogService) GetAlbum(ctx context.Context, albumID string) (*Album, error) {
	data, err := s.t.get(ctx, fmt.Sprintf("albums/%s", albumID), map[string]string{})
	if err != nil {
		return nil, err
	}

	album, _, err := ParseOne[Album](data, func(a *Album, id string) { a.ID = id })
	if err != nil {
		return nil, fmt.Errorf("parse album: %w", err)
	}
	return album, nil
}

// GetAlbumItems fetches the tracks in an album.
func (s *CatalogService) GetAlbumItems(ctx context.Context, albumID string) ([]Track, string, error) {
	params := map[string]string{
		"include": "items",
	}
	data, err := s.t.get(ctx, fmt.Sprintf("albums/%s/relationships/items", albumID), params)
	if err != nil {
		return nil, "", err
	}
	return ParseRelationship[Track](data, func(t *Track, id string) { t.ID = id })
}

// --- Artists ---

// GetArtist fetches a single artist by ID.
func (s *CatalogService) GetArtist(ctx context.Context, artistID string) (*Artist, error) {
	data, err := s.t.get(ctx, fmt.Sprintf("artists/%s", artistID), map[string]string{})
	if err != nil {
		return nil, err
	}

	artist, _, err := ParseOne[Artist](data, func(a *Artist, id string) { a.ID = id })
	if err != nil {
		return nil, fmt.Errorf("parse artist: %w", err)
	}
	return artist, nil
}

// GetArtistAlbums fetches albums for an artist.
func (s *CatalogService) GetArtistAlbums(ctx context.Context, artistID string) ([]Album, string, error) {
	params := map[string]string{
		"include": "albums",
	}
	data, err := s.t.get(ctx, fmt.Sprintf("artists/%s/relationships/albums", artistID), params)
	if err != nil {
		return nil, "", err
	}
	return ParseRelationship[Album](data, func(a *Album, id string) { a.ID = id })
}

// --- Tracks ---

// GetTrack fetches a single track by ID.
func (s *CatalogService) GetTrack(ctx context.Context, trackID string) (*Track, error) {
	data, err := s.t.get(ctx, fmt.Sprintf("tracks/%s", trackID), map[string]string{})
	if err != nil {
		return nil, err
	}

	track, _, err := ParseOne[Track](data, func(t *Track, id string) { t.ID = id })
	if err != nil {
		return nil, fmt.Errorf("parse track: %w", err)
	}
	return track, nil
}

// --- Genres ---

// GetGenres returns all available genres.
func (s *CatalogService) GetGenres(ctx context.Context) ([]Genre, error) {
	data, err := s.t.get(ctx, "genres", map[string]string{})
	if err != nil {
		return nil, err
	}

	genres, _, _, err := ParseMany[Genre](data, func(g *Genre, id string) { g.ID = id })
	if err != nil {
		return nil, fmt.Errorf("parse genres: %w", err)
	}
	return genres, nil
}
