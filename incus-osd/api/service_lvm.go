package api

// ServiceLVM represents the state and configuration of the LVM service.
type ServiceLVM struct {
	State struct{} `json:"state" yaml:"state"`

	Config struct {
		Enabled  bool  `json:"enabled"   yaml:"enabled"`
		SystemID int64 `json:"system_id" yaml:"system_id"`
	} `json:"config" yaml:"config"`
}
