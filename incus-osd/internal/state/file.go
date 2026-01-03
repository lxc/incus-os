package state

import (
	"log/slog"
	"os"

	"github.com/lxc/incus-os/incus-osd/api"
	"github.com/lxc/incus-os/incus-osd/internal/scheduling"
)

var currentStateVersion = 7

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

		Applications: map[string]api.Application{},

		JobScheduler: scheduler,
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

	err = os.WriteFile(s.path, body, 0o600)
	if err != nil {
		return err
	}

	return nil
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
