package keyring

import (
	"bufio"
	"context"
	"os"
	"strings"
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
func GetKeys(ctx context.Context, keyring string) ([]Key, error) {
	keys := []Key{}

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
