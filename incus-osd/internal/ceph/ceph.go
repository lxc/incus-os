package ceph

import (
	"bytes"
	"context"
	"embed"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"slices"
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/google/uuid"
	incus "github.com/lxc/incus/v7/client"
	incusapi "github.com/lxc/incus/v7/shared/api"
	"gopkg.in/ini.v1"

	"github.com/lxc/incus-os/incus-osd/api"
)

//go:embed *.sh
var embeddedScripts embed.FS

var cephDockerHost = "quay.io"

var cephDockerImage = "ceph/ceph:v20"

var cephControlContainerNames = []string{"ceph-central01", "ceph-central02", "ceph-central03"}

var projectName = "internal"

type shTemplate struct {
	DEVICE_CLASS string //nolint:revive
	DEVICE_PATH  string //nolint:revive
	FSID         string //nolint:revive
	INST_IPV6    string //nolint:revive
	NET_IPV6     string //nolint:revive
	INST_NAME    string //nolint:revive
}

type clusterConfigFiles struct {
	Conf         []byte
	AdminKeyring []byte
	MonKeyring   []byte
}

type osdCreationInfo struct {
	Host        string `json:"host"`
	DeviceID    string `json:"device_id"`
	DeviceClass string `json:"device_class"`
}

// InitializeCephCluster creates an initial Ceph cluster consisting of three control plane
// servers. OSDs must be added separately. It also ensures the "incus-ceph" application is
// installed on each member of the Incus cluster, and that the Ceph service is properly configured.
//
// Configuration fields:
//
//	control_servers -- A comma-separated list of three Incus servers to use for hosting the Ceph control plane.
//	                   If not specified, the first three reported cluster members will be used.
//	network -- The Incus network for the Ceph cluster to use. If not specified, defaults to "meshbr0".
func InitializeCephCluster(ctx context.Context, config map[string]string) error {
	incusClient, err := incus.ConnectIncusUnixWithContext(ctx, "", nil)
	if err != nil {
		return err
	}

	//
	// Start by checking the provided configuration.
	//

	if config == nil {
		config = make(map[string]string)
	}

	// If no network was provided, supply a default.
	if config["network"] == "" {
		config["network"] = "meshbr0"
	}

	// Ensure the specified network exists.
	_, _, err = incusClient.GetNetwork(config["network"])
	if err != nil {
		return errors.New("the Incus network '" + config["network"] + "' doesn't exist")
	}

	// Ensure the 'internal' project exists.
	project, projectEtag, err := incusClient.GetProject(projectName)
	if err != nil {
		return errors.New("the Incus project '" + projectName + "' doesn't exist")
	}

	// Ensure we're running in an Incus cluster.
	if !incusClient.IsClustered() {
		return errors.New("can only deploy Ceph in an Incus cluster")
	}

	clusterMembers, err := incusClient.GetClusterMembers()
	if err != nil {
		return err
	}

	// Ensure the cluster consists of at least three members.
	if len(clusterMembers) < 3 {
		return errors.New("the Incus cluster must consist of at least three servers")
	}

	// Ensure that each cluster member is online and doesn't have a Ceph service already configured.
	for _, member := range clusterMembers {
		if member.Status != "Online" {
			return errors.New("Incus server '" + member.ServerName + "' state isn't Online: " + member.Status)
		}

		// Get the current ceph service configuration, if any.
		cephService := api.ServiceCeph{}

		resp, _, err := incusClient.RawQuery("GET", "/os/1.0/services/ceph?target="+member.ServerName, nil, "")
		if err == nil {
			err := json.Unmarshal(resp.Metadata, &cephService)
			if err != nil {
				return err
			}

			_, exists := cephService.Config.Clusters["ceph"]
			if exists {
				return errors.New("the Ceph service is already configured for cluster 'ceph' on Incus Server '" + member.ServerName + "'")
			}
		}
	}

	controlServers := []string{}

	if config["control_servers"] != "" {
		controlServers = strings.Split(config["control_servers"], ",")
	}

	// Ensure valid members were specified for hosting the Ceph control plane.
	switch len(controlServers) {
	case 0:
		// If no specific Incus servers were specified, arbitrarily pick the first three
		// reported cluster members.
		for i := range 3 {
			controlServers = append(controlServers, clusterMembers[i].ServerName)
		}
	case 3:
		// If three specific Incus servers were specified, make sure they exist.
		for i := range 3 {
			if !slices.ContainsFunc(clusterMembers, func(m incusapi.ClusterMember) bool {
				return m.ServerName == controlServers[i]
			}) {
				return errors.New("specified Incus server '" + controlServers[i] + "' isn't a member of the cluster")
			}
		}
	default:
		return errors.New("exactly zero or three Incus servers must be defined for hosting the Ceph data plane")
	}

	incusClient = incusClient.UseProject(projectName)

	// Check if any of the Ceph control plane containers currently exist.
	for _, srv := range cephControlContainerNames {
		_, _, err := incusClient.GetInstance(srv)
		if err == nil {
			return errors.New("a Ceph container '" + srv + "' currently exists; refusing to attempt to initialize a new Ceph cluster")
		}
	}

	//
	// Ensure the "incus-ceph" application is installed on each cluster member.
	//

	type applicationPost struct {
		Name string `json:"name"`
	}

	for _, member := range clusterMembers {
		resp, _, err := incusClient.RawQuery("POST", "/os/1.0/applications?target="+member.ServerName, applicationPost{Name: "incus-ceph"}, "")
		if err != nil && err.Error() != "already exists" {
			return err
		} else if resp.StatusCode != http.StatusOK && resp.Error != "already exists" {
			return errors.New("bad response: " + resp.Error)
		}
	}

	//
	// Save cluster-wide Ceph configuration.
	//

	project.Config["user.ceph.fsid"] = uuid.NewString()
	project.Config["user.ceph.network"] = config["network"]

	err = incusClient.UpdateProject(projectName, project.ProjectPut, projectEtag)
	if err != nil {
		return err
	}

	//
	// Deploy the Ceph control plane.
	//

	// Deploy the initial Ceph control plane server.
	err = deployCephContainer(ctx, controlServers[0], cephControlContainerNames[0], "ceph-initial.sh", nil)
	if err != nil {
		return err
	}

	// Deploy the other two Ceph control plane servers.
	for i, srv := range cephControlContainerNames[1:] {
		err := deployCephContainer(ctx, controlServers[i+1], srv, "ceph-additional.sh", nil)
		if err != nil {
			return err
		}
	}

	//
	// Configure the IncusOS Ceph service on each cluster member.
	//

	cephConfigFiles, err := getCephClusterConfigFiles(ctx)
	if err != nil {
		return err
	}

	// Extract the client key.
	parsedConfig, err := ini.Load(cephConfigFiles.AdminKeyring)
	if err != nil {
		return err
	}

	clientKey, err := parsedConfig.Section("client.admin").Key("key").String(), nil
	if err != nil {
		return err
	}

	monAddrs := []string{}

	// Get the IP of each Ceph monitor.
	for _, cephServerName := range cephControlContainerNames {
		ipv6Addr, err := getInstanceIPv6Addr(ctx, cephServerName)
		if err != nil {
			return err
		}

		monAddrs = append(monAddrs, ipv6Addr)
	}

	// For each member of the cluster, ensure the incus-ceph service is properly configured.
	// Using incusClient.UseTarget(host.IncusServerName) to properly switch targets
	// doesn't work with raw queries.
	for _, member := range clusterMembers {
		// Get the current ceph service configuration.
		cephService := api.ServiceCeph{}

		resp, _, err := incusClient.RawQuery("GET", "/os/1.0/services/ceph?target="+member.ServerName, nil, "")
		if err != nil {
			return err
		} else if resp.StatusCode != http.StatusOK {
			return errors.New("bad response: " + resp.Error)
		}

		err = json.Unmarshal(resp.Metadata, &cephService)
		if err != nil {
			return err
		}

		// Ensure the service is enabled.
		cephService.Config.Enabled = true

		if cephService.Config.Clusters == nil {
			cephService.Config.Clusters = make(map[string]api.ServiceCephCluster)
		}

		// Inform the service about our Ceph cluster.
		cephService.Config.Clusters["ceph"] = api.ServiceCephCluster{
			FSID:     project.Config["user.ceph.fsid"],
			Monitors: monAddrs,
			Keyrings: map[string]api.ServiceCephKeyring{
				"admin": {
					Key: clientKey,
				},
			},
		}

		// Update the ceph service configuration.
		resp, _, err = incusClient.RawQuery("PUT", "/os/1.0/services/ceph?target="+member.ServerName, cephService, "")
		if err != nil {
			return err
		} else if resp.StatusCode != http.StatusOK {
			return errors.New("bad response: " + resp.Error)
		}
	}

	return nil
}

