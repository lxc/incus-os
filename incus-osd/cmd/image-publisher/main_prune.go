package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"

	"github.com/spf13/cobra"

	apiupdate "github.com/lxc/incus-os/incus-osd/api/images"
)

type cmdPrune struct {
	global *cmdGlobal
}

func (c *cmdPrune) command() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Use = "prune <path> <count>"
	cmd.Short = "Prune the image server"
	cmd.Long = formatSection("Description",
		`Prunes the image server

This will prune images that aren't needed to satisfy the specified
per-channel retention policy.
`)
	cmd.RunE = c.run

	return cmd
}

func (c *cmdPrune) run(cmd *cobra.Command, args []string) error {
	ctx := context.TODO()

	// Quick checks.
	exit, err := c.global.CheckArgs(cmd, args, 2, 2)
	if exit {
		return err
	}

	// Parse the retention policy.
	retention, err := strconv.Atoi(args[1])
	if err != nil {
		return err
	}

	// Read the index.
	var metaIndex apiupdate.Index

	metaFile, err := os.Open(filepath.Join(args[0], "index.json"))
	if err != nil {
		return err
	}

	defer func() { _ = metaFile.Close() }()

	err = json.NewDecoder(metaFile).Decode(&metaIndex)
	if err != nil {
		return err
	}

	// Identify images to be deleted.
	imagesPerChannel := map[string]int{}

	for _, update := range metaIndex.Updates {
		used := false

		for _, channel := range update.Channels {
			if imagesPerChannel[channel] <= retention {
				imagesPerChannel[channel]++
				used = true
			}
		}

		if !used {
			slog.InfoContext(ctx, "Removing unused image", "image", update.Version)

			err = os.RemoveAll(filepath.Join(args[0], update.Version))
			if err != nil {
				return err
			}
		}
	}

	// Re-generate the index.
	return generateIndex(ctx, args[0])
}
