package auth

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"os"
	"strings"
	"time"

	"github.com/lxc/incus/v6/shared/subprocess"

	apiupdate "github.com/lxc/incus-os/incus-osd/api/images"
)

const (
	ctxPrimaryPath = "/run/incus-os/tpm-auth.primary.ctx"
	ctxKeyPath     = "/run/incus-os/tpm-auth.key.ctx"

	privatePath = "/var/lib/incus-os/tpm-auth.priv"
	publicPath  = "/var/lib/incus-os/tpm-auth.pub"
	pemPath     = "/var/lib/incus-os/tpm-auth.pem"

	authCertPath = "/usr/lib/incus-osd/certs/auth.crt"
)

func ensureSigningKey(ctx context.Context) error {
	// Setup a primary context if missing.
	_, err := os.Stat(ctxPrimaryPath)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return err
		}

		_, err = subprocess.RunCommandContext(ctx, "tpm2_createprimary", "-C", "e", "-c", ctxPrimaryPath)
		if err != nil {
			return err
		}
	}

	// Check of a signing key.
	_, err = os.Stat(privatePath)
	if err != nil {
		// Create a new key.
		_, err = subprocess.RunCommandContext(ctx, "tpm2_create", "-G", "ecc", "-u", publicPath, "-r", privatePath, "-C", ctxPrimaryPath, "-f", "pem", "-o", pemPath)
		if err != nil {
			return err
		}

		_, err = subprocess.RunCommandContext(ctx, "tpm2_load", "-C", ctxPrimaryPath, "-u", publicPath, "-r", privatePath, "-c", ctxKeyPath)
		if err != nil {
			return err
		}

		_, err = subprocess.RunCommandContext(ctx, "tpm2_evictcontrol", "-C", "o", "-c", ctxKeyPath)
		if err != nil {
			return err
		}
	} else {
		// Load the existing key.
		_, err = subprocess.RunCommandContext(ctx, "tpm2_load", "-C", ctxPrimaryPath, "-u", publicPath, "-r", privatePath, "-c", ctxKeyPath)
		if err != nil {
			return err
		}
	}

	return nil
}

// publicKey returns the PEM encoded public certificate for the system TPM-tied certificate.
func publicKey(ctx context.Context) (string, error) {
	// Ensure we have a key.
	err := ensureSigningKey(ctx)
	if err != nil {
		return "", err
	}

	// Get the PEM encoded public key.
	content, err := os.ReadFile(pemPath)
	if err != nil {
		return "", err
	}

	return string(content), nil
}

// GenerateRegistration generates a registration request struct.
func GenerateRegistration(ctx context.Context, machineID string, token string) (*apiupdate.AuthenticationRegister, error) {
	pubKey, err := publicKey(ctx)
	if err != nil {
		return nil, err
	}

	initialToken, err := GenerateToken(ctx, machineID)
	if err != nil {
		return nil, err
	}

	req := apiupdate.AuthenticationRegister{
		MachineID: machineID,
		PublicKey: pubKey,
		Token:     token,
		Initial:   initialToken,
	}

	return &req, nil
}

// GenerateToken generates a new signed authentication token.
func GenerateToken(ctx context.Context, machineID string) (string, error) {
	// Check if supported on system.
	_, err := os.Stat(authCertPath)
	if err != nil {
		return "", errors.New("authentication token generation isn't supported on this system")
	}

	var out strings.Builder

	// Ensure we have a key.
	err = ensureSigningKey(ctx)
	if err != nil {
		return "", err
	}

	_, err = out.WriteString(machineID + ":")
	if err != nil {
		return "", err
	}

	// Prepare the token.
	authToken := apiupdate.AuthenticationToken{
		MachineID: machineID,
		Timestamp: time.Now().UTC().Unix(),
	}

	tokenStr, err := json.Marshal(authToken)
	if err != nil {
		return "", err
	}

	// Encrypt the data.
	var enc bytes.Buffer

	b64 := base64.NewEncoder(base64.RawStdEncoding, &enc)

	defer func() { _ = b64.Close() }()

	gz, err := gzip.NewWriterLevel(b64, 9)
	if err != nil {
		return "", err
	}

	defer func() { _ = gz.Close() }()

	var stdout bytes.Buffer

	err = subprocess.RunCommandWithFds(ctx, bytes.NewReader(tokenStr), io.MultiWriter(&stdout, gz), "openssl", "smime", "-encrypt", "-binary", "-aes-256-cbc", "-in", "/dev/stdin", "-out", "/dev/stdout", "-outform", "DER", authCertPath)
	if err != nil {
		return "", err
	}

	// Flush all writes.
	err = gz.Close()
	if err != nil {
		return "", err
	}

	err = b64.Close()
	if err != nil {
		return "", err
	}

	// Add encrypted blob to token.
	_, err = out.WriteString(enc.String() + ":")
	if err != nil {
		return "", err
	}

	// Sign the data.
	var sig bytes.Buffer

	b64 = base64.NewEncoder(base64.RawStdEncoding, &sig)

	defer func() { _ = b64.Close() }()

	gz, err = gzip.NewWriterLevel(b64, 9)
	if err != nil {
		return "", err
	}

	defer func() { _ = gz.Close() }()

	err = subprocess.RunCommandWithFds(ctx, bytes.NewBuffer(stdout.Bytes()), gz, "tpm2_sign", "-c", ctxKeyPath, "--hash-algorithm", "sha256", "--scheme", "ecdsa", "--format", "plain", "--signature", "/dev/stdout", "/dev/stdin")
	if err != nil {
		return "", err
	}

	// Flush all writes.
	err = gz.Close()
	if err != nil {
		return "", err
	}

	err = b64.Close()
	if err != nil {
		return "", err
	}

	// Add signature to token.
	_, err = out.WriteString(sig.String())
	if err != nil {
		return "", err
	}

	return out.String(), nil
}
