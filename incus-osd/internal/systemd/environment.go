package systemd

import (
	"errors"
	"fmt"
	"os"

	"github.com/lxc/incus-os/incus-osd/api"
)

// UpdateEnvironment updates the system-wide /etc/environment file as well as
// updating the environment variables to the daemon's environment. For simplicity,
// the existing /etc/environment file is deleted, then re-created with whatever is
// passed to this function.
func UpdateEnvironment(proxyCfg *api.SystemNetworkProxy) error {
	// Remove any existing /etc/environment file.
	err := os.Remove("/etc/environment")
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}

	// Unset proxy environment variables.
	for _, envVarName := range []string{"http_proxy", "https_proxy", "no_proxy"} {
		err = os.Unsetenv(envVarName)
		if err != nil {
			return err
		}
	}

	// If no proxy configuration provided, return here.
	if proxyCfg == nil {
		return nil
	}

	// Set any defined environment variables.
	if proxyCfg.HTTPProxy != "" {
		err := writeAndSet("http_proxy", proxyCfg.HTTPProxy)
		if err != nil {
			return err
		}
	}

	if proxyCfg.HTTPSProxy != "" {
		err := writeAndSet("https_proxy", proxyCfg.HTTPSProxy)
		if err != nil {
			return err
		}
	}

	if proxyCfg.NoProxy != "" {
		err := writeAndSet("no_proxy", proxyCfg.NoProxy)
		if err != nil {
			return err
		}
	}

	return nil
}

func writeAndSet(key string, value string) error {
	envFile, err := os.OpenFile("/etc/environment", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o0644) //nolint:gosec
	if err != nil {
		return err
	}
	defer envFile.Close()

	_, err = fmt.Fprintf(envFile, "%s=%s\n", key, value)
	if err != nil {
		return err
	}

	return os.Setenv(key, value)
}
