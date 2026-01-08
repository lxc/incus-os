package main

import (
	"context"
	"os"

	"github.com/lxc/incus/v6/shared/subprocess"
)

// Generate a detached signature if provided with a signing certificate.
func sign(ctx context.Context, src string, dst string) error {
	if (os.Getenv("SIG_KEY") == "" && os.Getenv("SIG_TPM_KEY") == "") || os.Getenv("SIG_CERTIFICATE") == "" || os.Getenv("SIG_CHAIN") == "" {
		return nil
	}

	// Generate an SMIME signature.
	args := []string{"smime", "-text", "-sign"}
	if os.Getenv("SIG_TPM_KEY") != "" {
		args = append(args, "-provider", "tpm2", "-provider", "default", "-propquery", "?provider=tpm2", "-nodetach", "-md", "sha256")
	}

	args = append(args, "-signer", os.Getenv("SIG_CERTIFICATE"))

	if os.Getenv("SIG_TPM_KEY") != "" {
		args = append(args, "-inkey", "handle:"+os.Getenv("SIG_TPM_KEY"))
	} else {
		args = append(args, "-inkey", os.Getenv("SIG_KEY"))
	}

	args = append(args, "-in", src, "-out", dst, "-certfile", os.Getenv("SIG_CHAIN"))

	_, err := subprocess.RunCommandContext(ctx, "openssl", args...)
	if err != nil {
		return err
	}

	return nil
}
