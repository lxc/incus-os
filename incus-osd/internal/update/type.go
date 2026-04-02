package update

import (
	"strconv"
)

// Type represents a specific update type.
type Type int

// Supported update types.
const (
	TypeSecureBoot Type = iota
	TypeOS
	TypeApplication
)

func (t Type) String() string {
	switch t {
	case TypeSecureBoot:
		return "SecureBoot"
	case TypeOS:
		return "OS"
	case TypeApplication:
		return "application"
	default:
		return "unknown update type " + strconv.Itoa(int(t))
	}
}
