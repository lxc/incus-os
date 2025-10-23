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

// ServiceCeph represents the state and configuration of the Ceph service.
type ServiceCeph struct {
	State struct{} `incusos:"-" json:"state" yaml:"state"`

	Config struct {
		Enabled  bool                          `json:"enabled"  yaml:"enabled"`
		Clusters map[string]ServiceCephCluster `json:"clusters" yaml:"clusters"`
	} `json:"config" yaml:"config"`
}
