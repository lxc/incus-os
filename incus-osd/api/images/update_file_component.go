package images

// UpdateFileComponent represents the component affected by an update.
type UpdateFileComponent string

const (
	// UpdateFileComponentOS represents an OS update.
	UpdateFileComponentOS UpdateFileComponent = "os"

	// UpdateFileComponentDebug represents a debug application update.
	UpdateFileComponentDebug UpdateFileComponent = "debug"

	// UpdateFileComponentIncus represents an Incus application update.
	UpdateFileComponentIncus UpdateFileComponent = "incus"

	// UpdateFileComponentIncusCeph represents a Ceph application update for Incus.
	UpdateFileComponentIncusCeph UpdateFileComponent = "incus-ceph"

	// UpdateFileComponentIncusLinstor represents a Linstor application update for Incus.
	UpdateFileComponentIncusLinstor UpdateFileComponent = "incus-linstor"

	// UpdateFileComponentMigrationManager represents a Migration Manager application update.
	UpdateFileComponentMigrationManager UpdateFileComponent = "migration-manager"

	// UpdateFileComponentOperationsCenter represents an Operations Center application update.
	UpdateFileComponentOperationsCenter UpdateFileComponent = "operations-center"
)

// UpdateFileComponents is a map of the supported update file components.
var UpdateFileComponents = map[UpdateFileComponent]struct{}{
	UpdateFileComponentOS:               {},
	UpdateFileComponentDebug:            {},
	UpdateFileComponentIncus:            {},
	UpdateFileComponentIncusCeph:        {},
	UpdateFileComponentIncusLinstor:     {},
	UpdateFileComponentMigrationManager: {},
	UpdateFileComponentOperationsCenter: {},
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
	*u = UpdateFileComponent(text)

	return nil
}
