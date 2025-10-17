//go:build windows

package cli

import (
	"golang.org/x/sys/windows"
)

func getStdinFd() int {
	return int(windows.Stdin)
}
