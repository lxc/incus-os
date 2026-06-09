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

// ServiceCephState represents state for the Ceph service. This largely matches most of the
// output of running `ceph status --format=json`.
type ServiceCephState struct {
	FSID   string `json:"fsid,omitempty" yaml:"fsid,omitempty"`
	Health *struct {
		Status string `json:"status" yaml:"status"`
		Checks map[string]struct {
			Severity string `json:"severity" yaml:"severity"`
			Summary  struct {
				Message string `json:"message" yaml:"message"`
				Count   int    `json:"count"   yaml:"count"`
			} `json:"summary" yaml:"summary"`
			Muted bool `json:"muted" yaml:"muted"`
		} `json:"checks,omitempty" yaml:"checks,omitempty"`
	} `json:"health,omitempty"         yaml:"health,omitempty"`
	ElectionEpoch int      `json:"election_epoch,omitempty" yaml:"election_epoch,omitempty"`
	Quorum        []int    `json:"quorum,omitempty"         yaml:"quorum,omitempty"`
	QuorumNames   []string `json:"quorum_names,omitempty"   yaml:"quorum_names,omitempty"`
	QuorumAge     int      `json:"quorum_age,omitempty"     yaml:"quorum_age,omitempty"`
	Monmap        *struct {
		Epoch             int    `json:"epoch"                yaml:"epoch"`
		MinMonReleaseName string `json:"min_mon_release_name" yaml:"min_mon_release_name"`
		NumMons           int    `json:"num_mons"             yaml:"num_mons"`
	} `json:"monmap,omitempty" yaml:"monmap,omitempty"`
	Osdmap *struct {
		Epoch          int `json:"epoch"            yaml:"epoch"`
		NumOsds        int `json:"num_osds"         yaml:"num_osds"`
		NumUpOsds      int `json:"num_up_osds"      yaml:"num_up_osds"`
		OsdUpSince     int `json:"osd_up_since"     yaml:"osd_up_since"`
		NumInOsds      int `json:"num_in_osds"      yaml:"num_in_osds"`
		OsdInSince     int `json:"osd_in_since"     yaml:"osd_in_since"`
		NumRemappedPgs int `json:"num_remapped_pgs" yaml:"num_remapped_pgs"`
	} `json:"osdmap,omitempty" yaml:"osdmap,omitempty"`
	Pgmap *struct {
		PgsByState []struct {
			StateName string `json:"state_name" yaml:"state_name"`
			Count     int    `json:"count"      yaml:"count"`
		} `json:"pgs_by_state" yaml:"pgs_by_state"`
		NumPgs      int `json:"num_pgs"      yaml:"num_pgs"`
		NumPools    int `json:"num_pools"    yaml:"num_pools"`
		NumProjects int `json:"num_objects"  yaml:"num_objects"`
		DataBytes   int `json:"data_bytes"   yaml:"data_bytes"`
		BytesUsed   int `json:"bytes_used"   yaml:"bytes_used"`
		BytesAvail  int `json:"bytes_avail"  yaml:"bytes_avail"`
		BytesTotal  int `json:"bytes_total"  yaml:"bytes_total"`
	} `json:"pgmap,omitempty" yaml:"pgmap,omitempty"`
	Fsmap *struct {
		Epoch int    `json:"epoch" yaml:"epoch"`
		Btime string `json:"btime" yaml:"btime"`
	} `json:"fsmap,omitempty" yaml:"fsmap,omitempty"`
	Mgrmap *struct {
		Available   bool     `json:"available"    yaml:"available"`
		NumStandbys int      `json:"num_standbys" yaml:"num_standbys"`
		Modules     []string `json:"modules"      yaml:"modules"`
	} `json:"mgrmap,omitempty" yaml:"mgrmap,omitempty"`
	Servicemap *struct {
		Epoch    int    `json:"epoch"    yaml:"epoch"`
		Modified string `json:"modified" yaml:"modified"`

		// The returned daemons are mostly proper structs, but there is a "status" field
		// that's a regular string. This prevents a clean json umarshaling, so use
		// `any` to handle this bit of the struct
		Services map[string]map[string]map[string]any `json:"services" yaml:"services"`
	} `json:"servicemap,omitempty" yaml:"servicemap,omitempty"`
}

// ServiceCeph represents the state and configuration of the Ceph service.
type ServiceCeph struct {
	State ServiceCephState `incusos:"-" json:"state" yaml:"state"`

	Config ServiceCephConfig `json:"config" yaml:"config"`
}
