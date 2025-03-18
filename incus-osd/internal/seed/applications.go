package seed

import (
	"context"
)

// Application represents an application.
type Application struct {
	Name string `json:"name"`
}

// Applications represents a list of application.
type Applications struct {
	Applications []Application `json:"applications"`
	Version      string        `json:"version"`
}

// GetApplications extracts the list of applications from the seed data.
func GetApplications(_ context.Context, partition string) (*Applications, error) {
	// Get applications list
	var apps Applications

	err := parseFileContents(partition, "applications", &apps)
	if err != nil {
		return nil, err
	}

	return &apps, nil
}
