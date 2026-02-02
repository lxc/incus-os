package customizer

// CertificateGet represents the data returned on GET /1.0/certifcate.
type CertificateGet struct {
	Certificate string `json:"certificate"`
	Key         string `json:"key"`
	PFX         string `json:"pfx"`
}

// OIDCGet represents the data returned on GET /1.0/oidc?username=NAME.
type OIDCGet struct {
	Issuer   string `json:"issuer"`
	ClientID string `json:"client_id"`
}
