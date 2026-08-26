package rest

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"slices"
	"strings"

	"github.com/lxc/incus-os/incus-osd/api"
	"github.com/lxc/incus-os/incus-osd/internal/auth"
	"github.com/lxc/incus-os/incus-osd/internal/rest/response"
	"github.com/lxc/incus-os/incus-osd/internal/secureboot"
	"github.com/lxc/incus-os/incus-osd/internal/storage"
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
//	          $ref: "#/definitions/SystemSecurity"
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
//	      $ref: "#/definitions/SystemSecurity"
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
		s.state.System.Security.State.TPMStatus, err = secureboot.TPMStatus()
		if err != nil {
			_ = response.InternalError(err).Render(w)

			return
		}

		// Get drive encryption keys.
		s.state.System.Security.State.DriveRecoveryKeys, err = storage.GetDriveKeys()
		if err != nil {
			_ = response.InternalError(err).Render(w)

			return
		}

		// Get zpool encryption keys.
		s.state.System.Security.State.PoolRecoveryKeys, err = zfs.GetZpoolEncryptionKeys()
		if err != nil {
			_ = response.InternalError(err).Render(w)

			return
		}

		s.state.System.Security.State.SystemStateIsTrusted = !secureboot.IsTrustedFuseBlown()

		// If the system state is untrusted, report details about why.
		if !s.state.System.Security.State.SystemStateIsTrusted {
			issues := []string{}

			if s.state.UsingSWTPM {
				issues = append(issues, "using swtpm")
			}

			if s.state.SecureBootDisabled {
				issues = append(issues, "Secure Boot is disabled")
			}

			if s.state.FullAgentEnabled {
				issues = append(issues, "incus-agent is/was fully enabled")
			}

			s.state.System.Security.State.SystemStateStatus = strings.Join(issues, ", ")

			if s.state.System.Security.State.SystemStateStatus == "" {
				s.state.System.Security.State.SystemStateStatus = "no current issues, but system was previously in an untrusted state"
			}
		} else {
			s.state.System.Security.State.SystemStateStatus = "system state is fully trusted"
		}

		// Get TPM public key, if it exists.
		contents, err := os.ReadFile(auth.PEMPath)
		if err == nil {
			s.state.System.Security.State.TPMPublicKey = string(contents)
		}

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
				err := systemd.AddEncryptionKey(r.Context(), s.state, newKey, false)
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

		slog.InfoContext(r.Context(), "Custom CA certificates updated, but may not fully take effect until the system is rebooted")

		_ = response.EmptySyncResponse.Render(w)
	default:
		// If none of the supported methods, return NotImplemented.
		_ = response.NotImplemented(nil).Render(w)
	}

	_ = s.state.Save()
}

// swagger:operation POST /1.0/system/security/:retrieved system system_post_security_retrieved
//
//	Mark encryption recovery keys as retrieved
//
//	Marks the encryption recovery keys as having been retrieved, clearing the warning shown on the console.
//
//	---
//	produces:
//	  - application/json
//	responses:
//	  "200":
//	    $ref: "#/responses/EmptySyncResponse"
//	  "500":
//	    $ref: "#/responses/InternalServerError"
func (s *Server) apiSystemSecurityRetrieved(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodPost {
		_ = response.NotImplemented(nil).Render(w)

		return
	}

	s.state.System.Security.State.EncryptionRecoveryKeysRetrieved = true

	err := s.state.Save()
	if err != nil {
		_ = response.InternalError(err).Render(w)

		return
	}

	_ = response.EmptySyncResponse.Render(w)
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

	err := secureboot.ForceUpdatePCRBindings(r.Context(), s.state.OS.Name, s.state.OS.RunningRelease)
	if err != nil {
		_ = response.InternalError(err).Render(w)

		return
	}

	_ = response.EmptySyncResponse.Render(w)
	_ = s.state.Save()
}
