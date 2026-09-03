package systemd

import (
	"context"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strconv"

	"github.com/lxc/incus-os/incus-osd/api"
)

const (
	journalUploadConfigPath = "/etc/systemd/journal-upload.conf"
	journalUploadCertsPath  = "/etc/systemd/journal-upload"
	journalUploadUser       = "systemd-journal-upload"
)

// SetJournalUpload sets the system's remote journal upload configuration.
func SetJournalUpload(ctx context.Context, config api.SystemLoggingJournalUpload) error {
	// Handle disabling journal upload.
	if config.URL == "" {
		err := os.Remove(journalUploadConfigPath)
		if err != nil && !os.IsNotExist(err) {
			return err
		}

		err = os.RemoveAll(journalUploadCertsPath)
		if err != nil {
			return err
		}

		return StopUnit(ctx, "systemd-journal-upload")
	}

	// The upload service runs unprivileged, so certificate files must be readable by its user.
	u, err := user.Lookup(journalUploadUser)
	if err != nil {
		return err
	}

	uid, err := strconv.Atoi(u.Uid)
	if err != nil {
		return err
	}

	gid, err := strconv.Atoi(u.Gid)
	if err != nil {
		return err
	}

	err = os.MkdirAll(journalUploadCertsPath, 0o755)
	if err != nil {
		return err
	}

	// Write the certificate files, returning the path to use in the configuration.
	writeCertificate := func(name string, contents string, fallback string) (string, error) {
		path := filepath.Join(journalUploadCertsPath, name)

		if contents == "" {
			err := os.Remove(path)
			if err != nil && !os.IsNotExist(err) {
				return "", err
			}

			return fallback, nil
		}

		err := os.WriteFile(path, []byte(contents), 0o600)
		if err != nil {
			return "", err
		}

		err = os.Chown(path, uid, gid)
		if err != nil {
			return "", err
		}

		return path, nil
	}

	// Without a CA certificate, rely on the system CA bundle.
	trustedCertificate, err := writeCertificate("ca.pem", config.TLSCACertificate, "/etc/ssl/certs/ca-certificates.crt")
	if err != nil {
		return err
	}

	// Without a client certificate and key, disable client certificate authentication.
	certificate, err := writeCertificate("cert.pem", config.TLSClientCertificate, "-")
	if err != nil {
		return err
	}

	key, err := writeCertificate("key.pem", config.TLSClientKey, "-")
	if err != nil {
		return err
	}

	// Write the configuration.
	w, err := os.Create(journalUploadConfigPath)
	if err != nil {
		return err
	}

	defer func() { _ = w.Close() }()

	_, err = fmt.Fprintf(w, `[Upload]
URL=%s
ServerKeyFile=%s
ServerCertificateFile=%s
TrustedCertificateFile=%s
`, config.URL, key, certificate, trustedCertificate)
	if err != nil {
		return err
	}

	// Start the daemon.
	return RestartUnit(ctx, "systemd-journal-upload")
}
