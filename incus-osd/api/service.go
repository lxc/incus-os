package api

// ServiceAction defines a generic struct that can be used when triggering a service-specific action.
//
// swagger:model
type ServiceAction struct {
	Action string            `json:"action"           yaml:"action"`
	Config map[string]string `json:"config,omitempty" yaml:"config,omitempty"`
}
