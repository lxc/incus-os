package api

// LicenseType defines a custom type for the kind of license entry.
type LicenseType string

// Define constants used for the supported license types.
const (
	LicenseTypePackage LicenseType = "debian"
	LicenseTypeLocal   LicenseType = "local"
	LicenseTypeLicense LicenseType = "license"
)

// License holds the license information for a single component of the system.
//
// swagger:model
type License struct {
	Name    string      `json:"name"    yaml:"name"`    // Name of the component.
	Type    LicenseType `json:"type"    yaml:"type"`    // One of "debian" (Debian package), "local" (other software) or "license" (plain license text).
	Content string      `json:"content" yaml:"content"` // Copyright and license text.
}
