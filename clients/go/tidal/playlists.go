package tidal

import (
	"context"
	"fmt"
)

// PlaylistsService provides methods for Tidal playlist endpoints.
type PlaylistsService struct {
	t      *transport
	userID string
}

// Create creates a new playlist and returns it.
func (s *PlaylistsService) Create(ctx context.Context, name, description string, public bool) (*Playlist, error) {
	attrs := map[string]any{
		"name": name,
	}
	if description != "" {
		attrs["description"] = description
	}
	if public {
		attrs["accessType"] = "PUBLIC"
	} else {
		attrs["accessType"] = "UNLISTED"
	}

	payload := CreateResourcePayload("playlists", attrs)
	data, err := s.t.postJSON(ctx, "playlists", payload)
	if err != nil {
		return nil, err
	}

	pl, _, err := ParseOne[Playlist](data, func(p *Playlist, id string) { p.ID = id })
	if err != nil {
		return nil, fmt.Errorf("parse created playlist: %w", err)
	}
	return pl, nil
}

// Delete deletes a playlist by ID.
func (s *PlaylistsService) Delete(ctx context.Context, playlistID string) error {
	_, err := s.t.delete(ctx, fmt.Sprintf("playlists/%s", playlistID))
	return err
}

// Update updates an existing playlist.
func (s *PlaylistsService) Update(ctx context.Context, playlistID string, opts *PlaylistUpdateOptions) (*Playlist, error) {
	attrs := map[string]any{}
	if opts != nil {
		if opts.Name != nil {
			attrs["name"] = *opts.Name
		}
		if opts.Description != nil {
			attrs["description"] = *opts.Description
		}
		if opts.AccessType != nil {
			attrs["accessType"] = *opts.AccessType
		}
	}

	payload := UpdateResourcePayload("playlists", playlistID, attrs)
	data, err := s.t.patchJSON(ctx, fmt.Sprintf("playlists/%s", playlistID), payload)
	if err != nil {
		return nil, err
	}

	pl, _, err := ParseOne[Playlist](data, func(p *Playlist, id string) { p.ID = id })
	if err != nil {
		return nil, fmt.Errorf("parse updated playlist: %w", err)
	}
	return pl, nil
}

// Get fetches a single playlist by ID.
func (s *PlaylistsService) Get(ctx context.Context, playlistID string) (*Playlist, error) {
	data, err := s.t.get(ctx, fmt.Sprintf("playlists/%s", playlistID), map[string]string{})
	if err != nil {
		return nil, err
	}

	pl, _, err := ParseOne[Playlist](data, func(p *Playlist, id string) { p.ID = id })
	if err != nil {
		return nil, fmt.Errorf("parse playlist: %w", err)
	}
	return pl, nil
}

// List returns the authenticated user's playlists.
func (s *PlaylistsService) List(ctx context.Context, limit int) ([]Playlist, string, error) {
	params := map[string]string{
		"filter[owners.id]": "me",
	}
	data, err := s.t.get(ctx, "playlists", params)
	if err != nil {
		return nil, "", err
	}
	playlists, _, cursor, err := ParseMany[Playlist](data, func(p *Playlist, id string) { p.ID = id })
	return playlists, cursor, err
}

// GetItems returns tracks/videos in a playlist.
func (s *PlaylistsService) GetItems(ctx context.Context, playlistID string) ([]Track, string, error) {
	params := map[string]string{
		"include": "items",
	}
	data, err := s.t.get(ctx, fmt.Sprintf("playlists/%s/relationships/items", playlistID), params)
	if err != nil {
		return nil, "", err
	}
	return ParseRelationship[Track](data, func(t *Track, id string) { t.ID = id })
}

// AddTracks adds tracks to a playlist.
func (s *PlaylistsService) AddTracks(ctx context.Context, playlistID string, trackIDs []string) error {
	// Batch in groups of 20 (API limit)
	const batchSize = 20
	for i := 0; i < len(trackIDs); i += batchSize {
		end := i + batchSize
		if end > len(trackIDs) {
			end = len(trackIDs)
		}
		batch := trackIDs[i:end]
		payload := ResourcePayload("tracks", batch)
		_, err := s.t.postJSON(ctx, fmt.Sprintf("playlists/%s/relationships/items", playlistID), payload)
		if err != nil {
			return err
		}
	}
	return nil
}

// RemoveTracks removes tracks from a playlist.
func (s *PlaylistsService) RemoveTracks(ctx context.Context, playlistID string, trackIDs []string) error {
	payload := ResourcePayload("tracks", trackIDs)
	_, err := s.t.deleteJSON(ctx, fmt.Sprintf("playlists/%s/relationships/items", playlistID), payload)
	return err
}
