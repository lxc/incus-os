package ceph

import (
	"bytes"
	"context"
	"embed"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"slices"
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

var cephDockerImage = "ceph/ceph:v20"

var cephServerNames = []string{"ceph-central01", "ceph-central02", "ceph-central03"}

type shTemplate struct {
	DEVICE_CLASS string //nolint:revive
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

type osdInfo struct {
	Host        string `json:"host"`
	DeviceID    string `json:"device_id"`
	DeviceClass string `json:"device_class"`
}

// InitializeCephCluster creates an initial Ceph cluster consisting of three control plane
// servers. OSDs must be added separately. It also ensures the "incus-ceph" application is
// installed on each member of the Incus cluster.
//
// Configuration fields:
//
//	control_servers -- A comma-separated list of three Incus servers to use for hosting the Ceph control plane.
//	                   If not specified, the first three reported cluster members will be used.
//	network -- The Incus network for the Ceph cluster to use. If not specified, defaults to "meshbr0".
//	project -- The Incus project for the Ceph cluster to use. If not specified, defaults to "internal".
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

	// If no project was provided, supply a default.
	if config["project"] == "" {
		config["project"] = "internal"
	}

	// Ensure the specified project exists.
	project, projectEtag, err := incusClient.GetProject(config["project"])
	if err != nil {
		return errors.New("the Incus project '" + config["project"] + "' doesn't exist")
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
			if clusterMembers[i].Status != "Online" {
				return errors.New("Incus server '" + clusterMembers[i].ServerName + "' state isn't Online: " + clusterMembers[i].Status)
			}

			controlServers = append(controlServers, clusterMembers[i].ServerName)
		}
	case 3:
		// If three specific Incus servers were specified, make sure they are in a good state.
		for i := range 3 {
			member, _, err := incusClient.GetClusterMember(controlServers[i])
			if err != nil {
				return err
			}

			if member.Status != "Online" {
				return errors.New("Incus server '" + member.ServerName + "' state isn't Online: " + member.Status)
			}
		}
	default:
		return errors.New("exactly zero or three Incus servers must be defined for hosting the Ceph data plane")
	}

	incusClient = incusClient.UseProject(config["project"])

	// Check if any of the Ceph control plane containers currently exist.
	for _, srv := range cephServerNames {
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

	server, serverEtag, err := incusClient.GetServer()
	if err != nil {
		return err
	}

	server.Config["user.ceph.project"] = config["project"]

	err = incusClient.UpdateServer(server.ServerPut, serverEtag)
	if err != nil {
		return err
	}

	project.Config["user.ceph.fsid"] = uuid.NewString()
	project.Config["user.ceph.network"] = config["network"]

	err = incusClient.UpdateProject(config["project"], project.ProjectPut, projectEtag)
	if err != nil {
		return err
	}

	//
	// Deploy the Ceph control plane.
	//

	// Deploy the initial Ceph control plane server.
	err = deployCephContainer(ctx, controlServers[0], cephServerNames[0], "ceph-initial.sh", nil)
	if err != nil {
		return err
	}

	// Deploy the other two Ceph control plane servers.
	for i, srv := range cephServerNames[1:] {
		err := deployCephContainer(ctx, controlServers[i+1], srv, "ceph-additional.sh", nil)
		if err != nil {
			return err
		}
	}

	return nil
}

// AddOSD adds a Ceph OSD with storage backing from the local server. Because Ceph requires
// a minimum of three OSDs, the actual OSD instances won't be created until a third OSD is
// added to the Ceph cluster. Fourth and later OSDs will be crated immediately.
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

	incusClient = incusClient.UseProject(server.Config["user.ceph.project"])

	//
	// Check that there's not already an OSD on this host.
	//

	_, _, err = incusClient.GetInstance("ceph-osd-" + server.Environment.ServerName)
	if err == nil {
		return errors.New("a Ceph OSD instance already exists on Incus server " + server.Environment.ServerName)
	}

	//
	// Ensure the local raw device is encrypted.
	//

	encryptedDeviceID, deviceClass, err := ensureDeviceIsEncrypted(ctx, config["device_id"])
	if err != nil {
		return err
	}

	//
	// Add the OSD to the cluster-wide Ceph configuration.
	//

	project, etag, err := incusClient.GetProject(server.Config["user.ceph.project"])
	if err != nil {
		return err
	}

	osds := []osdInfo{} //nolint:prealloc

	if project.Config["user.ceph.osds"] != "" {
		err := json.Unmarshal([]byte(project.Config["user.ceph.osds"]), &osds)
		if err != nil {
			return err
		}
	}

	if slices.ContainsFunc(osds, func(osd osdInfo) bool {
		return osd.Host == server.Environment.ServerName
	}) {
		return errors.New("a Ceph OSD is already configured for Incus server " + server.Environment.ServerName)
	}

	osds = append(osds, osdInfo{
		Host:        server.Environment.ServerName,
		DeviceID:    encryptedDeviceID,
		DeviceClass: deviceClass,
	})

	contents, err := json.Marshal(osds)
	if err != nil {
		return err
	}

	project.Config["user.ceph.osds"] = string(contents)

	err = incusClient.UpdateProject(server.Config["user.ceph.project"], project.ProjectPut, etag)
	if err != nil {
		return err
	}

	//
	// If there are three or more OSDs, actually create the instance(s).
	//

	if len(osds) == 3 {
		// When the third OSD is defined, deploy the initial set.
		for _, osd := range osds {
			err := deployCephContainer(ctx, osd.Host, "ceph-osd-"+osd.Host, "ceph-osd.sh", &osd)
			if err != nil {
				return err
			}
		}
	} else if len(osds) > 3 {
		// Add an OSD to the Ceph cluster.
		err := deployCephContainer(ctx, osds[len(osds)-1].Host, "ceph-osd-"+osds[len(osds)-1].Host, "ceph-osd.sh", &osds[len(osds)-1])
		if err != nil {
			return err
		}
	}

	return nil
}

