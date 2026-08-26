package api

// SystemLoggingSyslog contains the configuration options for a remote syslog server.
type SystemLoggingSyslog struct {
	Address   string `json:"address"    yaml:"address"`
	Protocol  string `json:"protocol"   yaml:"protocol"`
	LogFormat string `json:"log_format" yaml:"log_format"`
}

// SystemLoggingJournalUpload contains the configuration options for uploading the journal to a remote server.
type SystemLoggingJournalUpload struct {
	URL                  string `json:"url"                    yaml:"url"`
	TLSClientCertificate string `json:"tls_client_certificate" yaml:"tls_client_certificate"`
	TLSClientKey         string `json:"tls_client_key"         yaml:"tls_client_key"`
	TLSCACertificate     string `json:"tls_ca_certificate"     yaml:"tls_ca_certificate"`
}

// SystemLoggingConfig holds the modifiable part of the logging data.
type SystemLoggingConfig struct {
	JournalUpload SystemLoggingJournalUpload `json:"journal_upload" yaml:"journal_upload"`
	Syslog        SystemLoggingSyslog        `json:"syslog"         yaml:"syslog"`
}

// SystemLoggingState represents state for the system's logging configuration.
type SystemLoggingState struct{}

// SystemLogging defines a struct to hold information about the system's logging configuration.
//
// swagger:model
type SystemLogging struct {
	Config SystemLoggingConfig `json:"config" yaml:"config"`
	State  SystemLoggingState  `incusos:"-"   json:"state"  yaml:"state"`
}
