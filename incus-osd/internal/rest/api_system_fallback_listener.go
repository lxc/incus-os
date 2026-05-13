package rest

import (
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"net/http"
	"net/netip"

	"github.com/lxc/incus-os/incus-osd/api"
	"github.com/lxc/incus-os/incus-osd/internal/rest/response"
)

// swagger:operation GET /1.0/system/fallback-listener system system_get_fallback_listener
//
//	Get information about the fallback HTTPS listener
//
//	Returns information about the system's fallback HTTPS listener, which is activated if
//	the primary application fails to start.
//
//	---
//	produces:
//	  - application/json
//	responses:
//	  "200":
//	    description: State and configuration for the fallback listener
//	    schema:
//	      type: object
//	      description: Sync response
//	      properties:
//	        type:
//	          description: Response type
//	          example: sync
//	          type: string
//	        status:
//	          type: string
//	          description: Status description
//	          example: Success
//	        status_code:
//	          type: integer
//	          description: Status code
//	          example: 200
//	        metadata:
//	          type: json
//	          description: State and configuration for the fallback listener
//	          example: {"config":{"listen_address":"192.168.100.100:8443","trusted_client_certificates":["-----BEGIN CERTIFICATE-----\n[cert]\n-----END CERTIFICATE-----"]},"state":{"active":false}}
//	  "500":
//	    $ref: "#/responses/InternalServerError"

// swagger:operation PUT /1.0/system/fallback-listener system system_put_fallback_listener
//
//	Update the fallback HTTPS listener configuration
//
//	Updates the configuration for the fallback HTTPS listener. Supported options include
//	listening on a specific IP:port and setting the list of trusted client TLS certificates,
//	which should be provided as PEM-encoded certificates.
//
//	---
//	consumes:
//	  - application/json
//	produces:
//	  - application/json
//	parameters:
//	  - in: body
//	    name: configuration
//	    description: Fallback listener configuration
//	    required: true
//	    schema:
//	      type: object
//	      properties:
//	        config:
//	          type: object
//	          description: The fallback listener configuration
//	          example: {"listen_address":"192.168.100.100:8443","trusted_client_certificates":["-----BEGIN CERTIFICATE-----\n[cert]\n-----END CERTIFICATE-----"]}
//	responses:
//	  "200":
//	    $ref: "#/responses/EmptySyncResponse"
//	  "400":
//	    $ref: "#/responses/BadRequest"
//	  "500":
//	    $ref: "#/responses/InternalServerError"
func (s *Server) apiSystemFallbackListener(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	switch r.Method {
	case http.MethodGet:
		// Return the current system fallback listener state.
		_ = response.SyncResponse(true, s.state.System.FallbackListener).Render(w)
	case http.MethodPut:
		// Update the fallback listener configuration.
		fallbackListenerStruct := &api.SystemFallbackListener{}

		counter := &countWrapper{ReadCloser: r.Body}

		err := json.NewDecoder(counter).Decode(fallbackListenerStruct)
		if err != nil && counter.n > 0 {
			_ = response.BadRequest(err).Render(w)

			return
		}

		// Validate the listen address, if specified.
		if fallbackListenerStruct.Config.ListenAddress != "" {
			_, err := netip.ParseAddrPort(fallbackListenerStruct.Config.ListenAddress)
			if err != nil {
				_ = response.BadRequest(errors.New("invalid listen address: " + err.Error())).Render(w)

				return
			}
		}

		if len(fallbackListenerStruct.Config.TrustedClientCertificates) == 0 {
			_ = response.BadRequest(errors.New("at least one trusted client TLS certificate must be defined")).Render(w)

			return
		}

		// Verify that we can parse each trusted certificate.
		for i, pemCert := range fallbackListenerStruct.Config.TrustedClientCertificates {
			block, _ := pem.Decode([]byte(pemCert))
			if block == nil || block.Type != "CERTIFICATE" {
				_ = response.BadRequest(fmt.Errorf("certificate at index %d is not PEM-encoded", i)).Render(w)

				return
			}

			_, err := x509.ParseCertificate(block.Bytes)
			if err != nil {
				_ = response.BadRequest(fmt.Errorf("certificate at index %d is not valid: %s", i, err.Error())).Render(w)

				return
			}
		}

		s.state.System.FallbackListener = *fallbackListenerStruct

		_ = response.EmptySyncResponse.Render(w)
	default:
		// If none of the supported methods, return NotImplemented.
		_ = response.NotImplemented(nil).Render(w)
	}

	_ = s.state.Save()
}
