package rest

import (
	"encoding/json"
	"errors"
	"net/http"
	"slices"

	"github.com/lxc/incus-os/incus-osd/api"
	"github.com/lxc/incus-os/incus-osd/internal/rest/response"
	"github.com/lxc/incus-os/incus-osd/internal/secureboot"
	"github.com/lxc/incus-os/incus-osd/internal/systemd"
	"github.com/lxc/incus-os/incus-osd/internal/util"
	"github.com/lxc/incus-os/incus-osd/internal/zfs"
)

// swagger:operation GET /1.0/system/security system system_get_security
//
//	Get security information
//
//	Returns information about the system's security state, such as Secure Boot and TPM status, encryption recovery keys, etc.
//
//	---
//	produces:
//	  - application/json
//	responses:
//	  "200":
//	    description: State and configuration for the system security
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
//	          description: State and configuration for the system security
//	          example: {"config":{"encryption_recovery_keys":["fkrjjenn-tbtjbjgh-jtvvchjr-ctienevu-crknfkvi-vjlvblhl-kbneribu-htjtldch"]},"state":{"encryption_recovery_keys_retrieved":true,"encrypted_volumes":[{"volume":"root","state":"unlocked (TPM)"},{"volume":"swap","state":"unlocked (TPM)"}],"secure_boot_enabled":true,"secure_boot_certificates":[{"type":"PK","fingerprint":"26dce4dbb3de2d72bd16ae91a85cfeda84535317d3ee77e0d4b2d65e714cf111","subject":"CN=Incus OS - Secure Boot PK R1,O=Linux Containers","issuer":"CN=Incus OS - Secure Boot E1,O=Linux Containers"},{"type":"KEK","fingerprint":"9a42866f496834bde7e1b26a862b1e1b6dea7b78b91a948aecfc4e6ef79ea6c1","subject":"CN=Incus OS - Secure Boot KEK R1,O=Linux Containers","issuer":"CN=Incus OS - Secure Boot E1,O=Linux Containers"},{"type":"db","fingerprint":"21b6f423cf80fe6c436dfea0683460312f276debe2a14285bfdc22da2d00fc20","subject":"CN=Incus OS - Secure Boot 2025 R1,O=Linux Containers","issuer":"CN=Incus OS - Secure Boot E1,O=Linux Containers"},{"type":"db","fingerprint":"2243c49fcf6f84fe670f100ecafa801389dc207536cb9ca87aa2c062ddebfde5","subject":"CN=Incus OS - Secure Boot 2026 R1,O=Linux Containers","issuer":"CN=Incus OS - Secure Boot E1,O=Linux Containers"}],"tpm_status":"ok","pool_recovery_keys":{"local":"F7zrtdHEaivKqofZbVFs2EeANyK77DbLi6Z8sqYVhr0="},"system_state_is_trusted":true}}
//	  "500":
//	    $ref: "#/responses/InternalServerError"