// AddOSD adds a Ceph OSD with storage backing from the local server. If the local server
// doesn't have an OSD instance yet, one is created; otherwise the drive is attached to the
// existing instance and an additional OSD is provisioned in it.
//
// Configuration fields:
//
//	device_id -- The ID of the raw device that should be used by the OSD. Will be LUKS
//	             encrypted if not already encrypted prior to use.
func AddOSD(ctx context.Context, config map[string]string) error {
	incusClient, err := incus.ConnectIncusUnixWithContext(ctx, "", nil)
	if err != nil {
		return err
	}

	server, _, err := incusClient.GetServer()
	if err != nil {
		return err
	}

	containerName := "ceph-osd-" + server.Environment.ServerName

	//
	// Ensure the local raw device is encrypted.
	//

	encryptedDeviceID, deviceClass, err := ensureDeviceIsEncrypted(ctx, config["device_id"])
	if err != nil {
		return err
	}

	//
	// Check if an OSD instance already exists on this host.
	//

	incusClient = incusClient.UseProject(projectName)

	instance, etag, err := incusClient.GetInstance(containerName)
	if err != nil {
		if !incusapi.StatusErrorCheck(err, http.StatusNotFound) {
			return err
		}

		// Create the new OSD instance.
		return deployCephContainer(ctx, server.Environment.ServerName, containerName, "ceph-osd.sh", &osdCreationInfo{
			Host:        server.Environment.ServerName,
			DeviceID:    encryptedDeviceID,
			DeviceClass: deviceClass,
		})
	}

	//
	// Attach the drive to the existing OSD instance.
	//

	if instance.Devices == nil {
		instance.Devices = map[string]map[string]string{}
	}

	// Ensure the drive isn't already attached and find the next free device name.
	nextIndex := 1

	for name, device := range instance.Devices {
		if device["type"] != "unix-block" {
			continue
		}

		if device["source"] == encryptedDeviceID {
			return errors.New("device '" + config["device_id"] + "' is already used by an OSD on Incus server " + server.Environment.ServerName)
		}

		index, err := strconv.Atoi(strings.TrimPrefix(name, "ceph-"))
		if err == nil && index >= nextIndex {
			nextIndex = index + 1
		}
	}

	devicePath := "/dev/ceph-" + strconv.Itoa(nextIndex)

	instance.Devices["ceph-"+strconv.Itoa(nextIndex)] = map[string]string{
		"type":   "unix-block",
		"source": encryptedDeviceID,
		"path":   devicePath,
		"uid":    "167",
		"gid":    "167",
	}

	op, err := incusClient.UpdateInstance(containerName, instance.InstancePut, etag)
	if err != nil {
		return err
	}

	err = op.Wait()
	if err != nil {
		return err
	}

	// Provision the new OSD.
	return execCephScript(incusClient, containerName, "ceph-osd.sh", shTemplate{
		DEVICE_CLASS: deviceClass,
		DEVICE_PATH:  devicePath,
	})
}

