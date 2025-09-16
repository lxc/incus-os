package seed

import (
	"archive/tar"
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"

	"gopkg.in/yaml.v3"
)

// GetSeedPath defines the path to the expected seed configuration. It will first search for any
// disk with a "SEED_DATA" label, which would be externally provided by the user. If not found,
// defaults to the "seed-data" partition that exists on install media.
func GetSeedPath() string {
	_, err := os.Stat("/dev/disk/by-partlabel/SEED_DATA")
	if err == nil {
		return "/dev/disk/by-partlabel/SEED_DATA"
	}

	_, err = os.Stat("/dev/disk/by-label/SEED_DATA")
	if err == nil {
		return "/dev/disk/by-label/SEED_DATA"
	}

	return "/dev/disk/by-partlabel/seed-data"
}

// IsMissing checks whether the provided error is an expected error for missing seed data.
func IsMissing(e error) bool {
	for _, entry := range []error{ErrNoSeedPartition, ErrNoSeedData, ErrNoSeedSection} {
		if errors.Is(e, entry) {
			return true
		}
	}

	return false
}

// parseFileContents searches for a given file in the seed partition and returns its contents as a byte array if found.
func parseFileContents(partition string, filename string, target any) error {
	// Open the seed-data partition.
	f, err := os.Open(partition) //nolint:gosec
	if err != nil {
		if os.IsNotExist(err) {
			return ErrNoSeedPartition
		}

		return err
	}

	defer f.Close()

	// Check if seed-data is a tarball.
	header := make([]byte, 263)

	n, err := f.Read(header)
	if err != nil {
		return err
	}

	if n != 263 || !bytes.Equal(header[257:262], []byte{'u', 's', 't', 'a', 'r'}) {
		return ErrNoSeedData
	}

	_, err = f.Seek(0, 0)
	if err != nil {
		return err
	}

	// Parse the tarball.
	var hdr *tar.Header

	tr := tar.NewReader(f)
	for {
		// Get the next file.
		hdr, err = tr.Next()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return ErrNoSeedSection
			}

			return err
		}

		// Check if expected file.
		switch hdr.Name {
		case filename + ".json":
			decoder := json.NewDecoder(tr)

			err = decoder.Decode(target)
			if err != nil {
				return err
			}

			return nil

		case filename + ".yaml", filename + ".yml":
			decoder := yaml.NewDecoder(tr)

			err = decoder.Decode(target)
			if err != nil {
				return err
			}

			return nil

		default:
		}
	}
}
