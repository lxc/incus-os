package api

// ServiceISCSITarget represents a single ISCSI target.
type ServiceISCSITarget struct {
	Target  string `json:"target"  yaml:"target"`
	Address string `json:"address" yaml:"address"`
	Port    int    `json:"port"    yaml:"port"`
}

// ServiceISCSI represents the state and configuration of the ISCSI service.
type ServiceISCSI struct {
	State struct {
		InitiatorName string `json:"initiator_name" yaml:"initiator_name"`
	} `json:"state" yaml:"state"`

	Config struct {
		Enabled bool                 `json:"enabled" yaml:"enabled"`
		Targets []ServiceISCSITarget `json:"targets" yaml:"targets"`
	} `json:"config" yaml:"config"`
}