// RefreshCephOCIImages refreshes the OCI image used by the Ceph containers.
//
// The current remote OCI image is compared with the one used to spawn a Ceph
// container running on the local system (avoiding any architecture mismatch).
// If they differ, all the containers get rebuilt, each cluster member
// downloading the image matching its own architecture.
//
// Configuration fields:
//
//	oci_tag -- Optional; if specified, set the Ceph OCI image tag to the provided value.
//	           This is useful for performing major version updates, such as from v19 to
//	           v20, or pinning to an exact version.
func RefreshCephOCIImages(ctx context.Context, config map[string]string) error {
	// Temporarily add /opt/incus/bin to $PATH so the skopeo binary bundled with Incus
	// can be found. We don't just add /opt/incus/bin to the incus-osd service definition
	// because of conflicts with various tpm2_* commands.
	originalPath := os.Getenv("PATH")

	err := os.Setenv("PATH", originalPath+":/opt/incus/bin")
	if err != nil {
		return err
	}

	defer func() {
		_ = os.Setenv("PATH", originalPath)
	}()

	incusClient, err := incus.ConnectIncusUnixWithContext(ctx, "", nil)
	if err != nil {
		return err
	}

	server, _, err := incusClient.GetServer()
	if err != nil {
		return err
	}

	incusClient = incusClient.UseProject(projectName)

	cephContainers := cephControlContainerNames

	// Add any existing OSDs to the list of containers to refresh.
	osdHosts, err := getOSDHosts(ctx)
	if err != nil {
		return err
	}

	for _, host := range osdHosts {
		cephContainers = append(cephContainers, "ceph-osd-"+host)
	}

	// Find a Ceph container running on the local system.
	var localInstance *incusapi.Instance

	for _, containerName := range cephContainers {
		instance, _, err := incusClient.GetInstance(containerName)
		if err != nil {
			return err
		}

		if instance.Location == server.Environment.ServerName {
			localInstance = instance

			break
		}
	}

	if localInstance == nil {
		return errors.New("no Ceph container found on Incus server " + server.Environment.ServerName)
	}

	// Determine the target image alias.
	imageAlias := localInstance.Config["image.id"]

	if config["oci_tag"] != "" {
		imageAlias = "ceph/ceph:" + config["oci_tag"]
	}

	// Get the fingerprint of the current remote OCI image for the local architecture.
	ociRemote, err := incus.ConnectOCI("https://"+cephDockerHost, nil)
	if err != nil {
		return err
	}

	alias, _, err := ociRemote.GetImageAlias(imageAlias)
	if err != nil {
		if strings.Contains(err.Error(), "manifest unknown") {
			return errors.New("OCI image '" + cephDockerHost + "/" + imageAlias + "' doesn't exist")
		}

		return err
	}

	// Nothing to do if the local container was spawned from the current remote image.
	if localInstance.Config["image.id"] == imageAlias && localInstance.Config["volatile.base_image"] == alias.Target {
		return nil
	}

	// Rebuild all the containers.
	for _, containerName := range cephContainers {
		err := refreshCephOCIImage(ctx, containerName, imageAlias)
		if err != nil {
			return err
		}

		// Sleep a few seconds to allow Ceph to stabilize its state.
		time.Sleep(10 * time.Second)
	}

	return nil
}

