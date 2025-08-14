package seed

// MigrationManager represents a Migration Manager seed file.
type MigrationManager struct {
	// PEM-encoded server TLS certificate. If not specified, a certificate will
	// be auto-generated when Migration Manager first starts up.
	ServerCertificate string `json:"server_certificate,omitempty" yaml:"server_certificate,omitempty"`
	ServerKey         string `json:"server_key,omitempty"         yaml:"server_key,omitempty"`

	// An array of SHA256 certificate fingerprints that belong to trusted TLS clients.
	TrustedTLSClientCertFingerprints []string `json:"trusted_tls_client_cert_fingerprints,omitempty" yaml:"trusted_tls_client_cert_fingerprints,omitempty"`

	// OIDC-specific configuration.
	OidcIssuer   string `json:"oidc.issuer,omitempty"    yaml:"oidc.issuer,omitempty"`    //nolint:tagliatelle
	OidcClientID string `json:"oidc.client.id,omitempty" yaml:"oidc.client.id,omitempty"` //nolint:tagliatelle
	OidcScope    string `json:"oidc.scopes,omitempty"    yaml:"oidc.scopes,omitempty"`    //nolint:tagliatelle
	OidcAudience string `json:"oidc.audience,omitempty"  yaml:"oidc.audience,omitempty"`  //nolint:tagliatelle
	OidcClaim    string `json:"oidc.claim,omitempty"     yaml:"oidc.claim,omitempty"`     //nolint:tagliatelle

	// OpenFGA-specific configuration.
	OpenfgaAPIToken string `json:"openfga.api.token,omitempty" yaml:"openfga.api.token,omitempty"` //nolint:tagliatelle
	OpenfgaAPIURL   string `json:"openfga.api.url,omitempty"   yaml:"openfga.api.url,omitempty"`   //nolint:tagliatelle
	OpenfgaStoreID  string `json:"openfga.store.id,omitempty"  yaml:"openfga.store.id,omitempty"`  //nolint:tagliatelle
}
