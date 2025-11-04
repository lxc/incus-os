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
	"github.com/lxc/incus-os/incus-osd/internal/zfs"
)

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

		// Return the current system security state.
		_ = response.SyncResponse(true, s.state.System.Security).Render(w)
	case http.MethodPut:
		// Update the list of encryption recovery keys.
		if r.ContentLength <= 0 {
			_ = response.BadRequest(errors.New("no security configuration provided")).Render(w)

			return
		}

		securityStruct := &api.SystemSecurity{}

		err := json.NewDecoder(r.Body).Decode(securityStruct)
		if err != nil {
			_ = response.BadRequest(err).Render(w)

			return
		}

		if len(securityStruct.Config.EncryptionRecoveryKeys) == 0 {
			_ = response.BadRequest(errors.New("no encryption key provided")).Render(w)

			return
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

		_ = response.EmptySyncResponse.Render(w)
	default:
		// If none of the supported methods, return NotImplemented.
		_ = response.NotImplemented(nil).Render(w)
	}

	_ = s.state.Save()
}

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
