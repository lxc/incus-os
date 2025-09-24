package images

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

// UpdateFileArchitectures is a map of the supported file architectures.
var UpdateFileArchitectures = map[UpdateFileArchitecture]struct{}{
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
	*u = UpdateFileArchitecture(text)

	return nil
}