// CreateCephPool initializes a new Ceph-backed Incus storage pool and adds it to the
// Incus cluster.
//
// Configuration fields:
//
//	pool_name -- The name to use when creating the new Incus storage pool. If not
//	             specified, defaults to "ceph".
func CreateCephPool(ctx context.Context, config map[string]string) error {
	incusClient, err := incus.ConnectIncusUnixWithContext(ctx, "", nil)
	if err != nil {
		return err
	}

	server, _, err := incusClient.GetServer()
	if err != nil {
		return err
	}

	incusClient = incusClient.UseProject(server.Config["user.ceph.project"])

	project, _, err := incusClient.GetProject(server.Config["user.ceph.project"])
	if err != nil {
		return err
	}

	if config == nil {
		config = make(map[string]string)
	}

	if config["pool_name"] == "" {
		config["pool_name"] = "ceph"
	}

	cephConfigFiles, err := getCephClusterConfigFiles(ctx, cephServerNames[0])
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
	for _, cephServerName := range cephServerNames {
		ipv6Addr, err := getInstanceIPv6Addr(ctx, cephServerName)
		if err != nil {
			return err
		}

		monAddrs = append(monAddrs, ipv6Addr)
	}

	// Get a list of all cluster members.
	clusterMembers, err := incusClient.GetClusterMembers()
	if err != nil {
		return err
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

		// Define the storage pool.
		incusClient = incusClient.UseTarget(member.ServerName)

		err = incusClient.CreateStoragePool(incusapi.StoragePoolsPost{
			Name:   config["pool_name"],
			Driver: "ceph",
		})
		if err != nil {
			return err
		}
	}

	// Finalize creation of the new Ceph storage pool.
	incusClient = incusClient.UseTarget("")

	err = incusClient.CreateStoragePool(incusapi.StoragePoolsPost{
		Name:   config["pool_name"],
		Driver: "ceph",
	})
	if err != nil {
		return err
	}

	return nil
}

