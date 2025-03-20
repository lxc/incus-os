package state

// Application represents an installed application (system extension).
type Application struct {
	Initialized bool   `json:"initialized"`
	Version     string `json:"version"`
}

// State represents the on-disk persistent state.
type State struct {
	path string

	Applications   map[string]Application `json:"applications"`
	RunningRelease string                 `json:"running_release"`
}
