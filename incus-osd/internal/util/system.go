package util

import (
	"context"
	"regexp"

	"github.com/lxc/incus/v6/shared/subprocess"
)

// PriorBootRelease queries the systemd journal to get the version of
// IncusOS that was running in the prior boot.
func PriorBootRelease(ctx context.Context) string {
	// Ignore errors, because on first boot there won't be a prior boot journal to check.
	output, _ := subprocess.RunCommandContext(ctx, "journalctl", "-b", "-1", "-o", "cat", "-g", "System is ready version=", "-n", "1", "-u", "incus-osd")

	releaseRegex := regexp.MustCompile(`System is ready version=(\d+)`)
	releaseGroup := releaseRegex.FindStringSubmatch(output)

	if len(releaseGroup) != 2 {
		return ""
	}

	return releaseGroup[1]
}
