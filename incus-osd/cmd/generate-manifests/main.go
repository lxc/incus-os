// Helper utility to generate manifests for each image created.
package main

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/lxc/incus-os/incus-osd/internal/manifests"
)

func main() {
	if len(os.Args) != 2 {
		slog.Error("Usage: " + os.Args[0] + " <repo base dir>")
		os.Exit(1)
	}

	dirEntries, err := os.ReadDir(filepath.Join(os.Args[1], "mkosi.images/"))
	if err != nil {
		slog.Error("Error: " + err.Error())
		os.Exit(1)
	}

	// The list of images to generate manifests for. We assume base will always be first, so manually
	// insert it and then skip it when iterating through the other images.
	images := []string{"base"}

	for _, dir := range dirEntries {
		if !dir.IsDir() {
			continue
		}

		if dir.Name() == "base" {
			continue
		}

		images = append(images, dir.Name())
	}

	m, err := manifests.ReadManifests(filepath.Join(os.Args[1], "mkosi.output/"), images)
	if err != nil {
		slog.Error("Error: " + err.Error())
		os.Exit(1)
	}

	m, err = manifests.GenerateManifests(context.Background(), os.Args[1], m)
	if err != nil {
		slog.Error("Error: " + err.Error())
		os.Exit(1)
	}

	err = manifests.WriteManifests(filepath.Join(os.Args[1], "upload/"), m)
	if err != nil {
		slog.Error("Error: " + err.Error())
		os.Exit(1)
	}
}