// GetServiceState returns the state of the managed Ceph cluster, based on Incus API data.
func GetServiceState(ctx context.Context) (*api.ApplicationIncusStateServicesCeph, error) {
	ret := &api.ApplicationIncusStateServicesCeph{}

	incusClient, err := incus.ConnectIncusUnixWithContext(ctx, "", nil)
	if err != nil {
		return nil, err
	}

	// Get the cluster FSID from the project configuration.
	project, _, err := incusClient.GetProject(projectName)
	if err != nil {
		// Without the project, no Ceph cluster was ever deployed.
		return ret, nil //nolint:nilerr
	}

	ret.FSID = project.Config["user.ceph.fsid"]
	if ret.FSID == "" {
		return ret, nil
	}

	incusClient = incusClient.UseProject(projectName)

	// Consider the cluster deployed once the initial control plane container exists.
	instance, _, err := incusClient.GetInstance(cephControlContainerNames[0])
	if err != nil {
		if incusapi.StatusErrorCheck(err, http.StatusNotFound) {
			return ret, nil
		}

		return nil, err
	}

	ret.Deployed = true

	// Report the Ceph version from the container image tag.
	_, version, ok := strings.Cut(instance.Config["image.id"], ":")
	if ok {
		ret.Version = version
	}

	// Collect the list of OSDs.
	instances, err := incusClient.GetInstances("container")
	if err != nil {
		return nil, err
	}

	for _, instance := range instances {
		host, ok := strings.CutPrefix(instance.Name, "ceph-osd-")
		if !ok {
			continue
		}

		for name, device := range instance.Devices {
			if device["type"] != "unix-block" || !strings.HasPrefix(name, "ceph-") {
				continue
			}

			ret.OSDs = append(ret.OSDs, api.ApplicationIncusStateServicesCephOSD{
				Host:   host,
				Device: device["source"],
			})
		}
	}

	slices.SortFunc(ret.OSDs, func(a, b api.ApplicationIncusStateServicesCephOSD) int {
		return strings.Compare(a.Host+a.Device, b.Host+b.Device)
	})

	return ret, nil
}

