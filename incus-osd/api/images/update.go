package images

import (
	"time"
)

// UpdateFull represents an update entry in the index.json/index.sjson file.
type UpdateFull struct {
	Update

	URL string `json:"url,omitempty"`
}

// Update represents the content of update.json/update.sjson.
type Update struct {
	Format string `json:"format"`

	Channels    []string       `json:"channels"`
	Files       []UpdateFile   `json:"files"`
	Origin      string         `json:"origin"`
	PublishedAt time.Time      `json:"published_at"` // In UTC.
	Severity    UpdateSeverity `json:"severity"`
	Version     string         `json:"version"`
}