// RefreshCephOCIImages refreshes the OCI image used by the Ceph containers. This is an
// inefficient operation, as Incus cannot query the OCI remote so the existing OCI image
// must be re-downloaded to determine if a newer version is available.
//
// Configuration fields:
//
//	oci_tag -- Optional; if specified, set the Ceph OCI image tag to the provided value.
//	           This is useful for performing major version updates, such as from v19 to
//	           v20, or pinning to an exact version.
func RefreshCephOCIImages(ctx context.Context, config map[string]string) error {
	incusClient, err := incus.ConnectIncusUnixWithContext(ctx, "", nil)
	if err != nil {
		return err
	}

	server, _, err := incusClient.GetServer()
	if err != nil {
		return err
	}

	incusClient = incusClient.UseProject(server.Config["user.ceph.project"])

	project, _, err := incusClient.GetProject(server.Config["user.ceph.project"])
	if err != nil {
		return err
	}

	cephContainers := cephServerNames

	if project.Config["user.ceph.osds"] != "" {
		osds := []osdInfo{}

		err := json.Unmarshal([]byte(project.Config["user.ceph.osds"]), &osds)
		if err != nil {
			return err
		}

		for _, osd := range osds {
			cephContainers = append(cephContainers, "ceph-osd-"+osd.Host)
		}
	}

	for _, containerName := range cephContainers {
		var imageAlias string

		if config["oci_tag"] != "" {
			imageAlias = "ceph/ceph:" + config["oci_tag"]
		} else {
			instance, _, err := incusClient.GetInstance(containerName)
			if err != nil {
				return err
			}

			imageAlias = instance.Config["image.id"]
		}

		err := refreshCephOCIImage(ctx, containerName, imageAlias)
		if err != nil {
			return err
		}

		// Sleep a few seconds to allow Ceph to stabilize its state.
		time.Sleep(10 * time.Second)
	}

	return nil
}

