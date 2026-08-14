package api

// InternalSecureBootCertificates holds the PEM encoded Secure Boot certificates used by the system.
type InternalSecureBootCertificates struct {
	PK  string   `json:"pk"  yaml:"pk"`
	KEK []string `json:"kek" yaml:"kek"`
	DB  []string `json:"db"  yaml:"db"`
	DBX []string `json:"dbx" yaml:"dbx"`
}
