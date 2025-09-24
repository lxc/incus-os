package api

import (
	"log/slog"
)

// DebugTUI defines a struct to hold a message to log along with its severity.
type DebugTUI struct {
	Level   slog.Level `json:"level"   yaml:"level"`
	Message string     `json:"message" yaml:"message"`
}
