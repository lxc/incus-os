package main

import (
	"context"
	"os"

	"github.com/lxc/incus/v6/shared/subprocess"
)

// Generate a detached signature if provided with a signing certificate.
func sign(ctx context.Context, src string, dst string) error {
	if os.Getenv("SIG_KEY") == "" || os.Getenv("SIG_CERTIFICATE") == "" || os.Getenv("SIG_CHAIN") == "" {
		return nil
	}

	// Generate an SMIME signature.
	_, err := subprocess.RunCommandContext(ctx, "openssl", "smime", "-text", "-sign", "-signer", os.Getenv("SIG_CERTIFICATE"), "-inkey", os.Getenv("SIG_KEY"), "-in", src, "-out", dst, "-certfile", os.Getenv("SIG_CHAIN"))
	if err != nil {
		return err
	}

	return nil
}
