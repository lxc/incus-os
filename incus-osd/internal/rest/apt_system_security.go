package rest

import (
	"errors"
	"io"
	"net/http"

	"github.com/lxc/incus-os/incus-osd/internal/rest/response"
	"github.com/lxc/incus-os/incus-osd/internal/secureboot"
	"github.com/lxc/incus-os/incus-osd/internal/systemd"
)

func (s *Server) apiSystemSecurity(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	switch r.Method {
	case http.MethodGet:
		var err error

		// Mark that the keys have been retrieved via the API.
		s.state.System.Security.State.EncryptionRecoveryKeysRetrieved = true

		// Get Secure Boot state (we always expect this to be true).
		s.state.System.Security.State.SecureBootEnabled, err = secureboot.Enabled()
		if err != nil {
			_ = response.BadRequest(err).Render(w)

			return
		}

		// Get a list of Secure Boot certificates.
		s.state.System.Security.State.SecureBootCertificates = secureboot.ListCertificates()

		// Get TPM status.
		s.state.System.Security.State.TPMStatus = secureboot.TPMStatus()

		// Return the current system security state.
		_ = response.SyncResponse(true, s.state.System.Security).Render(w)
	case http.MethodPut, http.MethodDelete:
		// Add or remove an encryption key.
		if r.ContentLength <= 0 {
			_ = response.BadRequest(errors.New("no encryption key provided")).Render(w)

			return
		}

		b, err := io.ReadAll(r.Body)
		if err != nil {
			_ = response.BadRequest(err).Render(w)

			return
		}

		if r.Method == http.MethodPut {
			err = systemd.AddEncryptionKey(r.Context(), s.state, string(b))
		} else {
			err = systemd.DeleteEncryptionKey(r.Context(), s.state, string(b))
		}

		if err != nil {
			_ = response.BadRequest(err).Render(w)

			return
		}

		_ = response.EmptySyncResponse.Render(w)
	default:
		// If none of the supported methods, return NotImplemented.
		_ = response.NotImplemented(nil).Render(w)
	}

	_ = s.state.Save(r.Context())
}
