package services

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"strings"

	"github.com/lxc/incus/v6/shared/subprocess"

	"github.com/lxc/incus-os/incus-osd/api"
	"github.com/lxc/incus-os/incus-osd/internal/state"
	"github.com/lxc/incus-os/incus-osd/internal/systemd"
)

// Linstor represents the system OVS/Linstor service.
type Linstor struct {
	common

	state *state.State
}

// Get returns the current service state.
func (n *Linstor) Get(_ context.Context) (any, error) {
	return n.state.Services.Linstor, nil
}

// Update updates the service configuration.
func (n *Linstor) Update(ctx context.Context, req any) error {
	newState, ok := req.(*api.ServiceLinstor)
	if !ok {
		return fmt.Errorf("request type \"%T\" isn't expected ServiceLinstor", req)
	}

	// Save the state on return.
	defer n.state.Save()

	// Stop any running daemon.
	if n.state.Services.Linstor.Config.Enabled {
		err := n.Stop(ctx)
		if err != nil {
			return err
		}
	}

	// Update the configuration.
	n.state.Services.Linstor.Config = newState.Config

	// Start the daemon if needed.
	if n.state.Services.Linstor.Config.Enabled {
		err := n.Start(ctx)
		if err != nil {
			return err
		}
	}

	return nil
}

// Stop stops the service.
func (n *Linstor) Stop(ctx context.Context) error {
	if !n.state.Services.Linstor.Config.Enabled {
		return nil
	}

	// Stop Linstor Satellite.
	err := systemd.StopUnit(ctx, "linstor-satellite.service")
	if err != nil {
		return err
	}

	return nil
}

// Start starts the service.
func (n *Linstor) Start(ctx context.Context) error {
	config := n.state.Services.Linstor.Config

	if !config.Enabled {
		return nil
	}

	// Parse the config.
	isTLS := config.TLSServerCertificate != "" && config.TLSServerKey != "" && len(config.TLSTrustedCertificates) > 0

	bindType := "plain"
	bindAddress := "[::]"
	bindPort := "3366"

	if isTLS {
		bindType = "ssl"
		bindPort = "3367"
	}

	if config.ListenAddress != "" {
		var err error

		bindAddress, bindPort, err = net.SplitHostPort(config.ListenAddress)
		if err != nil {
			return err
		}
	}

	// Create the config directory.
	err := os.RemoveAll("/etc/linstor")
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}

	err = os.Mkdir("/etc/linstor", 0o700)
	if err != nil && !errors.Is(err, os.ErrExist) {
		return err
	}

	// Create the config file.
	wr, err := os.Create("/etc/linstor/linstor_satellite.toml")
	if err != nil {
		return err
	}

	err = wr.Chmod(0o600)
	if err != nil {
		return err
	}

	defer func() { _ = wr.Close() }()

	// Write the config file.
	_, err = fmt.Fprintf(wr, `[netcom]
  type = "%s"
  bind_address = "%s"
  port = %s
`, bindType, bindAddress, bindPort)
	if err != nil {
		return err
	}

	if isTLS {
		// TLS configuration.
		_, err = fmt.Fprint(wr, `
  server_certificate="/etc/linstor/server.jks"
  trusted_certificates="/etc/linstor/trusted.jks"
  key_password="linstor"
  keystore_password="linstor"
  truststore_password="linstor"
  ssl_protocol="TLSv1.3"
`)
		if err != nil {
			return err
		}

		// TLS files.
		err = os.WriteFile("/etc/linstor/server.crt", []byte(config.TLSServerCertificate), 0o600)
		if err != nil {
			return err
		}

		err = os.WriteFile("/etc/linstor/server.key", []byte(config.TLSServerKey), 0o600)
		if err != nil {
			return err
		}

		err = os.WriteFile("/etc/linstor/trusted.crt", []byte(strings.Join(config.TLSTrustedCertificates, "\n")), 0o600)
		if err != nil {
			return err
		}

		// Java keystores.
		_, err = subprocess.RunCommandContext(ctx, "openssl", "pkcs12", "-export", "-in", "/etc/linstor/server.crt", "-inkey", "/etc/linstor/server.key", "-out", "/etc/linstor/server.p12", "-name", "certificate", "-password", "pass:linstor")
		if err != nil {
			return err
		}

		_, err = subprocess.RunCommandContext(ctx, "keytool", "-importkeystore", "-srckeystore", "/etc/linstor/server.p12", "-srcstoretype", "pkcs12", "-destkeystore", "/etc/linstor/server.jks", "-srcstorepass", "linstor", "-deststorepass", "linstor")
		if err != nil {
			return err
		}

		_, err = subprocess.RunCommandContext(ctx, "keytool", "-keystore", "/etc/linstor/trusted.jks", "-importcert", "-file", "/etc/linstor/trusted.crt", "-alias", "trusted", "-deststorepass", "linstor", "-noprompt")
		if err != nil {
			return err
		}
	}

	// Configure DRBD.
	err = os.Mkdir("/etc/drbd.d", 0o755)
	if err != nil && !errors.Is(err, os.ErrExist) {
		return err
	}

	err = os.WriteFile("/etc/drbd.conf", []byte("include \"drbd.d/*.res\";\n"), 0o644)
	if err != nil {
		return err
	}

	// Start Linstor Satellite.
	err = systemd.EnableUnit(ctx, true, "linstor-satellite.service")
	if err != nil {
		return err
	}

	return nil
}

// ShouldStart returns true if the service should be started on boot.
func (n *Linstor) ShouldStart() bool {
	return n.state.Services.Linstor.Config.Enabled
}

// Struct returns the API struct for the Linstor service.
func (*Linstor) Struct() any {
	return &api.ServiceLinstor{}
}

// Supported returns whether the system can use Linstor.
func (n *Linstor) Supported() bool {
	// Linstor requires incus-linstor to be installed.
	_, ok := n.state.Applications["incus-linstor"]

	return ok
}
