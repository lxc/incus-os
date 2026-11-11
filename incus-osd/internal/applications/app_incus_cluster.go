package applications

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"strings"

	"github.com/lxc/incus/v7/shared/subprocess"
	"go.yaml.in/yaml/v4"

	"github.com/lxc/incus-os/incus-osd/internal/systemd"
)

// incusClusterMember matches the cluster member format used by "incusd cluster show" and "incusd cluster edit".
type incusClusterMember struct {
	ID      uint64 `yaml:"id"`
	Name    string `yaml:"name,omitempty"`
	Address string `yaml:"address"`
	Role    string `yaml:"role"`
}

// incusClusterConfig matches the cluster configuration format used by "incusd cluster show" and "incusd cluster edit".
type incusClusterConfig struct {
	Members []incusClusterMember `yaml:"members"`
}

// clusterSetAddresses updates the address of cluster members in the local database, keyed by member name.
func (*incus) clusterSetAddresses(ctx context.Context, addresses map[string]string) error {
	if len(addresses) == 0 {
		return errors.New("no cluster member address provided")
	}

	// The database can only be modified while Incus is stopped.
	if systemd.IsActive(ctx, "incus.service") || systemd.IsActive(ctx, "incus.socket") {
		return errors.New("the Incus application must be stopped on all cluster members before changing addresses")
	}

	// Get the current cluster configuration.
	output, err := subprocess.RunCommandContext(ctx, "/opt/incus/lib/systemd/incusd", "cluster", "show")
	if err != nil {
		return err
	}

	config := incusClusterConfig{}

	err = yaml.Unmarshal([]byte(output), &config)
	if err != nil {
		return err
	}

	if len(config.Members) == 0 {
		return errors.New("this server isn't part of a cluster")
	}

	// Apply the new addresses.
	var localPatch strings.Builder

	for name, address := range addresses {
		host, port, err := net.SplitHostPort(address)
		if err != nil || host == "" || port == "" || strings.ContainsAny(address, `'"`) {
			return fmt.Errorf("invalid address %q for cluster member %q, must be in host:port format", address, name)
		}

		found := false

		for i, member := range config.Members {
			if member.Name != name {
				continue
			}

			found = true

			if member.Address == address {
				break
			}

			// Keep core.https_address in sync on the member that's listening on its old cluster address.
			fmt.Fprintf(&localPatch, "UPDATE config SET value = '%s' WHERE key = 'core.https_address' AND value = '%s';\n", address, strings.ReplaceAll(member.Address, "'", "''"))

			config.Members[i].Address = address

			break
		}

		if !found {
			return fmt.Errorf("cluster member %q not found", name)
		}
	}

	// Nothing to do if all members already have the requested address.
	if localPatch.Len() == 0 {
		return nil
	}

	seen := map[string]bool{}

	for _, member := range config.Members {
		if seen[member.Address] {
			return fmt.Errorf("address %q would be used by more than one cluster member", member.Address)
		}

		seen[member.Address] = true
	}

	// Update the raft configuration, which also queues the matching update of the global database.
	content, err := yaml.Marshal(config)
	if err != nil {
		return err
	}

	err = subprocess.RunCommandWithFds(ctx, bytes.NewReader(content), nil, "/opt/incus/lib/systemd/incusd", "cluster", "edit")
	if err != nil {
		return err
	}

	// Queue the update of the local database, applied on the next Incus startup.
	f, err := os.OpenFile("/var/lib/incus/database/patch.local.sql", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}

	defer f.Close()

	_, err = f.WriteString(localPatch.String())
	if err != nil {
		return err
	}

	return f.Close()
}
