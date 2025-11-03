package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	apiupdate "github.com/lxc/incus-os/incus-osd/api/images"
)

type cmdDemote struct {
	global *cmdGlobal
}

func (c *cmdDemote) command() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Use = "demote <path> <image> <channel>"
	cmd.Short = "Demotes a build from the target channel"
	cmd.Long = formatSection("Description",
		`Demotes a build from the target channel

This command is used to demote an existing build from being in
the specified channel.
`)
	cmd.RunE = c.run

	return cmd
}

func (c *cmdDemote) run(cmd *cobra.Command, args []string) error {
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
	if !slices.Contains(image.Channels, args[2]) {
		return fmt.Errorf("image %q isn't in channel %q", args[1], args[2])
	}

	newChannels := []string{}

	for _, channel := range image.Channels {
		if channel == args[2] {
			continue
		}

		newChannels = append(newChannels, channel)
	}

	image.Channels = newChannels

	// Remove the changelog(s).
	newFiles := []apiupdate.UpdateFile{}

	for _, file := range image.Files {
		if file.Type == apiupdate.UpdateFileTypeChangelog && strings.Contains(file.Filename, "changelog-"+args[2]+".yaml") {
			err := os.Remove(filepath.Join(args[0], args[1], file.Filename))
			if err != nil {
				return err
			}

			continue
		}

		newFiles = append(newFiles, file)
	}

	image.Files = newFiles

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
