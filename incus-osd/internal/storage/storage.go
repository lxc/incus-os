package storage

// BlockDevices stores specific fields for each device reported by `lsblk`.
type BlockDevices struct {
	KName string `json:"kname"`
	ID    string `json:"id-link"` //nolint:tagliatelle
	Size  int    `json:"size"`
}

// LsblkOutput stores the output of running `lsblk -J ...`.
type LsblkOutput struct {
	Blockdevices []BlockDevices `json:"blockdevices"`
}
