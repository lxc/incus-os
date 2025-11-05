package main

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"

	apiupdate "github.com/lxc/incus-os/incus-osd/api/images"
	"github.com/lxc/incus-os/incus-osd/internal/manifests"
)

func generateChangelog(metaUpdate *apiupdate.Update, channel string, targetPath string) error {
	slog.Info("Preparing changelog", "version", metaUpdate.Version, "channel", channel)

	var metaIndex apiupdate.Index

	// Get information about currently published releases.
	// #nosec G304
	metaFile, err := os.Open(filepath.Join(targetPath, "../", "index.json"))
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	defer func() { _ = metaFile.Close() }()

	if err == nil {
		err := json.NewDecoder(metaFile).Decode(&metaIndex)
		if err != nil {
			return err
		}
	}

	priorVersion := ""

	// Get the prior release, if any. Assumes updates in the index are sorted in reverse order.
	for _, update := range metaIndex.Updates {
		if slices.Contains(update.Channels, channel) {
			priorVersion = update.Version

			break
		}
	}

	// Create a map of changelogs, one for each architecture.
	changelogs := make(map[apiupdate.UpdateFileArchitecture]apiupdate.Changelog)

	// For each manifest, generate its changelog entries.
	for _, f := range metaUpdate.Files {
		if f.Type == apiupdate.UpdateFileTypeImageManifest { //nolint:nestif
			parts := strings.Split(f.Filename, "/")
			archName := apiupdate.UpdateFileArchitecture(parts[0])
			filename := parts[1]

			_, exists := changelogs[archName]
			if !exists {
				changelogs[archName] = apiupdate.Changelog{
					CurrnetVersion: metaUpdate.Version,
					PriorVersion:   priorVersion,
					Channel:        channel,
					Components:     make(map[string]apiupdate.ChangelogEntries),
				}
			}

			var currentManifest manifests.IncusOSManifest

			var priorManifest manifests.IncusOSManifest

			// #nosec G304
			currentManifestFileGz, err := os.Open(filepath.Join(targetPath, f.Filename))
			if err != nil {
				return err
			}

			defer func() { _ = currentManifestFileGz.Close() }() //nolint:revive

			currentManifestFile, err := gzip.NewReader(currentManifestFileGz)
			if err != nil {
				return err
			}

			defer func() { _ = currentManifestFile.Close() }() //nolint:revive

			err = json.NewDecoder(currentManifestFile).Decode(&currentManifest)
			if err != nil {
				return err
			}

			if priorVersion != "" {
				// Replace the version string, if any, in the filename to use the previous version.
				priorFilename := strings.Replace(f.Filename, "_"+metaUpdate.Version, "_"+priorVersion, 1)

				// #nosec G304
				priorManifestFileGz, err := os.Open(filepath.Join(targetPath, "../", priorVersion, priorFilename))
				if err != nil && !os.IsNotExist(err) {
					return err
				}

				if err == nil {
					defer func() { _ = priorManifestFileGz.Close() }() //nolint:revive

					priorManifestFile, err := gzip.NewReader(priorManifestFileGz)
					if err != nil {
						return err
					}

					defer func() { _ = priorManifestFile.Close() }() //nolint:revive

					err = json.NewDecoder(priorManifestFile).Decode(&priorManifest)
					if err != nil {
						return err
					}
				}
			}

			diff := manifests.DiffManifests(priorManifest, currentManifest)
			if len(diff.Added) > 0 || len(diff.Updated) > 0 || len(diff.Removed) > 0 {
				componentName := strings.TrimSuffix(filename, ".manifest.json.gz")            // Trim the filename extension.
				componentName = strings.Replace(componentName, "_"+metaUpdate.Version, "", 1) // Trim any version string.

				changelogs[archName].Components[componentName] = diff
			}
		}
	}

	// Write each changelog as a gzip'ed yaml file and add it to the update metadata.
	for archName, changelog := range changelogs {
		contents, err := yaml.Marshal(&changelog)
		if err != nil {
			return err
		}

		var contentsGz bytes.Buffer

		gw := gzip.NewWriter(&contentsGz)

		_, err = gw.Write(contents)
		if err != nil {
			return err
		}

		err = gw.Close()
		if err != nil {
			return err
		}

		hash256 := sha256.New()

		_, err = hash256.Write(contentsGz.Bytes())
		if err != nil {
			return err
		}

		err = os.WriteFile(filepath.Join(targetPath, archName.String(), "changelog-"+channel+".yaml.gz"), contentsGz.Bytes(), 0o644)
		if err != nil {
			return err
		}

		metaUpdate.Files = append(metaUpdate.Files, apiupdate.UpdateFile{
			Architecture: archName,
			Component:    apiupdate.UpdateFileComponentOS,
			Filename:     filepath.Join(archName.String(), "changelog-"+channel+".yaml.gz"),
			Sha256:       hex.EncodeToString(hash256.Sum(nil)),
			Size:         int64(contentsGz.Len()),
			Type:         apiupdate.UpdateFileTypeChangelog,
		})
	}

	return nil
}
