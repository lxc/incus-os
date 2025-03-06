package seed

import (
	"archive/tar"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
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
func GetApplications(_ context.Context) (*Applications, error) {
	// Open the seed-data partition.
	f, err := os.Open(seedPartitionPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNoSeedPartition
		}

		return nil, err
	}

	defer f.Close()

	// Check if seed-data is a tarball.
	header := make([]byte, 263)
	n, err := f.Read(header)
	if err != nil {
		return nil, err
	}

	if n != 263 || !bytes.Equal(header[257:262], []byte{'u', 's', 't', 'a', 'r'}) {
		return nil, ErrNoSeedData
	}

	_, err = f.Seek(0, 0)
	if err != nil {
		return nil, err
	}

	// Parse the tarball.
	var hdr *tar.Header

	tr := tar.NewReader(f)
	for {
		// Get the next file.
		hdr, err = tr.Next()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil, ErrNoSeedSection
			}

			return nil, err
		}

		// Check if expected file.
		if hdr.Name == "applications.json" {
			break
		}
	}

	// Parse applications.json into Applications.
	var apps Applications

	decoder := json.NewDecoder(tr)
	err = decoder.Decode(&apps)
	if err != nil {
		return nil, err
	}

	return &apps, nil
}
