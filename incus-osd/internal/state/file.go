package state

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/lxc/incus-os/incus-osd/internal/scheduling"
)

var currentStateVersion = 8

// LoadOrCreate parses the on-disk state file and returns a State struct.
// If no file exists, a new empty one is created.
func LoadOrCreate(path string) (*State, error) {
	scheduler, err := scheduling.NewScheduler()
	if err != nil {
		return nil, err
	}

	s := State{
		path: path,

		StateVersion: currentStateVersion,

		JobScheduler: scheduler,

		NetworkConfigurationChannel: make(chan error, 1),
	}

	body, err := os.ReadFile(s.path)
	if err == nil {
		err = Decode(body, nil, &s)

		return &s, err
	}

	if os.IsNotExist(err) {
		// Initialize with default values.
		err = s.initialize()
		if err != nil {
			return nil, err
		}

		// State file doesn't exist, create it and return it.
		err = s.Save()
		if err != nil {
			return nil, err
		}

		return &s, nil
	}

	return nil, err
}

// Save writes out the current state struct into its on-disk storage.
func (s *State) Save() error {
	// If we failed to fully load the existing state, refuse to save any changes to prevent accidental data loss.
	if len(s.UnrecognizedFields) > 0 {
		slog.Error("Refusing to save state because we previously failed to properly load the existing state")

		return nil
	}

	body, err := Encode(s)
	if err != nil {
		return err
	}

	// Write the state to a new file on disk. This allows us to perform an atomic rename
	// after ensuring all data is properly written to disk and that we didn't encounter an
	// issue like running out of disk space mid-write of the new state. If something did
	// go wrong updating the state, the prior state will still exist on disk so some recent
	// system configuration might be lost, but the system should remain operational.
	err = writeFile(s.path+".tmp", body)
	if err != nil {
		return err
	}

	// Overwrite the prior state file with the updated version.
	return os.Rename(s.path+".tmp", s.path)
}

func writeFile(filename string, body []byte) error {
	fd, err := os.Create(filename)
	if err != nil {
		return err
	}

	defer fd.Close()

	err = fd.Chmod(0o600)
	if err != nil {
		return err
	}

	count, err := fd.Write(body)
	if err != nil {
		return err
	}

	if count != len(body) {
		return fmt.Errorf("failed to write state file '%s': only wrote %d of %d bytes", filename, count, len(body))
	}

	// Ensure the file contents have been properly synced to disk.
	return fd.Sync()
}

// initialize sets default values for a new state file.
func (s *State) initialize() error {
	// Use the default update channel.
	s.System.Update.Config.Channel = "stable"

	// Set the initial update frequency to 6 hours.
	s.System.Update.Config.CheckFrequency = "6h"

	// Set the initial scrub schedule to weekly on sunday 4 AM.
	s.System.Storage.Config.ScrubSchedule = "0 4 * * 0"

	return nil
}
