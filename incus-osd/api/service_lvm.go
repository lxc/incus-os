package api

// ServiceLVMConfig represents additional configuration for the LVM service.
type ServiceLVMConfig struct {
	Enabled  bool `json:"enabled"   yaml:"enabled"`
	SystemID int  `json:"system_id" yaml:"system_id"`
}

// ServiceLVM represents the state and configuration of the LVM service.
type ServiceLVM struct {
	State ServiceLVMState `incusos:"-" json:"state" yaml:"state"`

	Config ServiceLVMConfig `json:"config" yaml:"config"`
}

// ServiceLVMState represents the state for the LVM service.
type ServiceLVMState struct {
	PVs []ServiceLVMPV  `json:"pvs,omitempty" yaml:"pvs,omitempty"`
	VGs []ServiceLVMVG  `json:"vgs,omitempty" yaml:"vgs,omitempty"`
	Log []ServiceLVMLog `json:"log,omitempty" yaml:"log,omitempty"`
}

// ServiceLVMPV defines information about a given physical volume.
type ServiceLVMPV struct {
	PVName string `json:"pv_name" yaml:"pv_name"`
	VGName string `json:"vg_name" yaml:"vg_name"`
	PVFmt  string `json:"pv_fmt"  yaml:"pv_fmt"`
	PVAttr string `json:"pv_attr" yaml:"pv_attr"`
	PVSize string `json:"pv_size" yaml:"pv_size"`
	PVFree string `json:"pv_free" yaml:"pv_free"`
}

// ServiceLVMVG defines information about a given volume group.
type ServiceLVMVG struct {
	VGName    string `json:"vg_name"    yaml:"vg_name"`
	PVCount   int    `json:"pv_count"   yaml:"pv_count"`
	LVCount   int    `json:"lv_count"   yaml:"lv_count"`
	SnapCount int    `json:"snap_count" yaml:"snap_count"`
	VGAttr    string `json:"vg_attr"    yaml:"vg_attr"`
	VGSize    string `json:"vg_size"    yaml:"vg_size"`
	VGFree    string `json:"vg_free"    yaml:"vg_free"`
}

// ServiceLVMLog defines a LVM log entry.
type ServiceLVMLog struct {
	LogSeqNum        int    `json:"log_seq_num"         yaml:"log_seq_num"`
	LogType          string `json:"log_type"            yaml:"log_type"`
	LogContext       string `json:"log_context"         yaml:"log_context"`
	LogObjectType    string `json:"log_object_type"     yaml:"log_object_type"`
	LogObjectName    string `json:"log_object_name"     yaml:"log_object_name"`
	LogObjectID      string `json:"log_object_id"       yaml:"log_object_id"`
	LogObjectGroup   string `json:"log_object_group"    yaml:"log_object_group"`
	LogObjectGroupID string `json:"log_object_group_id" yaml:"log_object_group_id"`
	LogMessage       string `json:"log_message"         yaml:"log_message"`
	LogErrno         int    `json:"log_errno"           yaml:"log_errno"`
	LogRetCode       int    `json:"log_ret_code"        yaml:"log_ret_code"`
}
