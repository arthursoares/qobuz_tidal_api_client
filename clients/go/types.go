package qobuz

import (
	"encoding/json"
)

// ImageSet holds the three image sizes returned by Qobuz.
type ImageSet struct {
	Small     *string `json:"small"`
	Thumbnail *string `json:"thumbnail"`
	Large     *string `json:"large"`
}

// ArtistSummary is a minimal artist reference.
// The Qobuz API sometimes returns "name" as a string and sometimes as {"display": "..."}.
type ArtistSummary struct {
	ID   int    `json:"id"`
	Name string `json:"-"` // handled by custom UnmarshalJSON
}

type artistSummaryJSON struct {
	ID      int             `json:"id"`
	RawName json.RawMessage `json:"name"`
}

// UnmarshalJSON handles both string and {"display": "..."} name formats.
func (a *ArtistSummary) UnmarshalJSON(data []byte) error {
	var raw artistSummaryJSON
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	a.ID = raw.ID

	if len(raw.RawName) == 0 {
		a.Name = "Unknown"
		return nil
	}

	// Try string first
	var nameStr string
	if err := json.Unmarshal(raw.RawName, &nameStr); err == nil {
		a.Name = nameStr
		return nil
	}

	// Try {"display": "..."}
	var nameObj struct {
		Display string `json:"display"`
	}
	if err := json.Unmarshal(raw.RawName, &nameObj); err == nil {
		a.Name = nameObj.Display
		return nil
	}

	a.Name = "Unknown"
	return nil
}

// MarshalJSON serializes ArtistSummary with name as a plain string.
func (a ArtistSummary) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}{ID: a.ID, Name: a.Name})
}

// ArtistRole is an artist with their roles on a release.
type ArtistRole struct {
	ID    int      `json:"id"`
	Name  string   `json:"name"`
	Roles []string `json:"roles"`
}

// Label represents a record label.
type Label struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// Genre represents a music genre.
type Genre struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Color string `json:"color"`
	Path  []int  `json:"path"`
	Slug  string `json:"slug"`
}

// AudioInfo holds audio quality metadata for a track.
type AudioInfo struct {
	MaximumBitDepth      int     `json:"maximum_bit_depth"`
	MaximumChannelCount  int     `json:"maximum_channel_count"`
	MaximumSamplingRate  float64 `json:"maximum_sampling_rate"`
}

// Rights describes what operations are permitted on a resource.
type Rights struct {
	Streamable       bool `json:"streamable"`
	Downloadable     bool `json:"downloadable"`
	HiresStreamable  bool `json:"hires_streamable"`
	Purchasable      bool `json:"purchasable"`
}

// AlbumSummary is a minimal album reference (used inside Track).
type AlbumSummary struct {
	ID    string   `json:"id"`
	Title string   `json:"title"`
	Image ImageSet `json:"image"`
}

