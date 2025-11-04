package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"

	"github.com/spf13/cobra"

	apiupdate "github.com/lxc/incus-os/incus-osd/api/images"
)

type cmdPromote struct {
	global *cmdGlobal
}

func (c *cmdPromote) command() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Use = "promote <path> <image> <channel>"
	cmd.Short = "Promotes a build to a target channel"
	cmd.Long = formatSection("Description",
		`Promotes a build to a target channel

This command is used to promote an existing build (typically in the "testing" channel)
to appear in another channel (typically the "stable" channel).
`)
	cmd.RunE = c.run

	return cmd
}

func (c *cmdPromote) run(cmd *cobra.Command, args []string) error {
	ctx := context.TODO()

	// Quick checks.
	exit, err := c.global.CheckArgs(cmd, args, 3, 3)
	if exit {
		return err
	}

	// Open the image metadata.
	metaPath := filepath.Join(args[0], args[1], "update.json")

	meta, err := os.OpenFile(metaPath, os.O_RDWR, 0) //nolint:gosec
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("no such image %q", args[1])
		}

		return err
	}

	defer func() { _ = meta.Close() }()

	// Parse the current data.
	var image apiupdate.Update

	err = json.NewDecoder(meta).Decode(&image)
	if err != nil {
		return err
	}

	// Update the channel list.
	if slices.Contains(image.Channels, args[2]) {
		return fmt.Errorf("image %q is already in channel %q", args[1], args[2])
	}

	if image.Channels == nil {
		image.Channels = []string{}
	}

	image.Channels = append(image.Channels, args[2])

	// Generate changelog(s).
	err = generateChangelog(&image, args[2], filepath.Join(args[0], args[1]))
	if err != nil {
		return err
	}

	// Write the updated data.
	_, err = meta.Seek(0, io.SeekStart)
	if err != nil {
		return err
	}

	err = meta.Truncate(0)
	if err != nil {
		return err
	}

	err = json.NewEncoder(meta).Encode(image)
	if err != nil {
		return err
	}

	// Re-generate the index.
	err = generateIndex(ctx, args[0])
	if err != nil {
		return err
	}

	return nil
}
