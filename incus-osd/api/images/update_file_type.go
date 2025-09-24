package images

// UpdateFileType represents the type in an update file.
type UpdateFileType string

const (
	// UpdateFileTypeUndefined represents an unknown file type.
	UpdateFileTypeUndefined UpdateFileType = ""

	// UpdateFileTypeImageRaw represents a raw disk image.
	UpdateFileTypeImageRaw UpdateFileType = "image-raw"

	// UpdateFileTypeImageISO represents an ISO image.
	UpdateFileTypeImageISO UpdateFileType = "image-iso"

	// UpdateFileTypeImageManifest represents an image manifest.
	UpdateFileTypeImageManifest UpdateFileType = "image-manifest"

	// UpdateFileTypeUpdateEFI represents the EFI part of an OS update.
	UpdateFileTypeUpdateEFI UpdateFileType = "update-efi"

	// UpdateFileTypeUpdateUsr represents the /usr part of an OS update.
	UpdateFileTypeUpdateUsr UpdateFileType = "update-usr"

	// UpdateFileTypeUpdateUsrVerity represents the /usr verity tree part of an OS update.
	UpdateFileTypeUpdateUsrVerity UpdateFileType = "update-usr-verity"

	// UpdateFileTypeUpdateUsrVeritySignature represents the /usr verity signature part of an OS update.
	UpdateFileTypeUpdateUsrVeritySignature UpdateFileType = "update-usr-verity-signature"

	// UpdateFileTypeApplication represents an application.
	UpdateFileTypeApplication UpdateFileType = "application"
)

// UpdateFileTypes is a map of the supported update file types.
var UpdateFileTypes = map[UpdateFileType]struct{}{
	UpdateFileTypeUndefined:                {},
	UpdateFileTypeImageRaw:                 {},
	UpdateFileTypeImageISO:                 {},
	UpdateFileTypeImageManifest:            {},
	UpdateFileTypeUpdateEFI:                {},
	UpdateFileTypeUpdateUsr:                {},
	UpdateFileTypeUpdateUsrVerity:          {},
	UpdateFileTypeUpdateUsrVeritySignature: {},
	UpdateFileTypeApplication:              {},
}

func (u *UpdateFileType) String() string {
	return string(*u)
}

// MarshalText implements the encoding.TextMarshaler interface.
func (u *UpdateFileType) MarshalText() ([]byte, error) {
	return []byte(*u), nil
}

// UnmarshalText implements the encoding.TextUnmarshaler interface.
func (u *UpdateFileType) UnmarshalText(text []byte) error {
	*u = UpdateFileType(text)

	return nil
}