// RemoveOSD removes a Ceph OSD from the local server. If the local OSD instance has
// multiple drives attached, the drive to remove must be specified through device_id; the
// matching OSD is then stopped and its drive detached. When removing the last (or only)
// drive, the whole OSD instance is deleted instead. Because Ceph requires a minimum of
// three OSDs, removal won't be allowed if it would leave the cluster with fewer than
// three OSDs.
//
// WARNING -- This will forcefully remove the OSD from the Ceph cluster. Prior to removal,
// you should remove the OSD via Ceph's API and wait for data migration to complete.
//
// Configuration fields:
//
//	device_id -- The ID of the raw device backing the OSD to remove. Optional if the
//	             local OSD instance only has a single drive attached.
func RemoveOSD(ctx context.Context, config map[string]string) error {
	incusClient, err := incus.ConnectIncusUnixWithContext(ctx, "", nil)
	if err != nil {
		return err
	}

	server, _, err := incusClient.GetServer()
	if err != nil {
		return err
	}

	containerName := "ceph-osd-" + server.Environment.ServerName

	incusClient = incusClient.UseProject(projectName)

	//
	// Check if it's possible to remove the OSD.
	//

	instance, etag, err := incusClient.GetInstance(containerName)
	if err != nil {
		if incusapi.StatusErrorCheck(err, http.StatusNotFound) {
			return errors.New("no Ceph OSD instance exists on Incus server " + server.Environment.ServerName)
		}

		return err
	}

	// Get the list of drives attached to the local OSD instance.
	localDrives := []string{}

	for name, device := range instance.Devices {
		if device["type"] == "unix-block" && strings.HasPrefix(name, "ceph-") {
			localDrives = append(localDrives, name)
		}
	}

	// Identify the drive to remove.
	deviceName := ""

	if config["device_id"] != "" {
		encryptedDeviceID, err := getEncryptedDeviceID(ctx, config["device_id"])
		if err != nil {
			return err
		}

		for _, name := range localDrives {
			if instance.Devices[name]["source"] == encryptedDeviceID || instance.Devices[name]["source"] == config["device_id"] {
				deviceName = name

				break
			}
		}

		if deviceName == "" {
			return errors.New("device '" + config["device_id"] + "' isn't used by an OSD on Incus server " + server.Environment.ServerName)
		}
	} else if len(localDrives) > 1 {
		return errors.New("multiple drives are attached to the OSD instance on Incus server " + server.Environment.ServerName + "; a device_id must be specified")
	}

	// Ensure enough OSDs remain after the removal.
	removeCount := len(localDrives)

	if deviceName != "" && len(localDrives) > 1 {
		removeCount = 1
	}

	driveCount, err := getOSDDriveCount(ctx)
	if err != nil {
		return err
	}

	if driveCount-removeCount < 3 {
		return errors.New("a minimum of three OSDs are required by Ceph; refusing to remove OSD from Incus server " + server.Environment.ServerName)
	}

	//
	// Remove a single drive if others remain on this host.
	//

	if deviceName != "" && len(localDrives) > 1 {
		// Stop the OSD and remove its local state.
		err = execCephScript(incusClient, containerName, "ceph-osd-remove.sh", shTemplate{
			DEVICE_PATH: instance.Devices[deviceName]["path"],
		})
		if err != nil {
			return err
		}

		// Detach the drive.
		delete(instance.Devices, deviceName)

		op, err := incusClient.UpdateInstance(containerName, instance.InstancePut, etag)
		if err != nil {
			return err
		}

		return op.Wait()
	}

	//
	// Delete the OSD instance.
	//

	op, err := incusClient.UpdateInstanceState(containerName, incusapi.InstanceStatePut{
		Action:  "stop",
		Timeout: -1,
	}, "")
	if err != nil {
		return err
	}

	err = op.Wait()
	if err != nil {
		return err
	}

	op, err = incusClient.DeleteInstance(containerName)
	if err != nil {
		return err
	}

	err = op.Wait()
	if err != nil {
		return err
	}

	return nil
}

