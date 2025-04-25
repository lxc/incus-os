package providers

import (
	"errors"
)

// ErrProviderUnavailable is returned when a provider isn't ready for use yet.
var ErrProviderUnavailable = errors.New("provider isn't currently available")

// ErrNoUpdateAvailable is returned if no OS or application update is available.
var ErrNoUpdateAvailable = errors.New("no update available")
