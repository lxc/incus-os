package api

// ServiceTailscale represents the state and configuration of the Tailscale service.
type ServiceTailscale struct {
	Config ServiceTailscaleConfig `json:"config" yaml:"config"`
	State  struct{}               `json:"state"  yaml:"state"`
}

// ServiceTailscaleConfig represents additional configuration for the Tailscale service.
type ServiceTailscaleConfig struct {
	Enabled          bool     `json:"enabled"           yaml:"enabled"`
	LoginServer      string   `json:"login_server"      yaml:"login_server"`
	AuthKey          string   `json:"auth_key"          yaml:"auth_key"`
	AcceptRoutes     bool     `json:"accept_routes"     yaml:"accept_routes"`
	AdvertisedRoutes []string `json:"advertised_routes" yaml:"advertised_routes"`
	ServeEnabled     bool     `json:"serve_enabled"     yaml:"serve_enabled"`
	ServePort        int      `json:"serve_port"        yaml:"serve_port"`
	ServeService     string   `json:"serve_service"     yaml:"serve_service"`
}
