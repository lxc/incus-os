package seed

// Applications represents the applications seed file.
type Applications struct {
	Version string `json:"version" yaml:"version"`

	Applications []Application `json:"applications" yaml:"applications"`
}

// Application represents a single application with the applications seed.
type Application struct {
	Name string `json:"name" yaml:"name"`
}