func deployCephContainer(ctx context.Context, incusTarget string, cephContainerName string, configScript string, osd *osdCreationInfo) error {
	incusClient, err := incus.ConnectIncusUnixWithContext(ctx, "", nil)
	if err != nil {
		return err
	}

	incusClient = incusClient.UseProject(projectName)
	incusClient = incusClient.UseTarget(incusTarget)

	project, _, err := incusClient.GetProject(projectName)
	if err != nil {
		return err
	}

	// Create a storage volume for /etc/ceph/ and /var/lib/ceph/.
	err = incusClient.CreateStoragePoolVolume("local", incusapi.StorageVolumesPost{
		Name: cephContainerName + "-data",
		Type: "custom",
	})
	if err != nil {
		return err
	}

	// Prepare the container's configuration and devices.
	config := map[string]string{
		"oci.entrypoint":   "/sbin/init",
		"cluster.evacuate": "stop",
	}

	devices := map[string]map[string]string{
		"eth0": {
			"type":    "nic",
			"network": project.Config["user.ceph.network"],
		},
		"etc": {
			"type":      "disk",
			"pool":      "local",
			"source":    cephContainerName + "-data/etc",
			"path":      "/etc/ceph/",
			"dependent": "true",
		},
		"var": {
			"type":      "disk",
			"pool":      "local",
			"source":    cephContainerName + "-data/var",
			"path":      "/var/lib/ceph/",
			"dependent": "true",
		},
	}

	if configScript == "ceph-osd.sh" {
		devices["ceph-1"] = map[string]string{
			"type":   "unix-block",
			"source": osd.DeviceID,
			"path":   "/dev/ceph-1",
			"uid":    "167",
			"gid":    "167",
		}
	}

	// Create and start the Ceph server. The OCI image is downloaded by the
	// target cluster member itself, so each member gets the image matching
	// its own architecture.
	op, err := incusClient.CreateInstance(incusapi.InstancesPost{
		Name: cephContainerName,
		InstancePut: incusapi.InstancePut{
			Config:  config,
			Devices: devices,
		},
		Source: incusapi.InstanceSource{
			Type:     "image",
			Protocol: "oci",
			Server:   "https://" + cephDockerHost,
			Alias:    cephDockerImage,
		},
		Type:  "container",
		Start: true,
	})
	if err != nil {
		return err
	}

	err = op.Wait()
	if err != nil {
		return err
	}

	// Allow the container to start up.
	time.Sleep(5 * time.Second)

	var templateVars shTemplate

	switch configScript {
	case "ceph-initial.sh":
		// Get the IPv6 network that Ceph will be using.
		network, _, err := incusClient.GetNetwork(project.Config["user.ceph.network"])
		if err != nil {
			return err
		}

		ipv6Net := network.Config["ipv6.address"]

		// Get the IPv6 address of the new Ceph container.
		ipv6Addr, err := getInstanceIPv6Addr(ctx, cephContainerName)
		if err != nil {
			return err
		}

		templateVars = shTemplate{
			FSID:      project.Config["user.ceph.fsid"],
			INST_IPV6: ipv6Addr,
			NET_IPV6:  ipv6Net,
			INST_NAME: cephContainerName,
		}
	case "ceph-additional.sh":
		templateVars = shTemplate{
			INST_NAME: cephContainerName,
		}

		cephConfigFiles, err := getCephClusterConfigFiles(ctx)
		if err != nil {
			return err
		}

		sftpConn, err := incusClient.GetInstanceFileSFTP(cephContainerName)
		if err != nil {
			return err
		}

		defer sftpConn.Close()

		// Push configuration files.
		file, err := sftpConn.OpenFile("/etc/ceph/ceph.conf", os.O_WRONLY|os.O_CREATE|os.O_TRUNC)
		if err != nil {
			return err
		}

		_, err = file.Write(cephConfigFiles.Conf)
		if err != nil {
			return err
		}

		_ = file.Close()

		file, err = sftpConn.OpenFile("/etc/ceph/ceph.client.admin.keyring", os.O_WRONLY|os.O_CREATE|os.O_TRUNC)
		if err != nil {
			return err
		}

		_, err = file.Write(cephConfigFiles.AdminKeyring)
		if err != nil {
			return err
		}

		_ = file.Close()

		file, err = sftpConn.OpenFile("/tmp/ceph.mon.keyring", os.O_WRONLY|os.O_CREATE|os.O_TRUNC)
		if err != nil {
			return err
		}

		_, err = file.Write(cephConfigFiles.MonKeyring)
		if err != nil {
			return err
		}

		_ = file.Close()
	case "ceph-osd.sh":
		templateVars = shTemplate{
			DEVICE_CLASS: osd.DeviceClass,
			DEVICE_PATH:  "/dev/ceph-1",
		}

		cephConfigFiles, err := getCephClusterConfigFiles(ctx)
		if err != nil {
			return err
		}

		sftpConn, err := incusClient.GetInstanceFileSFTP(cephContainerName)
		if err != nil {
			return err
		}

		defer sftpConn.Close()

		// Push configuration files.
		file, err := sftpConn.OpenFile("/etc/ceph/ceph.conf", os.O_WRONLY|os.O_CREATE|os.O_TRUNC)
		if err != nil {
			return err
		}

		_, err = file.Write(cephConfigFiles.Conf)
		if err != nil {
			return err
		}

		_ = file.Close()

		file, err = sftpConn.OpenFile("/etc/ceph/ceph.client.admin.keyring", os.O_WRONLY|os.O_CREATE|os.O_TRUNC)
		if err != nil {
			return err
		}

		_, err = file.Write(cephConfigFiles.AdminKeyring)
		if err != nil {
			return err
		}

		_ = file.Close()
	default:
		return errors.New("unrecognized configuration script: " + configScript)
	}

	// Execute the configuration script.
	return execCephScript(incusClient, cephContainerName, configScript, templateVars)
}

// execCephScript renders one of the embedded script templates and runs it in the container.
func execCephScript(incusClient incus.InstanceServer, cephContainerName string, configScript string, templateVars shTemplate) error {
	var buf bytes.Buffer

	// Parse and render the script template.
	t, err := template.ParseFS(embeddedScripts, configScript)
	if err != nil {
		return err
	}

	err = t.Execute(&buf, templateVars)
	if err != nil {
		return err
	}

	// Execute the configuration script.
	op, err := incusClient.ExecInstance(cephContainerName, incusapi.InstanceExecPost{
		Command:     []string{"sh", "-eux"},
		WaitForWS:   true,
		Interactive: false,
	}, &incus.InstanceExecArgs{
		Stdin:  &buf,
		Stdout: nil,
		Stderr: nil,
	})
	if err != nil {
		return err
	}

	err = op.Wait()
	if err != nil {
		return err
	}

	// Check the script's exit code.
	exitCode, ok := op.Get().Metadata["return"].(float64)
	if !ok || exitCode != 0 {
		return errors.New("script '" + configScript + "' failed in container " + cephContainerName)
	}

	return nil
}

