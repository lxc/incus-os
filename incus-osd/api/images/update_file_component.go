package images

import (
	"fmt"
)

// UpdateFileComponent represents the component affected by an update.
type UpdateFileComponent string

const (
	// UpdateFileComponentOS represents an OS update.
	UpdateFileComponentOS UpdateFileComponent = "os"

	// UpdateFileComponentIncus represents an Incus application update.
	UpdateFileComponentIncus UpdateFileComponent = "incus"

	// UpdateFileComponentDebug represents a debug application update.
	UpdateFileComponentDebug UpdateFileComponent = "debug"
)

var updateFileComponents = map[UpdateFileComponent]struct{}{
	UpdateFileComponentOS:    {},
	UpdateFileComponentIncus: {},
	UpdateFileComponentDebug: {},
}

func (u *UpdateFileComponent) String() string {
	return string(*u)
}

// MarshalText implements the encoding.TextMarshaler interface.
func (u *UpdateFileComponent) MarshalText() ([]byte, error) {
	return []byte(*u), nil
}

// UnmarshalText implements the encoding.TextUnmarshaler interface.
func (u *UpdateFileComponent) UnmarshalText(text []byte) error {
	_, ok := updateFileComponents[UpdateFileComponent(text)]
	if !ok {
		return fmt.Errorf("%q is not a valid update file component", string(text))
	}

	*u = UpdateFileComponent(text)

	return nil
}
