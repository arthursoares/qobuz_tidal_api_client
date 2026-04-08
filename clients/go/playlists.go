package qobuz

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

const playlistBatchSize = 50

// PlaylistsService provides methods for the Qobuz playlist/* endpoints.
type PlaylistsService struct {
	t *transport
}

// boolStr converts a Go bool to the "true"/"false" strings Qobuz expects.
func boolStr(v bool) string {
	if v {
		return "true"
	}
	return "false"
}

// Create creates a new playlist and returns it.
func (s *PlaylistsService) Create(ctx context.Context, name, description string, public, collaborative bool) (*Playlist, error) {
	data, err := s.t.postForm(ctx, "playlist/create", map[string]string{
		"name":             name,
		"description":      description,
		"is_public":        boolStr(public),
		"is_collaborative": boolStr(collaborative),
	})
	if err != nil {
		return nil, err
	}

	var p Playlist
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("parse playlist: %w", err)
	}
	return &p, nil
}

// Update updates an existing playlist. Only non-nil fields in opts are sent.
func (s *PlaylistsService) Update(ctx context.Context, playlistID int, opts *PlaylistUpdateOptions) (*Playlist, error) {
	form := map[string]string{
		"playlist_id": fmt.Sprintf("%d", playlistID),
	}
	if opts != nil {
		if opts.Name != nil {
			form["name"] = *opts.Name
		}
		if opts.Description != nil {
			form["description"] = *opts.Description
		}
		if opts.Public != nil {
			form["is_public"] = boolStr(*opts.Public)
		}
		if opts.Collaborative != nil {
			form["is_collaborative"] = boolStr(*opts.Collaborative)
		}
	}

	data, err := s.t.postForm(ctx, "playlist/update", form)
	if err != nil {
		return nil, err
	}

	var p Playlist
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("parse playlist: %w", err)
	}
	return &p, nil
}

// Delete deletes a playlist.
func (s *PlaylistsService) Delete(ctx context.Context, playlistID int) error {
	_, err := s.t.postForm(ctx, "playlist/delete", map[string]string{
		"playlist_id": fmt.Sprintf("%d", playlistID),
	})
	return err
}

// AddTracks adds tracks to a playlist, auto-batching in chunks of 50.
func (s *PlaylistsService) AddTracks(ctx context.Context, playlistID int, trackIDs []string, noDuplicate bool) error {
	for i := 0; i < len(trackIDs); i += playlistBatchSize {
		end := i + playlistBatchSize
		if end > len(trackIDs) {
			end = len(trackIDs)
		}
		batch := trackIDs[i:end]

		form := map[string]string{
			"playlist_id": fmt.Sprintf("%d", playlistID),
			"track_ids":   strings.Join(batch, ","),
		}
		if noDuplicate {
			form["no_duplicate"] = "true"
		}

		if _, err := s.t.postForm(ctx, "playlist/addTracks", form); err != nil {
			return err
		}
	}
	return nil
}

// Get fetches a single playlist by ID. Options can be nil for defaults
// (extra="tracks", offset=0, limit=50).
func (s *PlaylistsService) Get(ctx context.Context, playlistID int, opts *PlaylistGetOptions) (*Playlist, error) {
	extra := "tracks"
	offset := 0
	limit := 50
	if opts != nil {
		if opts.Extra != "" {
			extra = opts.Extra
		}
		if opts.Offset > 0 {
			offset = opts.Offset
		}
		if opts.Limit > 0 {
			limit = opts.Limit
		}
	}

	data, err := s.t.get(ctx, "playlist/get", map[string]string{
		"playlist_id": fmt.Sprintf("%d", playlistID),
		"extra":       extra,
		"offset":      fmt.Sprintf("%d", offset),
		"limit":       fmt.Sprintf("%d", limit),
	})
	if err != nil {
		return nil, err
	}

	var p Playlist
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("parse playlist: %w", err)
	}
	return &p, nil
}

// List returns the current user's playlists.
func (s *PlaylistsService) List(ctx context.Context, limit int) (*PaginatedResult, error) {
	data, err := s.t.get(ctx, "playlist/getUserPlaylists", map[string]string{
		"limit":  fmt.Sprintf("%d", limit),
		"filter": "owner",
	})
	if err != nil {
		return nil, err
	}
	return ParsePaginated(data, "playlists")
}

// Search searches for playlists.
func (s *PlaylistsService) Search(ctx context.Context, query string, limit, offset int) (*PaginatedResult, error) {
	data, err := s.t.get(ctx, "playlist/search", map[string]string{
		"query":  query,
		"limit":  fmt.Sprintf("%d", limit),
		"offset": fmt.Sprintf("%d", offset),
	})
	if err != nil {
		return nil, err
	}
	return ParsePaginated(data, "playlists")
}
