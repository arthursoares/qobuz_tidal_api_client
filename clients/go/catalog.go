package qobuz

import (
	"context"
	"encoding/json"
	"fmt"
)

// CatalogService provides methods for album/*, artist/*, and track/* endpoints.
type CatalogService struct {
	t *transport
}

// GetAlbum fetches a single album by ID.
func (s *CatalogService) GetAlbum(ctx context.Context, albumID string) (*Album, error) {
	data, err := s.t.get(ctx, "album/get", map[string]string{
		"album_id": albumID,
		"extra":    "track_ids,albumsFromSameArtist",
	})
	if err != nil {
		return nil, err
	}

	var a Album
	if err := json.Unmarshal(data, &a); err != nil {
		return nil, fmt.Errorf("parse album: %w", err)
	}
	return &a, nil
}

// SearchAlbums searches albums by query string.
func (s *CatalogService) SearchAlbums(ctx context.Context, query string, limit, offset int) (*PaginatedResult, error) {
	data, err := s.t.get(ctx, "album/search", map[string]string{
		"query":  query,
		"limit":  fmt.Sprintf("%d", limit),
		"offset": fmt.Sprintf("%d", offset),
	})
	if err != nil {
		return nil, err
	}
	return ParsePaginated(data, "albums")
}

// SuggestAlbum returns similar albums for a given album ID.
func (s *CatalogService) SuggestAlbum(ctx context.Context, albumID string) ([]Album, error) {
	data, err := s.t.get(ctx, "album/suggest", map[string]string{
		"album_id": albumID,
	})
	if err != nil {
		return nil, err
	}

	var body struct {
		Albums struct {
			Items []json.RawMessage `json:"items"`
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
	return albums, nil
}

// GetArtistPage fetches a full artist page. Returns raw JSON because
// the response structure is complex and varied.
func (s *CatalogService) GetArtistPage(ctx context.Context, artistID int) (json.RawMessage, error) {
	data, err := s.t.get(ctx, "artist/page", map[string]string{
		"artist_id": fmt.Sprintf("%d", artistID),
		"sort":      "release_date",
	})
	if err != nil {
		return nil, err
	}
	return json.RawMessage(data), nil
}

// GetArtistReleases fetches paginated releases for an artist.
func (s *CatalogService) GetArtistReleases(ctx context.Context, artistID int, opts *ArtistReleasesOptions) (*PaginatedResult, error) {
	params := map[string]string{
		"artist_id":    fmt.Sprintf("%d", artistID),
		"release_type": "all",
		"offset":       "0",
		"limit":        "20",
		"sort":         "release_date_by_priority",
	}
	if opts != nil {
		if opts.ReleaseType != "" {
			params["release_type"] = opts.ReleaseType
		}
		if opts.Offset > 0 {
			params["offset"] = fmt.Sprintf("%d", opts.Offset)
		}
		if opts.Limit > 0 {
			params["limit"] = fmt.Sprintf("%d", opts.Limit)
		}
		if opts.Sort != "" {
			params["sort"] = opts.Sort
		}
	}

	data, err := s.t.get(ctx, "artist/getReleasesList", params)
	if err != nil {
		return nil, err
	}
	// artist/getReleasesList uses has_more-style pagination (top-level items)
	return ParsePaginated(data, "")
}

// SearchArtists searches artists by query string.
func (s *CatalogService) SearchArtists(ctx context.Context, query string, limit, offset int) (*PaginatedResult, error) {
	data, err := s.t.get(ctx, "artist/search", map[string]string{
		"query":  query,
		"limit":  fmt.Sprintf("%d", limit),
		"offset": fmt.Sprintf("%d", offset),
	})
	if err != nil {
		return nil, err
	}
	return ParsePaginated(data, "artists")
}

// GetTrack fetches a single track by ID.
func (s *CatalogService) GetTrack(ctx context.Context, trackID int) (*Track, error) {
	data, err := s.t.get(ctx, "track/get", map[string]string{
		"track_id": fmt.Sprintf("%d", trackID),
	})
	if err != nil {
		return nil, err
	}

	var t Track
	if err := json.Unmarshal(data, &t); err != nil {
		return nil, fmt.Errorf("parse track: %w", err)
	}
	return &t, nil
}

// GetTracks batch-fetches tracks by IDs via POST track/getList.
func (s *CatalogService) GetTracks(ctx context.Context, trackIDs []int) ([]Track, error) {
	if len(trackIDs) == 0 {
		return nil, nil
	}

	data, err := s.t.postJSON(ctx, "track/getList", map[string]interface{}{
		"tracks_id": trackIDs,
	})
	if err != nil {
		return nil, err
	}

	var body struct {
		Tracks struct {
			Items []json.RawMessage `json:"items"`
		} `json:"tracks"`
	}
	if err := json.Unmarshal(data, &body); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	tracks := make([]Track, 0, len(body.Tracks.Items))
	for _, raw := range body.Tracks.Items {
		var t Track
		if err := json.Unmarshal(raw, &t); err != nil {
			return nil, fmt.Errorf("parse track: %w", err)
		}
		tracks = append(tracks, t)
	}
	return tracks, nil
}

// SearchTracks searches tracks by query string.
func (s *CatalogService) SearchTracks(ctx context.Context, query string, limit, offset int) (*PaginatedResult, error) {
	data, err := s.t.get(ctx, "track/search", map[string]string{
		"query":  query,
		"limit":  fmt.Sprintf("%d", limit),
		"offset": fmt.Sprintf("%d", offset),
	})
	if err != nil {
		return nil, err
	}
	return ParsePaginated(data, "tracks")
}
