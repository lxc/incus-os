package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
)

// ErrTraceInProgress is returned when a block device trace is already running.
var ErrTraceInProgress = errors.New("a block device trace is already running")

// ErrTraceInvalid is returned when a block device trace request can't be satisfied.
var ErrTraceInvalid = errors.New("invalid block device trace request")

// TraceActions lists the I/O categories which can be selected when tracing a block device.
var TraceActions = []string{"read", "write", "flush", "sync", "queue", "requeue", "issue", "complete", "fs", "pc", "notify", "ahead", "meta", "discard", "drv_data", "fua"}

const tracingPath = "/sys/kernel/tracing"

// The kernel only has a single block tracer.
var traceMutex sync.Mutex

// TraceBlockDevice writes the I/O events of a block device to the writer until the context is done.
//
// This relies on the kernel's own block tracer as the debugfs interface used by blktrace
// isn't accessible once the kernel is locked down.
func TraceBlockDevice(ctx context.Context, device string, actions []string, w io.Writer) error {
	traceDir, err := traceDirectory(device)
	if err != nil {
		return err
	}

	// An empty mask would otherwise silently trace nothing.
	mask := "0xffff"

	if len(actions) > 0 {
		for _, action := range actions {
			if !slices.Contains(TraceActions, action) {
				return fmt.Errorf("%w: unknown action %q", ErrTraceInvalid, action)
			}
		}

		mask = strings.Join(actions, ",")
	}

	if !traceMutex.TryLock() {
		return ErrTraceInProgress
	}

	defer traceMutex.Unlock()

	// Enable tracing on the device and restrict it to the requested actions.
	err = os.WriteFile(filepath.Join(traceDir, "enable"), []byte("1"), 0o644)
	if err != nil {
		return err
	}

	defer func() { _ = os.WriteFile(filepath.Join(traceDir, "enable"), []byte("0"), 0o644) }()

	err = os.WriteFile(filepath.Join(traceDir, "act_mask"), []byte(mask), 0o644)
	if err != nil {
		return err
	}

	// Switch to the block tracer, this also clears the trace buffer.
	err = os.WriteFile(filepath.Join(tracingPath, "current_tracer"), []byte("blk"), 0o644)
	if err != nil {
		return err
	}

	defer func() { _ = os.WriteFile(filepath.Join(tracingPath, "current_tracer"), []byte("nop"), 0o644) }()

	err = os.WriteFile(filepath.Join(tracingPath, "tracing_on"), []byte("1"), 0o644)
	if err != nil {
		return err
	}

	// Reading the pipe blocks, so use a command which can be killed when done.
	cmd := exec.CommandContext(ctx, "cat", tracingPath+"/trace_pipe")
	cmd.Stdout = w

	err = cmd.Run()
	if err != nil && ctx.Err() == nil {
		return err
	}

	return nil
}

// traceDirectory returns the sysfs directory controlling the tracing of a block device.
func traceDirectory(device string) (string, error) {
	if device == "" {
		return "", fmt.Errorf("%w: missing device", ErrTraceInvalid)
	}

	if !strings.HasPrefix(device, "/") {
		device = filepath.Join("/dev", device)
	}

	// Resolve aliases such as those under /dev/disk or /dev/mapper.
	device, err := filepath.EvalSymlinks(device)
	if err != nil {
		return "", fmt.Errorf("%w: %w", ErrTraceInvalid, err)
	}

	info, err := os.Stat(device)
	if err != nil {
		return "", fmt.Errorf("%w: %w", ErrTraceInvalid, err)
	}

	if info.Mode()&os.ModeDevice == 0 || info.Mode()&os.ModeCharDevice != 0 {
		return "", fmt.Errorf("%w: %q isn't a block device", ErrTraceInvalid, device)
	}

	traceDir := filepath.Join("/sys/class/block", filepath.Base(device), "trace")

	_, err = os.Stat(traceDir)
	if err != nil {
		return "", fmt.Errorf("%w: %q can't be traced", ErrTraceInvalid, device)
	}

	_, err = os.Stat(filepath.Join(tracingPath, "current_tracer"))
	if err != nil {
		return "", errors.New("kernel tracing isn't available")
	}

	return traceDir, nil
}
