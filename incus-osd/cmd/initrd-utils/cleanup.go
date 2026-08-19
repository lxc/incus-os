package main

import (
	"os"
	"path/filepath"
	"strings"
)

// cleanupAllowList lists the paths that are allowed to persist on the root partition.
var cleanupAllowList = []string{
	".deleted",
	"etc/group",
	"etc/gshadow",
	"etc/hostname",
	"etc/incusos-trusted-fuse-blown",
	"etc/iscsi/initiatorname.iscsi",
	"etc/localtime",
	"etc/machine-id",
	"etc/modprobe.d",
	"etc/netbird",
	"etc/nvme/hostid",
	"etc/nvme/hostnqn",
	"etc/openvswitch",
	"etc/passwd",
	"etc/shadow",
	"etc/ssl",
	"etc/sysctl.d",
	"var/lib/extensions",
	"var/lib/incus",
	"var/lib/incus-os",
	"var/lib/incus-os-extensions",
	"var/lib/linstor.d",
	"var/lib/migration-manager",
	"var/lib/netbird",
	"var/lib/openfga",
	"var/lib/operations-center",
	"var/lib/tailscale",
	"var/log/journal",
}

// cleanupRoot removes anything on the root partition that isn't part of the allow list.
// On the first run, identified by "/.deleted" not existing yet, everything is moved into
// "/.deleted" rather than deleted, allowing for recovery of unexpectedly removed data.
func cleanupRoot(root string) error {
	deletedPath := filepath.Join(root, ".deleted")

	_, err := os.Lstat(deletedPath)
	firstRun := err != nil

	if firstRun {
		err := os.Mkdir(deletedPath, 0o500)
		if err != nil {
			return err
		}
	}

	return cleanupTree(root, "", firstRun)
}

// cleanupTree removes anything under the provided directory that isn't needed by an
// entry in the allow list.
func cleanupTree(root string, dir string, firstRun bool) error {
	entries, err := os.ReadDir(filepath.Join(root, dir))
	if err != nil {
		return err
	}

	for _, entry := range entries {
		relPath := filepath.Join(dir, entry.Name())

		if isAllowed(relPath) {
			continue
		}

		// Descend into directories which hold allow list entries.
		if entry.IsDir() && holdsAllowed(relPath) {
			err := cleanupTree(root, relPath, firstRun)
			if err != nil {
				return err
			}

			continue
		}

		if firstRun {
			err := moveToDeleted(root, relPath)
			if err != nil {
				return err
			}

			continue
		}

		err := os.RemoveAll(filepath.Join(root, relPath))
		if err != nil {
			return err
		}
	}

	return nil
}

// moveToDeleted moves the provided path into "/.deleted", preserving its relative path.
func moveToDeleted(root string, relPath string) error {
	target := filepath.Join(root, ".deleted", relPath)

	err := os.MkdirAll(filepath.Dir(target), 0o500)
	if err != nil {
		return err
	}

	return os.Rename(filepath.Join(root, relPath), target)
}

// isAllowed checks if the provided path is part of or below an allow list entry.
func isAllowed(path string) bool {
	for _, allowed := range cleanupAllowList {
		if path == allowed || strings.HasPrefix(path, allowed+"/") {
			return true
		}
	}

	return false
}

// holdsAllowed checks if the provided path is a parent of an allow list entry.
func holdsAllowed(path string) bool {
	for _, allowed := range cleanupAllowList {
		if strings.HasPrefix(allowed, path+"/") {
			return true
		}
	}

	return false
}
