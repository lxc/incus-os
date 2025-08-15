package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strconv"

	apiupdate "github.com/lxc/incus-os/incus-osd/api/images"
)

func generateIndex(ctx context.Context, targetPath string) error {
	// Prepare the index.
	metaIndex := apiupdate.Index{
		Format:  "1.0",
		Updates: []apiupdate.UpdateFull{},
	}

	// Go through all current updates.
	files, err := os.ReadDir(targetPath)
	if err != nil {
		return err
	}

	for _, entry := range files {
		if !entry.IsDir() {
			continue
		}

		updateFile, err := os.Open(filepath.Join(targetPath, entry.Name(), "update.json")) //nolint:gosec
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}

			return err
		}

		var update apiupdate.Update

		err = json.NewDecoder(updateFile).Decode(&update)
		if err != nil {
			return err
		}

		err = updateFile.Close()
		if err != nil {
			return err
		}

		metaIndex.Updates = append(metaIndex.Updates, apiupdate.UpdateFull{URL: "/" + entry.Name(), Update: update})
	}

	// Sort the updates.
	slices.SortFunc(metaIndex.Updates, func(a apiupdate.UpdateFull, b apiupdate.UpdateFull) int {
		aVersion, err := strconv.Atoi(a.Version)
		if err != nil {
			return -1
		}

		bVersion, err := strconv.Atoi(b.Version)
		if err != nil {
			return -1
		}

		if aVersion == bVersion {
			return 0
		}

		if aVersion < bVersion {
			return -1
		}

		return 1
	})

	wr, err := os.Create(filepath.Join(targetPath, "index.json")) //nolint:gosec
	if err != nil {
		return err
	}

	defer func() { _ = wr.Close() }()

	err = json.NewEncoder(wr).Encode(metaIndex)
	if err != nil {
		return err
	}

	err = sign(ctx, filepath.Join(targetPath, "index.json"), filepath.Join(targetPath, "index.sjson"))
	if err != nil {
		return err
	}

	return nil
}
