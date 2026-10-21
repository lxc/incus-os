package api

// DebugLVM represents LVM debug information for the system.
//
// swagger:model
type DebugLVM struct {
	PVs []map[string]any `json:"pvs" yaml:"pvs"`
	VGs []map[string]any `json:"vgs" yaml:"vgs"`
	LVs []map[string]any `json:"lvs" yaml:"lvs"`
}