// swagger:operation PUT /1.0/system/security system system_put_security
//
//	Update system security configuration
//
//	Updates the list of encryption recovery keys. At least one recovery key must always be
//	specified. Keys must be at least 15 characters long, contain at least one special
//	character, and consist of at least five unique characters. Some other simple complexity
//	checks are applied, and any key that doesn't pass will be rejected with an error.
//
//	Optionally, specify one or more PEM-encoded custom CA certificates that should be added
//	to the system's root trust. Only certificates specified in the API call will be persisted.
//
//	---
//	consumes:
//	  - application/json
//	produces:
//	  - application/json
//	parameters:
//	  - in: body
//	    name: configuration
//	    description: Security configuration
//	    required: true
//	    schema:
//	      type: object
//	      properties:
//	        config:
//	          type: object
//	          description: The security configuration
//	          example: {"encryption_recovery_keys":["my-super-secret-passphrase"],"custom_ca_certs":["-----BEGIN CERTIFICATE-----\n[cert]\n-----END CERTIFICATE-----"]}
//	responses:
//	  "200":
//	    $ref: "#/responses/EmptySyncResponse"
//	  "400":
//	    $ref: "#/responses/BadRequest"
//	  "500":
//	    $ref: "#/responses/InternalServerError"
func (s *Server) apiSystemSecurity(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	switch r.Method {
	case http.MethodGet:
		var err error

		// Mark that the keys have been retrieved via the API.
		s.state.System.Security.State.EncryptionRecoveryKeysRetrieved = true

		// s.state.System.Security.State.EncryptedVolumes is pre-cached, because
		// getting the state of the LUKS volumes can be slow.

		// Get Secure Boot state (we always expect this to be true).
		s.state.System.Security.State.SecureBootEnabled, err = secureboot.Enabled()
		if err != nil {
			_ = response.InternalError(err).Render(w)

			return
		}

		// Get a list of Secure Boot certificates.
		s.state.System.Security.State.SecureBootCertificates = secureboot.ListCertificates()

		// Get TPM status.
		s.state.System.Security.State.TPMStatus = secureboot.TPMStatus()

		// Get zpool encryption keys.
		s.state.System.Security.State.PoolRecoveryKeys, err = zfs.GetZpoolEncryptionKeys()
		if err != nil {
			_ = response.InternalError(err).Render(w)

			return
		}

		s.state.System.Security.State.SystemStateIsTrusted = !secureboot.IsTrustedFuseBlown()

		// Return the current system security state.
		_ = response.SyncResponse(true, s.state.System.Security).Render(w)
	case http.MethodPut:
		// Update the list of encryption recovery keys.
		securityStruct := &api.SystemSecurity{}

		counter := &countWrapper{ReadCloser: r.Body}

		err := json.NewDecoder(counter).Decode(securityStruct)
		if err != nil && counter.n > 0 {
			_ = response.BadRequest(err).Render(w)

			return
		}

		if len(securityStruct.Config.EncryptionRecoveryKeys) == 0 {
			_ = response.BadRequest(errors.New("no encryption key provided")).Render(w)

			return
		}

		// Add any new encryption keys.
		for _, newKey := range securityStruct.Config.EncryptionRecoveryKeys {
			if !slices.Contains(s.state.System.Security.Config.EncryptionRecoveryKeys, newKey) {
				err := systemd.AddEncryptionKey(r.Context(), s.state, newKey)
				if err != nil {
					_ = response.InternalError(err).Render(w)

					return
				}
			}
		}

		// Remove any encryption keys no longer present.
		for _, existingKey := range s.state.System.Security.Config.EncryptionRecoveryKeys {
			if !slices.Contains(securityStruct.Config.EncryptionRecoveryKeys, existingKey) {
				err := systemd.DeleteEncryptionKey(r.Context(), s.state, existingKey)
				if err != nil {
					_ = response.InternalError(err).Render(w)

					return
				}
			}
		}

		// Configure custom CA certificates, if any.
		s.state.System.Security.Config.CustomCACerts = securityStruct.Config.CustomCACerts

		err = util.UpdateSystemCustomCACerts(s.state.System.Security.Config.CustomCACerts)
		if err != nil {
			_ = response.InternalError(err).Render(w)

			return
		}

		_ = response.EmptySyncResponse.Render(w)
	default:
		// If none of the supported methods, return NotImplemented.
		_ = response.NotImplemented(nil).Render(w)
	}

	_ = s.state.Save()
}

// swagger:operation POST /1.0/system/security/:tpm-rebind system system_post_security_tpm_rebind
//
//	Reset TPM bindings
//
//	Forcibly resets TPM encryption bindings; intended only for use if it was required to enter a recovery passphrase to boot the system.
//
//	---
//	produces:
//	  - application/json
//	responses:
//	  "200":
//	    $ref: "#/responses/EmptySyncResponse"
//	  "500":
//	    $ref: "#/responses/InternalServerError"
func (s *Server) apiSystemSecurityTPMRebind(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodPost {
		_ = response.NotImplemented(nil).Render(w)

		return
	}

	err := secureboot.ForceUpdatePCRBindings(r.Context(), s.state.OS.Name, s.state.OS.RunningRelease, s.state.System.Security.Config.EncryptionRecoveryKeys[0])
	if err != nil {
		_ = response.InternalError(err).Render(w)

		return
	}

	_ = response.EmptySyncResponse.Render(w)
	_ = s.state.Save()
}
