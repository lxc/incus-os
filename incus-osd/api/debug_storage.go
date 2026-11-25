package api

// DebugStorageTrace defines the arguments for a block device trace request.
//
// swagger:model
type DebugStorageTrace struct {
	// Block device to trace, either a device name or a path under /dev.
	Device string `json:"device" yaml:"device"`

	// I/O categories to trace using blktrace names such as "discard" or "write", defaults to all of them.
	Actions []string `json:"actions,omitempty" yaml:"actions,omitempty"`

	// Seconds to trace for, defaults to 10 if unset, 0 traces until the request is canceled.
	Duration int `json:"duration" yaml:"duration"`
}
