package keyring

import (
	"bufio"
	"context"
	"encoding/hex"
	"os"
	"strings"

	"github.com/lxc/incus-os/incus-osd/internal/secureboot"
	"github.com/lxc/incus-os/incus-osd/internal/util"
)

// PlatformKeyring is the SecureBoot platform keyring.
var PlatformKeyring = "1f010000"

// Key represents a key in the Linux kernel keyring.
type Key struct {
	Description string
	Fingerprint string
	Type        string
}

// GetKeys returns a list of keys in the requested keyring.
func GetKeys(_ context.Context, keyring string) ([]Key, error) {
	keys := []Key{}

	// Determine Secure Boot state.
	sbEnabled, err := secureboot.Enabled()
	if err != nil {
		return nil, err
	}

	// In normal operation, Secure Boot will be enabled and we can
	// directly fetch the actual kernel keyring.
	if sbEnabled {
		// Read the key list.
		fd, err := os.Open("/proc/keys")
		if err != nil {
			return nil, err
		}

		defer fd.Close()

		// Iterate over the entries..
		fdScan := bufio.NewScanner(fd)
		for fdScan.Scan() {
			fields := strings.Fields(fdScan.Text())

			if len(fields) < 10 {
				// Skipping invalid entries.
				continue
			}

			if fields[4] != keyring {
				// Skipping entries outside of the platform (SecureBoot) keyring.
				continue
			}

			keyFields := strings.Split(strings.Join(fields[8:], " "), ": ")

			keys = append(keys, Key{
				Description: strings.Join(keyFields[0:len(keyFields)-2], ": "),
				Fingerprint: keyFields[len(keyFields)-2],
				Type:        strings.Fields(keyFields[len(keyFields)-1])[0],
			})
		}

		return keys, nil
	}

	// When Secure Boot is disabled, rely on any certificates present in /usr/lib/incus-osd/certs/db.crt.
	// Since that file is part of the usr-verity image, it is read-only and the verity image
	// has been verified both during install/upgrade as well as boot-time checks against the TPM
	// event log. Therefore it should to be relatively safe to trust the contents.
	certs, err := util.GetFilesystemTrustedCerts("db.crt")
	if err != nil {
		return nil, err
	}

	for _, cert := range certs {
		description := ""
		val, ok := cert.Subject.Names[0].Value.(string)

		if ok {
			description = val
		}

		keys = append(keys, Key{
			Description: description,
			Fingerprint: hex.EncodeToString(cert.SubjectKeyId),
			Type:        "X509.rsa",
		})
	}

	return keys, nil
}
