package api

// SystemLoggingSyslog contains the configuration options for a remote syslog server.
type SystemLoggingSyslog struct {
	Address   string `json:"address"    yaml:"address"`
	Protocol  string `json:"protocol"   yaml:"protocol"`
	LogFormat string `json:"log_format" yaml:"log_format"`
}

// SystemLoggingConfig holds the modifiable part of the logging data.
type SystemLoggingConfig struct {
	Syslog SystemLoggingSyslog `json:"syslog" yaml:"syslog"`
}

// SystemLoggingState represents state for the system's logging configuration.
type SystemLoggingState struct{}

// SystemLogging defines a struct to hold information about the system's logging configuration.
type SystemLogging struct {
	Config SystemLoggingConfig `json:"config" yaml:"config"`
	State  SystemLoggingState  `incusos:"-"   json:"state"  yaml:"state"`
}
