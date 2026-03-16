package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	apiupdate "github.com/lxc/incus-os/incus-osd/api/images"
)

type cmdSeverity struct {
	global *cmdGlobal
}

func (c *cmdSeverity) command() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Use = "severity <path> <image> <severity>"
	cmd.Short = "Sets the severity value for a build"
	cmd.Long = formatSection("Description",
		`Sets the severity value for a build

This command is used to set the update severity to an existing build.
`)
	cmd.RunE = c.run

	return cmd
}

func (c *cmdSeverity) run(cmd *cobra.Command, args []string) error {
	ctx := context.TODO()

	// Quick checks.
	exit, err := c.global.CheckArgs(cmd, args, 3, 3)
	if exit {
		return err
	}

	severity := apiupdate.UpdateSeverity(args[2])

	_, ok := apiupdate.UpdateSeverities[severity]
	if !ok {
		return fmt.Errorf("unsupported severity value %q", severity)
	}

	// Open the image metadata.
	metaPath := filepath.Join(args[0], args[1], "update.json")
	signedMetaPath := filepath.Join(args[0], args[1], "update.sjson")

	meta, err := os.OpenFile(metaPath, os.O_RDWR, 0)
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

	// Update the severity.
	image.Severity = severity

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

	err = meta.Close()
	if err != nil {
		return err
	}

	// Sign the updated metadata.
	err = sign(ctx, metaPath, signedMetaPath)
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