func ensureDeviceIsEncrypted(ctx context.Context, deviceID string) (string, string, error) {
	incusClient, err := incus.ConnectIncusUnixWithContext(ctx, "", nil)
	if err != nil {
		return "", "", err
	}

	// Note: Every interaction with Incus in this function is calling an IncusOS API
	//       endpoint on the local host, so there's no need to set a project or target.

	resp, _, err := incusClient.RawQuery("GET", "/os/1.0/system/storage", nil, "")
	if err != nil {
		return "", "", err
	} else if resp.StatusCode != http.StatusOK {
		return "", "", errors.New("bad response: " + resp.Error)
	}

	storageInfo := api.SystemStorage{}

	err = json.Unmarshal(resp.Metadata, &storageInfo)
	if err != nil {
		return "", "", err
	}

	i := slices.IndexFunc(storageInfo.State.Drives, func(drive api.SystemStorageDrive) bool {
		return drive.ID == deviceID
	})

	if i < 0 {
		return "", "", errors.New("specified raw device '" + deviceID + "' doesn't exist")
	}

	// Determine the Ceph device class.
	var deviceClass string

	switch storageInfo.State.Drives[i].Bus {
	case "nvme":
		deviceClass = "nvme"
	case "scsi":
		deviceClass = "sdd"
	default:
		deviceClass = "hdd"
	}

	// If the drive is currently encrypted, return its encrypted device ID and class
	// as there's nothing else we need to do.
	if storageInfo.State.Drives[i].Encrypted {
		return storageInfo.State.Drives[i].EncryptedID, deviceClass, nil
	}

	// Encrypt the storage device.
	resp, _, err = incusClient.RawQuery("POST", "/os/1.0/system/storage/:encrypt-drive", api.SystemStorageEncrypt{ID: deviceID}, "")
	if err != nil {
		return "", "", err
	} else if resp.StatusCode != http.StatusOK {
		return "", "", errors.New("bad response: " + resp.Error)
	}

	// Get updated storage state and record the device's encrypted ID.
	resp, _, err = incusClient.RawQuery("GET", "/os/1.0/system/storage", nil, "")
	if err != nil {
		return "", "", err
	} else if resp.StatusCode != http.StatusOK {
		return "", "", errors.New("bad response: " + resp.Error)
	}

	storageInfo = api.SystemStorage{}

	err = json.Unmarshal(resp.Metadata, &storageInfo)
	if err != nil {
		return "", "", err
	}

	i = slices.IndexFunc(storageInfo.State.Drives, func(drive api.SystemStorageDrive) bool {
		return drive.ID == deviceID
	})

	return storageInfo.State.Drives[i].EncryptedID, deviceClass, nil
}

// getEncryptedDeviceID resolves a raw device ID to its encrypted counterpart, without
// modifying the device in any way. The ID is returned unchanged if the device isn't
// known or isn't encrypted.
func getEncryptedDeviceID(ctx context.Context, deviceID string) (string, error) {
	incusClient, err := incus.ConnectIncusUnixWithContext(ctx, "", nil)
	if err != nil {
		return "", err
	}

	resp, _, err := incusClient.RawQuery("GET", "/os/1.0/system/storage", nil, "")
	if err != nil {
		return "", err
	} else if resp.StatusCode != http.StatusOK {
		return "", errors.New("bad response: " + resp.Error)
	}

	storageInfo := api.SystemStorage{}

	err = json.Unmarshal(resp.Metadata, &storageInfo)
	if err != nil {
		return "", err
	}

	i := slices.IndexFunc(storageInfo.State.Drives, func(drive api.SystemStorageDrive) bool {
		return drive.ID == deviceID
	})

	if i < 0 || !storageInfo.State.Drives[i].Encrypted {
		return deviceID, nil
	}

	return storageInfo.State.Drives[i].EncryptedID, nil
}

