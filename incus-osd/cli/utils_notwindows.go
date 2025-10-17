//go:build (linux && !appengine) || darwin || freebsd || openbsd

package cli

import (
	"golang.org/x/sys/unix"
)

func getStdinFd() int {
	return unix.Stdin
}
