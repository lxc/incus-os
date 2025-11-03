package images

// Changelog represents the changes between two published IncusOS releases.
type Changelog struct {
	CurrnetVersion string                      `json:"current_version" yaml:"current_version"`
	PriorVersion   string                      `json:"prior_version"   yaml:"prior_version"`
	Channel        string                      `json:"channel"         yaml:"channel"`
	Components     map[string]ChangelogEntries `json:"components"      yaml:"components"`
}

// ChangelogEntries lists packages/artifacts added, updated, or removed for a given component.
type ChangelogEntries struct {
	Added   []string `json:"added,omitempty"   yaml:"added,omitempty"`
	Updated []string `json:"updated,omitempty" yaml:"updated,omitempty"`
	Removed []string `json:"removed,omitempty" yaml:"removed,omitempty"`
}
