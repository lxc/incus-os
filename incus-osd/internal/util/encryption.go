package util

import (
	"errors"
	"io"
	"os"
)

// GenerateEncryptionKeyFile generates a standard 32 bytes random encryption key and writes it to the target file.
func GenerateEncryptionKeyFile(keyfilePath string) error {
	devUrandom, err := os.OpenFile("/dev/urandom", os.O_RDONLY, 0o0600)
	if err != nil {
		return err
	}
	defer devUrandom.Close()

	// #nosec G304
	keyfile, err := os.OpenFile(keyfilePath, os.O_CREATE|os.O_WRONLY, 0o0600)
	if err != nil {
		return err
	}
	defer keyfile.Close()

	count, err := io.CopyN(keyfile, devUrandom, 32)
	if err != nil {
		return err
	}

	if count != 32 {
		// Remove the bad encryption key file, if it exists.
		_ = os.Remove(keyfilePath)

		return errors.New("failed to read 32 bytes from /dev/urandom")
	}

	return nil
}
