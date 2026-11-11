// Helper utility to generate the license file shipped in each image.
package main

import (
	"log/slog"
	"os"

	"github.com/lxc/incus-os/incus-osd/internal/licenses"
)

func main() {
	if len(os.Args) != 4 {
		slog.Error("Usage: " + os.Args[0] + " <image> <image root dir> <local license dir>")
		os.Exit(1)
	}

	err := licenses.Generate(os.Args[1], os.Args[2], os.Args[3])
	if err != nil {
		slog.Error("Error: " + err.Error())
		os.Exit(1)
	}
}
