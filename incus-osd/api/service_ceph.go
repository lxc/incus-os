package api

// ServiceCephCluster represents a single Ceph cluster.
type ServiceCephCluster struct {
	FSID         string                        `json:"fsid"          yaml:"fsid"`
	Monitors     []string                      `json:"monitors"      yaml:"monitors"`
	Keyrings     map[string]ServiceCephKeyring `json:"keyrings"      yaml:"keyrings"`
	ClientConfig map[string]string             `json:"client_config" yaml:"client_config"`
}

// ServiceCephKeyring represents a single Ceph keyring entry.
type ServiceCephKeyring struct {
	Key string `json:"key" yaml:"key"`
}

// ServiceCephConfig represents additional configuration for the Ceph service.
type ServiceCephConfig struct {
	Enabled  bool                          `json:"enabled"  yaml:"enabled"`
	Clusters map[string]ServiceCephCluster `json:"clusters" yaml:"clusters"`
}

// ServiceCephState represents state for the Ceph service.
type ServiceCephState struct{}

// ServiceCeph represents the state and configuration of the Ceph service.
type ServiceCeph struct {
	State ServiceCephState `incusos:"-" json:"state" yaml:"state"`

	Config ServiceCephConfig `json:"config" yaml:"config"`
}
