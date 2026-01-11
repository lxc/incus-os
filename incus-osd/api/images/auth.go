package images

// AuthenticationRegister is used for the optional registration of a client with an image server.
// This will be sent to a /register endpoint on the image server and is
// typically a required step for image servers enforcing authentication.
type AuthenticationRegister struct {
	MachineID string `json:"machine_id"`
	Token     string `json:"token"`
	PublicKey string `json:"public_key"`
	Initial   string `json:"initial"`
}

// AuthenticationToken contains the fields that are part of the optional authentication token.
// This typically gets JSON encoded, then encrypted with the remote key
// before finally being gzip compressed and base64 encoded to be sent as an
// HTTP header.
type AuthenticationToken struct {
	MachineID string `json:"machine_id"`
	Timestamp int64  `json:"timestamp"`
}
