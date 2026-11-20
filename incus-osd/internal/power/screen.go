package power

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strconv"

	"github.com/lxc/incus-os/incus-osd/api"
)

const consoleTTY = "/dev/tty1"

// validatePowerConfigurationScreen validates power configuration for the screen.
func validatePowerConfigurationScreen(config api.SystemPowerConfigScreen) error {
	// The kernel caps the blank timeout at 60 minutes.
	if config.BlankTimeout < 0 || config.BlankTimeout > 60 {
		return fmt.Errorf("%w: screen blank timeout must be between 0 and 60 minutes", ErrInvalidConfig)
	}

	return nil
}

// applyScreenBlankTimeout sets the console blank interval in minutes.
func applyScreenBlankTimeout(minutes int) error {
	// Skip if can't find tty1.
	_, err := os.Stat(consoleTTY)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}

		return err
	}

	// Set the blanking interval through the "ESC[9;n]" escape sequence.
	return os.WriteFile(consoleTTY, fmt.Appendf([]byte{0x1B}, "[9;%d]", minutes), 0o600)
}

// getPowerStateScreen returns current power management related state for the screen.
func getPowerStateScreen() (api.SystemPowerStateScreen, error) {
	blankInterval, err := readSysfsString("/sys/module/kernel/parameters/consoleblank")
	if err != nil {
		return api.SystemPowerStateScreen{}, err
	}

	if blankInterval == "" {
		return api.SystemPowerStateScreen{}, nil
	}

	seconds, err := strconv.Atoi(blankInterval)
	if err != nil {
		return api.SystemPowerStateScreen{}, err
	}

	return api.SystemPowerStateScreen{
		BlankTimeout: seconds / 60,
	}, nil
}
