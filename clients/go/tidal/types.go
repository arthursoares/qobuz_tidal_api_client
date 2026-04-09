package tidal

// Album represents a Tidal album parsed from JSON:API attributes.
type Album struct {
	ID              string   `json:"id"`
	Title           string   `json:"title"`
	Version         string   `json:"version,omitempty"`
	AlbumType       string   `json:"albumType"` // ALBUM, EP, SINGLE
	Duration        string   `json:"duration"`  // ISO 8601 e.g. "PT46M17S"
	DurationSeconds int      `json:"-"`          // computed from Duration
	NumberOfItems   int      `json:"numberOfItems"`
	NumberOfVolumes int      `json:"numberOfVolumes"`
	Explicit        bool     `json:"explicit"`
	ReleaseDate     string   `json:"releaseDate,omitempty"`
	BarcodeID       string   `json:"barcodeId,omitempty"`
	Popularity      float64  `json:"popularity"`
	MediaTags       []string `json:"mediaTags"` // e.g. ["HIRES_LOSSLESS", "LOSSLESS"]
	// Resolved from relationships/included
	ArtistNames []string `json:"-"`
	ImageURL    string   `json:"-"`
}

// IsHiRes returns true if the album has HIRES_LOSSLESS in its media tags.
func (a *Album) IsHiRes() bool {
	for _, tag := range a.MediaTags {
		if tag == "HIRES_LOSSLESS" {
			return true
		}
	}
	return false
}

// Track represents a Tidal track parsed from JSON:API attributes.
type Track struct {
	ID              string   `json:"id"`
	Title           string   `json:"title"`
	Version         string   `json:"version,omitempty"`
	Duration        string   `json:"duration"` // ISO 8601
	DurationSeconds int      `json:"-"`
	ISRC            string   `json:"isrc"`
	Explicit        bool     `json:"explicit"`
	Popularity      float64  `json:"popularity"`
	MediaTags       []string `json:"mediaTags"`
	// Resolved from relationships/included
	ArtistName string `json:"-"`
	AlbumTitle string `json:"-"`
	AlbumID    string `json:"-"`
}

// Artist represents a Tidal artist parsed from JSON:API attributes.
type Artist struct {
	ID         string  `json:"id"`
	Name       string  `json:"name"`
	Popularity float64 `json:"popularity"`
	Handle     string  `json:"handle,omitempty"`
}

// Playlist represents a Tidal playlist parsed from JSON:API attributes.
type Playlist struct {
	ID                string `json:"id"`
	Name              string `json:"name"`
	Description       string `json:"description,omitempty"`
	Duration          string `json:"duration,omitempty"` // ISO 8601
	NumberOfItems     int    `json:"numberOfItems"`
	NumberOfFollowers int    `json:"numberOfFollowers"`
	AccessType        string `json:"accessType"` // PUBLIC, UNLISTED
	PlaylistType      string `json:"playlistType"`
	Bounded           bool   `json:"bounded"`
	CreatedAt         string `json:"createdAt"`
	LastModifiedAt    string `json:"lastModifiedAt"`
}

// Genre represents a Tidal genre.
type Genre struct {
	ID   string `json:"id"`
	Name string `json:"genreName"`
}

// PlaylistUpdateOptions holds optional fields for updating a playlist.
type PlaylistUpdateOptions struct {
	Name        *string
	Description *string
	AccessType  *string // "PUBLIC" or "UNLISTED"
}
