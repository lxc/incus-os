package api

// ServiceNVMETarget represents a single NVME target.
type ServiceNVMETarget struct {
	Transport   string `json:"transport"              yaml:"transport"`
	Address     string `json:"address"                yaml:"address"`
	HostAddress string `json:"host_address,omitempty" yaml:"host_address,omitempty"`
	Port        int    `json:"port"                   yaml:"port"`
	NQN         string `json:"nqn,omitempty"          yaml:"nqn,omitempty"`
}

// ServiceNVMEController represents a single NVME controller.
type ServiceNVMEController struct {
	Name       string   `json:"name"       yaml:"name"`
	Transport  string   `json:"transport"  yaml:"transport"`
	Address    string   `json:"address"    yaml:"address"`
	State      string   `json:"state"      yaml:"state"`
	Namespaces []string `json:"namespaces" yaml:"namespaces"`
}

// ServiceNVMESubsystem represents a single NVME subsystem.
type ServiceNVMESubsystem struct {
	Name        string                  `json:"name"        yaml:"name"`
	NQN         string                  `json:"nqn"         yaml:"nqn"`
	Controllers []ServiceNVMEController `json:"controllers" yaml:"controllers"`
}

// ServiceNVMEConfig represents additional configuration for the NVME service.
type ServiceNVMEConfig struct {
	Enabled bool                `json:"enabled" yaml:"enabled"`
	Targets []ServiceNVMETarget `json:"targets" yaml:"targets"`
}

// ServiceNVME represents the state and configuration of the NVME service.
type ServiceNVME struct {
	State ServiceNVMEState `incusos:"-" json:"state" yaml:"state"`

	Config ServiceNVMEConfig `json:"config" yaml:"config"`
}

// ServiceNVMEState represents the state for the NVME service.
type ServiceNVMEState struct {
	HostID     string                 `json:"host_id"    yaml:"host_id"`
	HostNQN    string                 `json:"host_nqn"   yaml:"host_nqn"`
	Subsystems []ServiceNVMESubsystem `json:"subsystems" yaml:"subsystems"`
}
