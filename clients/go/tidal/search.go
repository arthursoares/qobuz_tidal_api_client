package tidal

import (
	"context"
	"fmt"
	"net/url"
)

// SearchService provides methods for the Tidal search endpoints.
type SearchService struct {
	t *transport
}

// Albums searches for albums matching the query.
func (s *SearchService) Albums(ctx context.Context, query string, limit int) ([]Album, string, error) {
	params := map[string]string{
		"include": "albums",
	}
	data, err := s.t.get(ctx, fmt.Sprintf("searchResults/%s/relationships/albums", url.PathEscape(query)), params)
	if err != nil {
		return nil, "", err
	}
	return ParseRelationship[Album](data, func(a *Album, id string) { a.ID = id })
}

// Tracks searches for tracks matching the query.
func (s *SearchService) Tracks(ctx context.Context, query string, limit int) ([]Track, string, error) {
	params := map[string]string{
		"include": "tracks",
	}
	data, err := s.t.get(ctx, fmt.Sprintf("searchResults/%s/relationships/tracks", url.PathEscape(query)), params)
	if err != nil {
		return nil, "", err
	}
	return ParseRelationship[Track](data, func(t *Track, id string) { t.ID = id })
}

// Artists searches for artists matching the query.
func (s *SearchService) Artists(ctx context.Context, query string, limit int) ([]Artist, string, error) {
	params := map[string]string{
		"include": "artists",
	}
	data, err := s.t.get(ctx, fmt.Sprintf("searchResults/%s/relationships/artists", url.PathEscape(query)), params)
	if err != nil {
		return nil, "", err
	}
	return ParseRelationship[Artist](data, func(a *Artist, id string) { a.ID = id })
}
