package api

// ServiceNVMETarget represents a single NVME target.
type ServiceNVMETarget struct {
	Transport string `json:"transport" yaml:"transport"`
	Address   string `json:"address"   yaml:"address"`
	Port      int    `json:"port"      yaml:"port"`
}

// ServiceNVME represents the state and configuration of the NVME service.
type ServiceNVME struct {
	State ServiceNVMEState `json:"state" yaml:"state"`

	Config struct {
		Enabled bool                `json:"enabled" yaml:"enabled"`
		Targets []ServiceNVMETarget `json:"targets" yaml:"targets"`
	} `json:"config" yaml:"config"`
}

// ServiceNVMEState represents the state for the NVME service.
type ServiceNVMEState struct {
	HostID  string `json:"host_id"  yaml:"host_id"`
	HostNQN string `json:"host_nqn" yaml:"host_nqn"`
}
