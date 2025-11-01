// Helper utility to generate manifests for each image created.
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/lxc/incus-os/incus-osd/internal/manifests"
)

func main() {
	if len(os.Args) != 4 {
		fmt.Printf("Usage: " + os.Args[0] + " <mkosi.output dir> <app-build dir> <output dir>\n")
		os.Exit(1)
	}

	m, err := manifests.ReadManifests(os.Args[1], []string{"base", "debug", "incus", "migration-manager", "operations-center"})
	if err != nil {
		fmt.Printf("Error: %s\n", err.Error())
		os.Exit(1)
	}

	m, err = manifests.GenerateManifests(context.Background(), os.Args[2], m)
	if err != nil {
		fmt.Printf("Error: %s\n", err.Error())
		os.Exit(1)
	}

	err = manifests.WriteManifests(os.Args[3], m)
	if err != nil {
		fmt.Printf("Error: %s\n", err.Error())
		os.Exit(1)
	}
}