// RemoveOSD removes a Ceph OSD from the local server. Because Ceph requires a minimum of
// three OSDs, removal won't be allowed if there are currently three or fewer OSDs.
//
// WARNING -- This will forcefully remove the OSD from the Ceph cluster. Prior to removal,
// you should remove the OSD via Ceph's API and wait for data migration to complete.
func RemoveOSD(ctx context.Context) error {
	incusClient, err := incus.ConnectIncusUnixWithContext(ctx, "", nil)
	if err != nil {
		return err
	}

	server, _, err := incusClient.GetServer()
	if err != nil {
		return err
	}

	incusClient = incusClient.UseProject(server.Config["user.ceph.project"])

	project, etag, err := incusClient.GetProject(server.Config["user.ceph.project"])
	if err != nil {
		return err
	}

	//
	// Check that there is an OSD on this host.
	//

	_, _, err = incusClient.GetInstance("ceph-osd-" + server.Environment.ServerName)
	if err != nil {
		return errors.New("no Ceph OSD instance exists on Incus server " + server.Environment.ServerName)
	}

	//
	// Remove the OSD from the cluster-wide Ceph configuration.
	//

	osds := []osdInfo{} //nolint:prealloc

	if project.Config["user.ceph.osds"] != "" {
		err := json.Unmarshal([]byte(project.Config["user.ceph.osds"]), &osds)
		if err != nil {
			return err
		}
	}

	if !slices.ContainsFunc(osds, func(osd osdInfo) bool {
		return osd.Host == server.Environment.ServerName
	}) {
		return errors.New("no Ceph OSD is configured for Incus server " + server.Environment.ServerName)
	}

	if len(osds) <= 3 {
		return errors.New("a minimum of three OSDs are required by Ceph; refusing to remove OSD from Incus server " + server.Environment.ServerName)
	}

	osds = slices.DeleteFunc(osds, func(osd osdInfo) bool {
		return osd.Host == server.Environment.ServerName
	})

	contents, err := json.Marshal(osds)
	if err != nil {
		return err
	}

	project.Config["user.ceph.osds"] = string(contents)

	err = incusClient.UpdateProject(server.Config["user.ceph.project"], project.ProjectPut, etag)
	if err != nil {
		return err
	}

	//
	// Delete the OSD.
	//

	op, err := incusClient.UpdateInstanceState("ceph-osd-"+server.Environment.ServerName, incusapi.InstanceStatePut{
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

	op, err = incusClient.DeleteInstance("ceph-osd-" + server.Environment.ServerName)
	if err != nil {
		return err
	}

	err = op.Wait()
	if err != nil {
		return err
	}

	return nil
}

func deployCephContainer(ctx context.Context, incusTarget string, cephContainerName string, configScript string, osd *osdInfo) error {
	incusClient, err := incus.ConnectIncusUnixWithContext(ctx, "", nil)
	if err != nil {
		return err
	}

	server, _, err := incusClient.GetServer()
	if err != nil {
		return err
	}

	incusClient = incusClient.UseProject(server.Config["user.ceph.project"])
	incusClient = incusClient.UseTarget(incusTarget)

	project, _, err := incusClient.GetProject(server.Config["user.ceph.project"])
	if err != nil {
		return err
	}

	// Create storage volumes for /etc/ceph/ and /var/lib/ceph/.
	err = incusClient.CreateStoragePoolVolume("local", incusapi.StorageVolumesPost{
		Name: cephContainerName + "-etc",
		Type: "custom",
	})
	if err != nil {
		return err
	}

	err = incusClient.CreateStoragePoolVolume("local", incusapi.StorageVolumesPost{
		Name: cephContainerName + "-var",
		Type: "custom",
	})
	if err != nil {
		return err
	}

	// Download the OCI image.
	err = fetchOCIImage(ctx, cephDockerImage)
	if err != nil {
		return err
	}

	// Prepare the container's configuration and devices.
	config := map[string]string{
		"oci.entrypoint": "/sbin/init",
	}

	devices := map[string]map[string]string{
		"eth0": {
			"type":    "nic",
			"network": project.Config["user.ceph.network"],
		},
		"etc": {
			"type":      "disk",
			"pool":      "local",
			"source":    cephContainerName + "-etc",
			"path":      "/etc/ceph/",
			"dependent": "true",
		},
		"var": {
			"type":      "disk",
			"pool":      "local",
			"source":    cephContainerName + "-var",
			"path":      "/var/lib/ceph/",
			"dependent": "true",
		},
	}

	if configScript == "ceph-osd.sh" {
		config["cluster.evacuate"] = "stop"
		devices["ceph-1"] = map[string]string{
			"type":   "unix-block",
			"source": osd.DeviceID,
			"path":   "/dev/ceph-1",
			"uid":    "167",
			"gid":    "167",
		}
	}

	// Create and start the Ceph server.
	op, err := incusClient.CreateInstance(incusapi.InstancesPost{
		Name: cephContainerName,
		InstancePut: incusapi.InstancePut{
			Config:  config,
			Devices: devices,
		},
		Source: incusapi.InstanceSource{
			Type:  "image",
			Alias: "quay.io/" + cephDockerImage,
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

	var buf bytes.Buffer

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

		cephConfigFiles, err := getCephClusterConfigFiles(ctx, cephServerNames[0])
		if err != nil {
			return err
		}

		// Push configuration files.
		err = incusClient.CreateInstanceFile(cephContainerName, "/etc/ceph/ceph.conf", incus.InstanceFileArgs{
			Type:    "file",
			Content: bytes.NewReader(cephConfigFiles.Conf),
		})
		if err != nil {
			return err
		}

		err = incusClient.CreateInstanceFile(cephContainerName, "/etc/ceph/ceph.client.admin.keyring", incus.InstanceFileArgs{
			Type:    "file",
			Content: bytes.NewReader(cephConfigFiles.AdminKeyring),
		})
		if err != nil {
			return err
		}

		err = incusClient.CreateInstanceFile(cephContainerName, "/tmp/ceph.mon.keyring", incus.InstanceFileArgs{
			Type:    "file",
			Content: bytes.NewReader(cephConfigFiles.MonKeyring),
		})
		if err != nil {
			return err
		}
	case "ceph-osd.sh":
		templateVars = shTemplate{
			DEVICE_CLASS: osd.DeviceClass,
		}

		cephConfigFiles, err := getCephClusterConfigFiles(ctx, cephServerNames[0])
		if err != nil {
			return err
		}

		// Push configuration files.
		err = incusClient.CreateInstanceFile(cephContainerName, "/etc/ceph/ceph.conf", incus.InstanceFileArgs{
			Type:    "file",
			Content: bytes.NewReader(cephConfigFiles.Conf),
		})
		if err != nil {
			return err
		}

		err = incusClient.CreateInstanceFile(cephContainerName, "/etc/ceph/ceph.client.admin.keyring", incus.InstanceFileArgs{
			Type:    "file",
			Content: bytes.NewReader(cephConfigFiles.AdminKeyring),
		})
		if err != nil {
			return err
		}
	default:
		return errors.New("unrecognized configuration script: " + configScript)
	}

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
	op, err = incusClient.ExecInstance(cephContainerName, incusapi.InstanceExecPost{
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

func fetchOCIImage(ctx context.Context, imageAlias string) error {
	incusClient, err := incus.ConnectIncusUnixWithContext(ctx, "", nil)
	if err != nil {
		return err
	}

	server, _, err := incusClient.GetServer()
	if err != nil {
		return err
	}

	incusClient = incusClient.UseProject(server.Config["user.ceph.project"])

	// Check if this OCI image is already present locally.
	_, _, err = incusClient.GetImageAlias("quay.io/" + imageAlias)
	if err == nil {
		// If the OCI image already exists, there's nothing to do.
		return nil
	}

	op, err := incusClient.CreateImage(incusapi.ImagesPost{
		Source: &incusapi.ImagesPostSource{
			Type: "image",
			ImageSource: incusapi.ImageSource{
				Alias:    imageAlias,
				Server:   "https://quay.io",
				Protocol: "oci",
			},
		},
		Aliases: []incusapi.ImageAlias{
			{Name: "quay.io/" + imageAlias},
		},
	}, nil)
	if err != nil {
		return err
	}

	err = op.Wait()
	if err != nil {
		return err
	}

	return nil
}

func getCephClusterConfigFiles(ctx context.Context, instanceName string) (*clusterConfigFiles, error) {
	incusClient, err := incus.ConnectIncusUnixWithContext(ctx, "", nil)
	if err != nil {
		return nil, err
	}

	server, _, err := incusClient.GetServer()
	if err != nil {
		return nil, err
	}

	incusClient = incusClient.UseProject(server.Config["user.ceph.project"])

	// Grab required configuration files from the initial Ceph server.
	ret := &clusterConfigFiles{}

	reader, _, err := incusClient.GetInstanceFile(instanceName, "/etc/ceph/ceph.conf")
	if err != nil {
		return nil, err
	}

	ret.Conf, err = io.ReadAll(reader)
	if err != nil {
		return nil, err
	}

	reader, _, err = incusClient.GetInstanceFile(instanceName, "/etc/ceph/ceph.client.admin.keyring")
	if err != nil {
		return nil, err
	}

	ret.AdminKeyring, err = io.ReadAll(reader)
	if err != nil {
		return nil, err
	}

	reader, _, err = incusClient.GetInstanceFile(instanceName, "/var/lib/ceph/mon/ceph-"+instanceName+"/keyring")
	if err != nil {
		return nil, err
	}

	ret.MonKeyring, err = io.ReadAll(reader)
	if err != nil {
		return nil, err
	}

	return ret, nil
}

func getInstanceIPv6Addr(ctx context.Context, instanceName string) (string, error) {
	incusClient, err := incus.ConnectIncusUnixWithContext(ctx, "", nil)
	if err != nil {
		return "", err
	}

	server, _, err := incusClient.GetServer()
	if err != nil {
		return "", err
	}

	incusClient = incusClient.UseProject(server.Config["user.ceph.project"])

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

	server, _, err := incusClient.GetServer()
	if err != nil {
		return err
	}

	incusClient = incusClient.UseProject(server.Config["user.ceph.project"])

	var existingImageFingerprint string

	// Determine if the OCI image currently exists locally.
	images, err := incusClient.GetImages()
	if err != nil {
		return err
	}

	i := slices.IndexFunc(images, func(image incusapi.Image) bool {
		return slices.ContainsFunc(image.Aliases, func(alias incusapi.ImageAlias) bool {
			return alias.Name == "quay.io/"+imageAlias
		})
	})

	if i >= 0 {
		existingImageFingerprint = images[i].Fingerprint
	}

	// Get the fingerprint of the remote OCI image.
	ociRemote, err := incus.ConnectOCI("https://quay.io", nil)
	if err != nil {
		return err
	}

	alias, _, err := ociRemote.GetImageAlias(imageAlias)
	if err != nil {
		if strings.Contains(err.Error(), "manifest unknown") {
			return errors.New("OCI image 'quay.io/" + imageAlias + "' doesn't exist")
		}

		return err
	}

	remoteImageFingerprint := alias.Target

	// If the new OCI image fingerprint is different than it was previously, the container
	// will need to be refreshed.
	containerNeedsRefresh := existingImageFingerprint != remoteImageFingerprint

	// Check if the new OCI alias no longer matches the container's, for example a major version bump.
	if !containerNeedsRefresh {
		instance, _, err := incusClient.GetInstance(containerName)
		if err != nil {
			return err
		}

		containerNeedsRefresh = instance.Config["image.id"] != imageAlias
	}

	// Return early if the container is running the latest version of the OCI image.
	if !containerNeedsRefresh {
		return nil
	}

	// Fetch the remote OCI image if not present locally.
	if existingImageFingerprint != remoteImageFingerprint {
		// If we currently have an older version of the OCI image, delete it
		// before downloading the latest version.
		if existingImageFingerprint != "" {
			op, err := incusClient.DeleteImage(existingImageFingerprint)
			if err != nil {
				return err
			}

			err = op.Wait()
			if err != nil {
				return err
			}
		}

		// Download the remote OCI image.
		err = fetchOCIImage(ctx, imageAlias)
		if err != nil {
			return err
		}
	}

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

	// Rebuild the container's rootfs.
	op, err = incusClient.RebuildInstance(containerName, incusapi.InstanceRebuildPost{
		Source: incusapi.InstanceSource{
			Type:  "image",
			Alias: "quay.io/" + imageAlias,
		},
	})
	if err != nil {
		return err
	}

	err = op.Wait()
	if err != nil {
		return err
	}

	// Update the container's image.id property.
	instance, etag, err := incusClient.GetInstance(containerName)
	if err != nil {
		return err
	}

	instance.Config["image.id"] = imageAlias

	op, err = incusClient.UpdateInstance(containerName, instance.InstancePut, etag)
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
	if !strings.HasPrefix(containerName, "ceph-osd-") {
		for _, serviceName := range []string{"ceph-mon@" + containerName + ".service", "ceph-mgr@" + containerName + ".service", "ceph-mds@" + containerName + ".service", "ceph-rbd-mirror@rbd-mirror." + containerName + ".service"} {
			op, err := incusClient.ExecInstance(containerName, incusapi.InstanceExecPost{
				Command:     []string{"systemctl", "enable", "--now", serviceName},
				WaitForWS:   true,
				Interactive: false,
			}, &incus.InstanceExecArgs{
				Stdin:  nil,
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
		}
	} else {
		op, err := incusClient.ExecInstance(containerName, incusapi.InstanceExecPost{
			Command:     []string{"sh", "-ceux", `systemctl enable --now "ceph-osd@$(ls /var/lib/ceph/osd/ | cut -d '-' -f 2)"`},
			WaitForWS:   true,
			Interactive: false,
		}, &incus.InstanceExecArgs{
			Stdin:  nil,
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
	}

	return nil
}
