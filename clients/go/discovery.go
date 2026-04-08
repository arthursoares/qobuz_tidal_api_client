package qobuz

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// DiscoveryService provides methods for the Qobuz discover/* and genre/* endpoints.
type DiscoveryService struct {
	t *transport
}

// ListGenres returns all available genres.
func (s *DiscoveryService) ListGenres(ctx context.Context) ([]Genre, error) {
	data, err := s.t.get(ctx, "genre/list", map[string]string{})
	if err != nil {
		return nil, err
	}

	var body struct {
		Genres struct {
			Items []Genre `json:"items"`
		} `json:"genres"`
	}
	if err := json.Unmarshal(data, &body); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}
	return body.Genres.Items, nil
}

// GetIndex returns the discovery index page as raw JSON.
func (s *DiscoveryService) GetIndex(ctx context.Context, genreIDs []int) (json.RawMessage, error) {
	params := map[string]string{}
	if len(genreIDs) > 0 {
		params["genre_ids"] = joinInts(genreIDs)
	}

	data, err := s.t.get(ctx, "discover/index", params)
	if err != nil {
		return nil, err
	}

	var body struct {
		Containers json.RawMessage `json:"containers"`
	}
	if err := json.Unmarshal(data, &body); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}
	return body.Containers, nil
}

// NewReleases returns new releases, optionally filtered by genre.
func (s *DiscoveryService) NewReleases(ctx context.Context, genreIDs []int, offset, limit int) (*PaginatedResult, error) {
	params := buildDiscoveryParams(genreIDs, offset, limit)
	data, err := s.t.get(ctx, "discover/newReleases", params)
	if err != nil {
		return nil, err
	}
	return ParsePaginated(data, "")
}

// CuratedPlaylists returns curated playlists, optionally filtered by genre.
func (s *DiscoveryService) CuratedPlaylists(ctx context.Context, genreIDs []int, offset, limit int) (*PaginatedResult, error) {
	params := buildDiscoveryParams(genreIDs, offset, limit)
	params["tags"] = ""
	data, err := s.t.get(ctx, "discover/playlists", params)
	if err != nil {
		return nil, err
	}
	return ParsePaginated(data, "")
}

// IdealDiscography returns ideal discography recommendations.
func (s *DiscoveryService) IdealDiscography(ctx context.Context, genreIDs []int, offset, limit int) (*PaginatedResult, error) {
	params := buildDiscoveryParams(genreIDs, offset, limit)
	data, err := s.t.get(ctx, "discover/idealDiscography", params)
	if err != nil {
		return nil, err
	}
	return ParsePaginated(data, "")
}

// AlbumOfTheWeek returns the album of the week.
func (s *DiscoveryService) AlbumOfTheWeek(ctx context.Context, genreIDs []int, offset, limit int) (*PaginatedResult, error) {
	params := buildDiscoveryParams(genreIDs, offset, limit)
	data, err := s.t.get(ctx, "discover/albumOfTheWeek", params)
	if err != nil {
		return nil, err
	}
	return ParsePaginated(data, "")
}

// buildDiscoveryParams creates the standard params for discovery endpoints.
func buildDiscoveryParams(genreIDs []int, offset, limit int) map[string]string {
	params := map[string]string{
		"offset": fmt.Sprintf("%d", offset),
		"limit":  fmt.Sprintf("%d", limit),
	}
	if len(genreIDs) > 0 {
		params["genre_ids"] = joinInts(genreIDs)
	}
	return params
}

// joinInts joins a slice of ints with commas.
func joinInts(ids []int) string {
	strs := make([]string, len(ids))
	for i, id := range ids {
		strs[i] = fmt.Sprintf("%d", id)
	}
	return strings.Join(strs, ",")
}