func getCephClusterConfigFiles(ctx context.Context) (*clusterConfigFiles, error) {
	incusClient, err := incus.ConnectIncusUnixWithContext(ctx, "", nil)
	if err != nil {
		return nil, err
	}

	incusClient = incusClient.UseProject(projectName)

	sftpConn, err := incusClient.GetInstanceFileSFTP(cephControlContainerNames[0])
	if err != nil {
		return nil, err
	}

	defer sftpConn.Close()

	// Grab required configuration files from the initial Ceph server.
	ret := &clusterConfigFiles{}

	reader, err := sftpConn.Open("/etc/ceph/ceph.conf")
	if err != nil {
		return nil, err
	}

	ret.Conf, err = io.ReadAll(reader)
	if err != nil {
		return nil, err
	}

	_ = reader.Close()

	reader, err = sftpConn.Open("/etc/ceph/ceph.client.admin.keyring")
	if err != nil {
		return nil, err
	}

	ret.AdminKeyring, err = io.ReadAll(reader)
	if err != nil {
		return nil, err
	}

	_ = reader.Close()

	reader, err = sftpConn.Open("/var/lib/ceph/mon/ceph-" + cephControlContainerNames[0] + "/keyring")
	if err != nil {
		return nil, err
	}

	ret.MonKeyring, err = io.ReadAll(reader)
	if err != nil {
		return nil, err
	}

	_ = reader.Close()

	return ret, nil
}

func getInstanceIPv6Addr(ctx context.Context, instanceName string) (string, error) {
	incusClient, err := incus.ConnectIncusUnixWithContext(ctx, "", nil)
	if err != nil {
		return "", err
	}

	incusClient = incusClient.UseProject(projectName)

	ret := ""

	instanceState, _, err := incusClient.GetInstanceState(instanceName)
	if err != nil {
		return "", err
	}

	for _, network := range instanceState.Network {
		for _, addr := range network.Addresses {
			if addr.Family == "inet6" && !strings.HasPrefix(addr.Address, "fe80::") && addr.Address != "::1" {
				ret = addr.Address

				break
			}
		}

		if ret != "" {
			break
		}
	}

	if ret == "" {
		return "", errors.New("unable to determine IPv6 address for " + instanceName)
	}

	return ret, nil
}

func refreshCephOCIImage(ctx context.Context, containerName string, imageAlias string) error {
	incusClient, err := incus.ConnectIncusUnixWithContext(ctx, "", nil)
	if err != nil {
		return err
	}

	incusClient = incusClient.UseProject(projectName)

	// Stop the container.
	op, err := incusClient.UpdateInstanceState(containerName, incusapi.InstanceStatePut{
		Action:  "stop",
		Timeout: -1,
	}, "")
	if err != nil {
		return err
	}

	err = op.Wait()
	if err != nil {
		return err
	}

	// Rebuild the container's rootfs from the remote OCI image. The download
	// is performed by the container's own cluster member, so the image
	// matches its architecture and Incus handles the caching.
	op, err = incusClient.RebuildInstance(containerName, incusapi.InstanceRebuildPost{
		Source: incusapi.InstanceSource{
			Type:     "image",
			Protocol: "oci",
			Server:   "https://" + cephDockerHost,
			Alias:    imageAlias,
		},
	})
	if err != nil {
		return err
	}

	err = op.Wait()
	if err != nil {
		return err
	}

	// Start the container.
	op, err = incusClient.UpdateInstanceState(containerName, incusapi.InstanceStatePut{
		Action:  "start",
		Timeout: -1,
	}, "")
	if err != nil {
		return err
	}

	err = op.Wait()
	if err != nil {
		return err
	}

	// Allow the container to start up.
	time.Sleep(5 * time.Second)

	// Re-enable the various systemd services.
	if strings.HasPrefix(containerName, "ceph-osd-") {
		return execCephScript(incusClient, containerName, "ceph-refresh-osd.sh", shTemplate{})
	}

	return execCephScript(incusClient, containerName, "ceph-refresh.sh", shTemplate{INST_NAME: containerName})
}

// Return a list of Incus servers that currently host an OSD.
func getOSDHosts(ctx context.Context) ([]string, error) {
	incusClient, err := incus.ConnectIncusUnixWithContext(ctx, "", nil)
	if err != nil {
		return nil, err
	}

	incusClient = incusClient.UseProject(projectName)

	instances, err := incusClient.GetInstances("container")
	if err != nil {
		return nil, err
	}

	ret := []string{}

	for _, i := range instances {
		if strings.HasPrefix(i.Name, "ceph-osd-") {
			ret = append(ret, i.Location)
		}
	}

	return ret, nil
}

// Return the total number of OSD drives attached across all OSD instances.
func getOSDDriveCount(ctx context.Context) (int, error) {
	incusClient, err := incus.ConnectIncusUnixWithContext(ctx, "", nil)
	if err != nil {
		return 0, err
	}

	incusClient = incusClient.UseProject(projectName)

	instances, err := incusClient.GetInstances("container")
	if err != nil {
		return 0, err
	}

	count := 0

	for _, i := range instances {
		if !strings.HasPrefix(i.Name, "ceph-osd-") {
			continue
		}

		for name, device := range i.Devices {
			if device["type"] == "unix-block" && strings.HasPrefix(name, "ceph-") {
				count++
			}
		}
	}

	return count, nil
}
