package providers

import (
	"errors"
)

// ErrProviderUnavailable is returned when a provider isn't ready for use yet.
var ErrProviderUnavailable = errors.New("provider isn't currently available")

// ErrNoUpdateAvailable is returned if no OS or application update is available.
var ErrNoUpdateAvailable = errors.New("no update available")

// ErrRegistrationUnsupported is returned if the provider doesn't (currently) support registration.
var ErrRegistrationUnsupported = errors.New("registration unsupported")

// ErrDeregistrationUnsupported is returned if the provider doesn't (currently) support deregistration.
var ErrDeregistrationUnsupported = errors.New("deregistration unsupported")
