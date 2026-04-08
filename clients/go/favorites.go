package qobuz

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// FavoritesService provides methods for the Qobuz favorite/* endpoints.
type FavoritesService struct {
	t *transport
}

// AddAlbum adds a single album to favorites.
func (s *FavoritesService) AddAlbum(ctx context.Context, albumID string) error {
	_, err := s.t.postForm(ctx, "favorite/create", map[string]string{
		"album_ids":  albumID,
		"artist_ids": "",
		"track_ids":  "",
	})
	return err
}

// AddAlbums adds multiple albums to favorites.
func (s *FavoritesService) AddAlbums(ctx context.Context, albumIDs []string) error {
	_, err := s.t.postForm(ctx, "favorite/create", map[string]string{
		"album_ids":  strings.Join(albumIDs, ","),
		"artist_ids": "",
		"track_ids":  "",
	})
	return err
}

// AddTrack adds a single track to favorites.
func (s *FavoritesService) AddTrack(ctx context.Context, trackID string) error {
	_, err := s.t.postForm(ctx, "favorite/create", map[string]string{
		"album_ids":  "",
		"artist_ids": "",
		"track_ids":  trackID,
	})
	return err
}

// AddTracks adds multiple tracks to favorites.
func (s *FavoritesService) AddTracks(ctx context.Context, trackIDs []string) error {
	_, err := s.t.postForm(ctx, "favorite/create", map[string]string{
		"album_ids":  "",
		"artist_ids": "",
		"track_ids":  strings.Join(trackIDs, ","),
	})
	return err
}

// AddArtist adds a single artist to favorites.
func (s *FavoritesService) AddArtist(ctx context.Context, artistID string) error {
	_, err := s.t.postForm(ctx, "favorite/create", map[string]string{
		"album_ids":  "",
		"artist_ids": artistID,
		"track_ids":  "",
	})
	return err
}

// RemoveAlbum removes an album from favorites.
func (s *FavoritesService) RemoveAlbum(ctx context.Context, albumID string) error {
	_, err := s.t.postForm(ctx, "favorite/delete", map[string]string{
		"album_ids":  albumID,
		"artist_ids": "",
		"track_ids":  "",
	})
	return err
}

// RemoveTrack removes a track from favorites.
func (s *FavoritesService) RemoveTrack(ctx context.Context, trackID string) error {
	_, err := s.t.postForm(ctx, "favorite/delete", map[string]string{
		"album_ids":  "",
		"artist_ids": "",
		"track_ids":  trackID,
	})
	return err
}

// RemoveArtist removes an artist from favorites.
func (s *FavoritesService) RemoveArtist(ctx context.Context, artistID string) error {
	_, err := s.t.postForm(ctx, "favorite/delete", map[string]string{
		"album_ids":  "",
		"artist_ids": artistID,
		"track_ids":  "",
	})
	return err
}

// GetAlbums returns paginated favorite albums with fully parsed Album objects.
func (s *FavoritesService) GetAlbums(ctx context.Context, limit, offset int) (*FavoriteAlbums, error) {
	data, err := s.t.get(ctx, "favorite/getUserFavorites", map[string]string{
		"type":   "albums",
		"limit":  fmt.Sprintf("%d", limit),
		"offset": fmt.Sprintf("%d", offset),
	})
	if err != nil {
		return nil, err
	}

	var body struct {
		Albums struct {
			Items  []json.RawMessage `json:"items"`
			Total  int               `json:"total"`
			Limit  int               `json:"limit"`
			Offset int               `json:"offset"`
		} `json:"albums"`
	}
	if err := json.Unmarshal(data, &body); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	albums := make([]Album, 0, len(body.Albums.Items))
	for _, raw := range body.Albums.Items {
		var a Album
		if err := json.Unmarshal(raw, &a); err != nil {
			return nil, fmt.Errorf("parse album: %w", err)
		}
		albums = append(albums, a)
	}

	return &FavoriteAlbums{
		Items:  albums,
		Total:  body.Albums.Total,
		Limit:  body.Albums.Limit,
		Offset: body.Albums.Offset,
	}, nil
}

// GetTracks returns paginated favorite tracks.
func (s *FavoritesService) GetTracks(ctx context.Context, limit, offset int) (*PaginatedResult, error) {
	data, err := s.t.get(ctx, "favorite/getUserFavorites", map[string]string{
		"type":   "tracks",
		"limit":  fmt.Sprintf("%d", limit),
		"offset": fmt.Sprintf("%d", offset),
	})
	if err != nil {
		return nil, err
	}
	return ParsePaginated(data, "tracks")
}

// GetArtists returns paginated favorite artists.
func (s *FavoritesService) GetArtists(ctx context.Context, limit, offset int) (*PaginatedResult, error) {
	data, err := s.t.get(ctx, "favorite/getUserFavorites", map[string]string{
		"type":   "artists",
		"limit":  fmt.Sprintf("%d", limit),
		"offset": fmt.Sprintf("%d", offset),
	})
	if err != nil {
		return nil, err
	}
	return ParsePaginated(data, "artists")
}

// GetIDs returns all favorite IDs (albums, tracks, artists, labels, awards).
func (s *FavoritesService) GetIDs(ctx context.Context, limit int) (*FavoriteIds, error) {
	data, err := s.t.get(ctx, "favorite/getUserFavoriteIds", map[string]string{
		"limit": fmt.Sprintf("%d", limit),
	})
	if err != nil {
		return nil, err
	}

	var ids FavoriteIds
	if err := json.Unmarshal(data, &ids); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}
	return &ids, nil
}
