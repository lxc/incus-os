package api

// ServiceTailscale represents the state and configuration of the Tailscale service.
type ServiceTailscale struct {
	State struct{} `json:"state" yaml:"state"`

	Config struct {
		Enabled          bool     `json:"enabled"           yaml:"enabled"`
		LoginServer      string   `json:"login_server"      yaml:"login_server"`
		AuthKey          string   `json:"auth_key"          yaml:"auth_key"`
		AcceptRoutes     bool     `json:"accept_routes"     yaml:"accept_routes"`
		AdvertisedRoutes []string `json:"advertised_routes" yaml:"advertised_routes"`
		ServeEnabled     bool     `json:"serve_enabled"     yaml:"serve_enabled"`
		ServePort        int16    `json:"serve_port"        yaml:"serve_port"`
	} `json:"config" yaml:"config"`
}
