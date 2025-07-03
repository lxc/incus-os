package images

import (
	"fmt"
)

// UpdateFileArchitecture represents the architecture for a given file.
type UpdateFileArchitecture string

const (
	// UpdateFileArchitectureUndefined represents an unknown architecture.
	UpdateFileArchitectureUndefined UpdateFileArchitecture = ""

	// UpdateFileArchitecture64BitX86 represents an x86_64 system.
	UpdateFileArchitecture64BitX86 UpdateFileArchitecture = "x86_64"

	// UpdateFileArchitecture64BitARM represents an aarch64 system.
	UpdateFileArchitecture64BitARM UpdateFileArchitecture = "aarch64"
)

var architecture = map[UpdateFileArchitecture]struct{}{
	UpdateFileArchitectureUndefined: {},
	UpdateFileArchitecture64BitX86:  {},
	UpdateFileArchitecture64BitARM:  {},
}

func (u *UpdateFileArchitecture) String() string {
	return string(*u)
}

// MarshalText implements the encoding.TextMarshaler interface.
func (u *UpdateFileArchitecture) MarshalText() ([]byte, error) {
	return []byte(*u), nil
}

// UnmarshalText implements the encoding.TextUnmarshaler interface.
func (u *UpdateFileArchitecture) UnmarshalText(text []byte) error {
	_, ok := architecture[UpdateFileArchitecture(text)]
	if !ok {
		return fmt.Errorf("%q is not a valid update file type", string(text))
	}

	*u = UpdateFileArchitecture(text)

	return nil
}