// UnmarshalJSON handles the case where album ID may be an integer or string.
func (a *AlbumSummary) UnmarshalJSON(data []byte) error {
	type Alias AlbumSummary
	var raw struct {
		Alias
		RawID json.RawMessage `json:"id"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	*a = AlbumSummary(raw.Alias)

	// Try string first
	var idStr string
	if err := json.Unmarshal(raw.RawID, &idStr); err == nil {
		a.ID = idStr
		return nil
	}
	// Try number
	var idNum json.Number
	if err := json.Unmarshal(raw.RawID, &idNum); err == nil {
		a.ID = idNum.String()
		return nil
	}

	return nil
}

// UserSummary is a minimal user reference.
type UserSummary struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// UnmarshalJSON handles both "name" and "display_name" fields.
func (u *UserSummary) UnmarshalJSON(data []byte) error {
	var raw struct {
		ID          int    `json:"id"`
		Name        string `json:"name"`
		DisplayName string `json:"display_name"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	u.ID = raw.ID
	u.Name = raw.Name
	if u.Name == "" {
		u.Name = raw.DisplayName
	}
	return nil
}

// Award represents an editorial award for an album.
type Award struct {
	ID        int     `json:"id"`
	Name      string  `json:"name"`
	AwardedAt *string `json:"awarded_at"`
}

// Album represents a full album response from the Qobuz API.
type Album struct {
	ID                  string        `json:"-"` // handled by custom UnmarshalJSON
	Title               string        `json:"title"`
	Version             *string       `json:"version"`
	Artist              ArtistSummary `json:"artist"`
	Artists             []ArtistRole  `json:"artists"`
	Image               ImageSet      `json:"image"`
	Duration            int           `json:"duration"`
	TracksCount         int           `json:"tracks_count"`
	MaximumBitDepth     int           `json:"maximum_bit_depth"`
	MaximumSamplingRate float64       `json:"maximum_sampling_rate"`
	MaximumChannelCount int           `json:"maximum_channel_count"`
	Streamable          bool          `json:"streamable"`
	Downloadable        bool          `json:"downloadable"`
	Hires               bool          `json:"hires"`
	HiresStreamable     bool          `json:"hires_streamable"`
	ReleaseDateOriginal *string       `json:"release_date_original"`
	UPC                 *string       `json:"upc"`
	Label               *Label        `json:"label"`
	Genre               *Genre        `json:"genre"`
	Description         *string       `json:"description"`
	Awards              []Award       `json:"awards"`
}

// UnmarshalJSON handles the case where album ID may be string or integer.
func (a *Album) UnmarshalJSON(data []byte) error {
	// Use an alias to avoid recursive UnmarshalJSON call
	type Alias Album
	var raw struct {
		Alias
		RawID json.RawMessage `json:"id"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	*a = Album(raw.Alias)

	// Try string first
	var idStr string
	if err := json.Unmarshal(raw.RawID, &idStr); err == nil {
		a.ID = idStr
		return nil
	}
	// Try number
	var idNum json.Number
	if err := json.Unmarshal(raw.RawID, &idNum); err == nil {
		a.ID = idNum.String()
		return nil
	}
	return nil
}

// MarshalJSON serializes Album with ID as a string.
func (a Album) MarshalJSON() ([]byte, error) {
	type Alias Album
	return json.Marshal(struct {
		ID string `json:"id"`
		Alias
	}{
		ID:    a.ID,
		Alias: Alias(a),
	})
}

// Track represents a full track response from the Qobuz API.
type Track struct {
	ID          int           `json:"id"`
	Title       string        `json:"title"`
	Version     *string       `json:"version"`
	Duration    int           `json:"duration"`
	TrackNumber int           `json:"track_number"`
	DiscNumber  int           `json:"media_number"`
	Explicit    bool          `json:"parental_warning"`
	Performer   ArtistSummary `json:"performer"`
	Album       AlbumSummary  `json:"album"`
	AudioInfo   AudioInfo     `json:"audio_info"`
	Rights      Rights        `json:"rights"`
	ISRC        *string       `json:"isrc"`
}

// Playlist represents a Qobuz playlist.
type Playlist struct {
	ID              int         `json:"id"`
	Name            string      `json:"name"`
	Description     string      `json:"description"`
	TracksCount     int         `json:"tracks_count"`
	UsersCount      int         `json:"users_count"`
	Duration        int         `json:"duration"`
	IsPublic        bool        `json:"is_public"`
	IsCollaborative bool        `json:"is_collaborative"`
	PublicAt        any `json:"public_at"` // int (unix timestamp) or bool (false)
	CreatedAt       int         `json:"created_at"`
	UpdatedAt       int         `json:"updated_at"`
	Owner           UserSummary `json:"owner"`
}

// FavoriteIds holds all favorite resource IDs.
type FavoriteIds struct {
	Albums  []string `json:"-"` // custom unmarshal to ensure string IDs
	Tracks  []int    `json:"tracks"`
	Artists []int    `json:"artists"`
	Labels  []int    `json:"labels"`
	Awards  []int    `json:"awards"`
}

// UnmarshalJSON handles album IDs that may be strings or numbers.
func (f *FavoriteIds) UnmarshalJSON(data []byte) error {
	var raw struct {
		Albums  []json.RawMessage `json:"albums"`
		Tracks  []int             `json:"tracks"`
		Artists []int             `json:"artists"`
		Labels  []int             `json:"labels"`
		Awards  []int             `json:"awards"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	f.Tracks = raw.Tracks
	f.Artists = raw.Artists
	f.Labels = raw.Labels
	f.Awards = raw.Awards

	f.Albums = make([]string, 0, len(raw.Albums))
	for _, rawID := range raw.Albums {
		var s string
		if err := json.Unmarshal(rawID, &s); err == nil {
			f.Albums = append(f.Albums, s)
			continue
		}
		var n json.Number
		if err := json.Unmarshal(rawID, &n); err == nil {
			f.Albums = append(f.Albums, n.String())
		}
	}
	return nil
}

// LastUpdate holds timestamps for various library sections.
type LastUpdate struct {
	Favorite       int `json:"favorite"`
	FavoriteAlbum  int `json:"favorite_album"`
	FavoriteArtist int `json:"favorite_artist"`
	FavoriteTrack  int `json:"favorite_track"`
	FavoriteLabel  int `json:"favorite_label"`
	Playlist       int `json:"playlist"`
	Purchase       int `json:"purchase"`
}

// lastUpdateWrapper handles the {"last_update": {...}} envelope.
type lastUpdateWrapper struct {
	LastUpdate LastUpdate `json:"last_update"`
}

// FileUrl represents a streaming file URL response.
type FileUrl struct {
	TrackID      int               `json:"track_id"`
	FormatID     int               `json:"format_id"`
	MimeType     string            `json:"mime_type"`
	SamplingRate int               `json:"sampling_rate"`
	BitsDepth    int               `json:"bits_depth"`
	Duration     float64           `json:"duration"`
	URLTemplate  string            `json:"url_template"`
	NSegments    int               `json:"n_segments"`
	KeyID        *string           `json:"key_id"`
	Key          *string           `json:"key"`
	Blob         *string           `json:"blob"`
	Restrictions []json.RawMessage `json:"restrictions"`
}

// Session represents a streaming session.
type Session struct {
	SessionID string `json:"session_id"`
	Profile   string `json:"profile"`
	ExpiresAt int    `json:"expires_at"`
}

// PaginatedResult wraps a paginated API response.
type PaginatedResult struct {
	Items   []json.RawMessage `json:"items"`
	Total   *int              `json:"total"`   // nil for has_more-style pagination
	Limit   int               `json:"limit"`
	Offset  int               `json:"offset"`
	HasMore bool              `json:"has_more"`
}

// ParsePaginated parses a paginated response from raw JSON bytes.
// key is the container key (e.g., "albums", "tracks", "playlists").
// If key is empty, it looks for top-level items/has_more (discovery style).
func ParsePaginated(data []byte, key string) (*PaginatedResult, error) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}

	if key != "" {
		if containerRaw, ok := raw[key]; ok {
			var container struct {
				Items  []json.RawMessage `json:"items"`
				Total  *int              `json:"total"`
				Limit  int               `json:"limit"`
				Offset int               `json:"offset"`
			}
			if err := json.Unmarshal(containerRaw, &container); err != nil {
				return nil, err
			}
			limit := container.Limit
			if limit == 0 {
				limit = 500
			}
			hasMore := false
			if container.Total != nil {
				hasMore = container.Offset+limit < *container.Total
			}
			return &PaginatedResult{
				Items:   container.Items,
				Total:   container.Total,
				Limit:   limit,
				Offset:  container.Offset,
				HasMore: hasMore,
			}, nil
		}
	}

	// Try top-level items/has_more style
	var topLevel struct {
		Items   []json.RawMessage `json:"items"`
		HasMore bool              `json:"has_more"`
	}
	if err := json.Unmarshal(data, &topLevel); err != nil {
		return nil, err
	}
	if topLevel.Items != nil {
		return &PaginatedResult{
			Items:   topLevel.Items,
			Total:   nil,
			Limit:   len(topLevel.Items),
			Offset:  0,
			HasMore: topLevel.HasMore,
		}, nil
	}

	return &PaginatedResult{
		Items:   nil,
		Total:   intPtr(0),
		Limit:   0,
		Offset:  0,
		HasMore: false,
	}, nil
}

// FavoriteAlbums is a paginated list of favorite albums with parsed Album objects.
type FavoriteAlbums struct {
	Items  []Album
	Total  int
	Limit  int
	Offset int
}

// PlaylistUpdateOptions holds optional fields for updating a playlist.
type PlaylistUpdateOptions struct {
	Name          *string
	Description   *string
	Public        *bool
	Collaborative *bool
}

// ArtistReleasesOptions holds optional parameters for GetArtistReleases.
type ArtistReleasesOptions struct {
	ReleaseType string // "all", "album", "single", etc. Default: "all"
	Offset      int
	Limit       int // Default: 20
	Sort        string // Default: "release_date_by_priority"
}

func intPtr(v int) *int {
	return &v
}
