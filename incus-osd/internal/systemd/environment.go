package systemd

import (
	"fmt"
	"os"

	"github.com/lxc/incus-os/incus-osd/api"
)

// SetProxyEnvironment appends specified proxy variables to /etc/environment
// and adds them to the current process' environment variables.
func SetProxyEnvironment(cfg api.SystemNetworkProxy) error {
	writeAndSet := func(key string, value string) error {
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

	if cfg.HTTPProxy != "" {
		err := writeAndSet("http_proxy", cfg.HTTPProxy)
		if err != nil {
			return err
		}
	}

	if cfg.HTTPSProxy != "" {
		err := writeAndSet("https_proxy", cfg.HTTPSProxy)
		if err != nil {
			return err
		}
	}

	if cfg.NoProxy != "" {
		err := writeAndSet("no_proxy", cfg.NoProxy)
		if err != nil {
			return err
		}
	}

	return nil
}
