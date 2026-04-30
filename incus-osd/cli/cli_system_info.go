package cli

import (
	"fmt"
	"os"
	"slices"
	"strconv"
	"strings"

	incusapi "github.com/lxc/incus/v6/shared/api"
	cli "github.com/lxc/incus/v6/shared/cmd"
	"github.com/lxc/incus/v6/shared/units"
	"github.com/spf13/cobra"
	"go.yaml.in/yaml/v4"

	"github.com/lxc/incus-os/incus-osd/api"
)

// makeInfoCommand is a generic helper that handles the common boilerplate for info commands.
func makeInfoCommand[T any](c *cmdAdminOS, endpoint string, description string, render func(T) error) *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Use = cli.Usage("info")
	cmd.Short = description
	cmd.Long = cli.FormatSection("Description", description)

	if c.args.SupportsTarget {
		cmd.Flags().StringVar(&c.flagTarget, "target", "", "Cluster member name``")
	}

	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		exit, err := cli.CheckArgs(cmd, args, 0, 0)
		if exit {
			return err
		}

		apiURL := "/os/1.0/" + endpoint
		if c.flagTarget != "" {
			apiURL += "?target=" + c.flagTarget
		}

		resp, _, err := doQuery(c.args.DoHTTP, "", "GET", apiURL, nil, nil, "")
		if err != nil {
			return err
		}

		var data T

		err = resp.MetadataAsStruct(&data)
		if err != nil {
			return err
		}

		return render(data)
	}

	return cmd
}

// systemInfoCommand returns an info command that renders the endpoint as YAML.
func systemInfoCommand(c *cmdAdminOS, endpoint string, description string) *cobra.Command {
	return makeInfoCommand[any](c, endpoint, description, func(data any) error {
		out, err := yaml.Dump(data, yaml.V2)
		if err != nil {
			return err
		}

		_, _ = fmt.Printf("%s", out) //nolint:forbidigo

		return nil
	})
}

// systemInfoNetworkCommand returns an info command for the system/network endpoint.
func systemInfoNetworkCommand(c *cmdAdminOS, endpoint string, description string) *cobra.Command {
	return makeInfoCommand[api.SystemNetwork](c, endpoint, description, func(network api.SystemNetwork) error {
		names := make([]string, 0, len(network.State.Interfaces))
		for name := range network.State.Interfaces {
			names = append(names, name)
		}

		slices.Sort(names)

		rows := make([][]string, 0, len(names))
		for _, name := range names {
			iface := network.State.Interfaces[name]

			rows = append(rows, []string{
				name,
				iface.Type,
				iface.State,
				iface.Hwaddr,
				strconv.Itoa(iface.MTU),
				iface.Speed,
				strings.Join(iface.Addresses, "\n"),
				strings.Join(iface.Roles, "\n"),
			})
		}

		header := []string{"NAME", "TYPE", "STATE", "HWADDR", "MTU", "SPEED", "ADDRESSES", "ROLES"}

		return cli.RenderTable(os.Stdout, "table", header, rows, nil)
	})
}

// systemInfoResourcesCommand returns an info command for the system/resources endpoint.
func systemInfoResourcesCommand(c *cmdAdminOS, endpoint string, description string) *cobra.Command {
	return makeInfoCommand[incusapi.Resources](c, endpoint, description, func(resources incusapi.Resources) error {
		renderResourcesSystem(resources.System)

		// Load.
		fmt.Print("\nLoad:\n") //nolint:forbidigo

		if resources.Load.Processes > 0 {
			fmt.Printf("  Processes: %d\n", resources.Load.Processes)                                                                      //nolint:forbidigo
			fmt.Printf("  Average: %.2f %.2f %.2f\n", resources.Load.Average1Min, resources.Load.Average5Min, resources.Load.Average10Min) //nolint:forbidigo
		}

		// CPU.
		if len(resources.CPU.Sockets) == 1 {
			fmt.Print("\nCPU:\n")                                          //nolint:forbidigo
			fmt.Printf("  Architecture: %s\n", resources.CPU.Architecture) //nolint:forbidigo
			renderResourcesCPU(resources.CPU.Sockets[0], "  ")
		} else if len(resources.CPU.Sockets) > 1 {
			fmt.Print("CPUs:\n")                                           //nolint:forbidigo
			fmt.Printf("  Architecture: %s\n", resources.CPU.Architecture) //nolint:forbidigo

			for _, socket := range resources.CPU.Sockets {
				fmt.Printf("  Socket %d:\n", socket.Socket) //nolint:forbidigo
				renderResourcesCPU(socket, "    ")
			}
		}

		// Memory.
		fmt.Print("\nMemory:\n") //nolint:forbidigo

		if resources.Memory.HugepagesTotal > 0 {
			fmt.Print("  Hugepages:\n")                                                                                                        //nolint:forbidigo
			fmt.Printf("    Free: %v\n", units.GetByteSizeStringIEC(int64(resources.Memory.HugepagesTotal-resources.Memory.HugepagesUsed), 2)) //nolint:forbidigo
			fmt.Printf("    Used: %v\n", units.GetByteSizeStringIEC(int64(resources.Memory.HugepagesUsed), 2))                                 //nolint:forbidigo
			fmt.Printf("    Total: %v\n", units.GetByteSizeStringIEC(int64(resources.Memory.HugepagesTotal), 2))                               //nolint:forbidigo
		}

		if len(resources.Memory.Nodes) > 1 {
			fmt.Print("  NUMA nodes:\n") //nolint:forbidigo

			for _, node := range resources.Memory.Nodes {
				fmt.Printf("    Node %d:\n", node.NUMANode) //nolint:forbidigo

				if node.HugepagesTotal > 0 {
					fmt.Print("      Hugepages:" + "\n")                                                                              //nolint:forbidigo
					fmt.Printf("        Free: %v"+"\n", units.GetByteSizeStringIEC(int64(node.HugepagesTotal-node.HugepagesUsed), 2)) //nolint:forbidigo
					fmt.Printf("        Used: %v"+"\n", units.GetByteSizeStringIEC(int64(node.HugepagesUsed), 2))                     //nolint:forbidigo
					fmt.Printf("        Total: %v"+"\n", units.GetByteSizeStringIEC(int64(node.HugepagesTotal), 2))                   //nolint:forbidigo
				}

				fmt.Printf("      Free: %v"+"\n", units.GetByteSizeStringIEC(int64(node.Total-node.Used), 2)) //nolint:forbidigo
				fmt.Printf("      Used: %v"+"\n", units.GetByteSizeStringIEC(int64(node.Used), 2))            //nolint:forbidigo
				fmt.Printf("      Total: %v"+"\n", units.GetByteSizeStringIEC(int64(node.Total), 2))          //nolint:forbidigo
			}
		}

		fmt.Printf("  "+"Free: %v\n", units.GetByteSizeStringIEC(int64(resources.Memory.Total-resources.Memory.Used), 2)) //nolint:forbidigo
		fmt.Printf("  "+"Used: %v\n", units.GetByteSizeStringIEC(int64(resources.Memory.Used), 2))                        //nolint:forbidigo
		fmt.Printf("  "+"Total: %v\n", units.GetByteSizeStringIEC(int64(resources.Memory.Total), 2))                      //nolint:forbidigo

		// GPUs.
		if len(resources.GPU.Cards) == 1 {
			fmt.Print("\nGPU:\n") //nolint:forbidigo
			renderResourcesGPU(resources.GPU.Cards[0], "  ", true)
		} else if len(resources.GPU.Cards) > 1 {
			fmt.Print("\nGPUs:\n") //nolint:forbidigo

			for id, gpu := range resources.GPU.Cards {
				fmt.Printf("  Card %d:\n", id) //nolint:forbidigo
				renderResourcesGPU(gpu, "    ", true)
			}
		}

		// NICs.
		if len(resources.Network.Cards) == 1 {
			fmt.Print("\nNIC:\n") //nolint:forbidigo
			renderResourcesNIC(resources.Network.Cards[0], "  ", true)
		} else if len(resources.Network.Cards) > 1 {
			fmt.Print("\nNICs:\n") //nolint:forbidigo

			for id, nic := range resources.Network.Cards {
				fmt.Printf("  Card %d:\n", id) //nolint:forbidigo
				renderResourcesNIC(nic, "    ", true)
			}
		}

		// Storage.
		if len(resources.Storage.Disks) == 1 {
			fmt.Print("\nDisk:\n") //nolint:forbidigo
			renderResourcesDisk(resources.Storage.Disks[0], "  ", true)
		} else if len(resources.Storage.Disks) > 1 {
			fmt.Print("\nDisks:\n") //nolint:forbidigo

			for id, disk := range resources.Storage.Disks {
				fmt.Printf("  Disk %d:\n", id) //nolint:forbidigo
				renderResourcesDisk(disk, "    ", true)
			}
		}

		// USB.
		if len(resources.USB.Devices) == 1 {
			fmt.Print("\nUSB device:\n") //nolint:forbidigo
			renderResourcesUSB(resources.USB.Devices[0], "  ")
		} else if len(resources.USB.Devices) > 1 {
			fmt.Print("\nUSB devices:\n") //nolint:forbidigo

			for id, usb := range resources.USB.Devices {
				fmt.Printf("  Device %d:\n", id) //nolint:forbidigo
				renderResourcesUSB(usb, "    ")
			}
		}

		// PCI.
		if len(resources.PCI.Devices) == 1 {
			fmt.Print("\nPCI device:\n") //nolint:forbidigo
			renderResourcesPCI(resources.PCI.Devices[0], "  ")
		} else if len(resources.PCI.Devices) > 1 {
			fmt.Print("\nPCI devices:\n") //nolint:forbidigo

			for id, pci := range resources.PCI.Devices {
				fmt.Printf("  Device %d:\n", id) //nolint:forbidigo
				renderResourcesPCI(pci, "    ")
			}
		}

		// Serial.
		if len(resources.Serial.Devices) == 1 {
			fmt.Print("\nSerial device:\n") //nolint:forbidigo
			renderResourcesSerial(resources.Serial.Devices[0], "  ")
		} else if len(resources.Serial.Devices) > 1 {
			fmt.Print("\nSerial devices:\n") //nolint:forbidigo

			for id, serial := range resources.Serial.Devices {
				fmt.Printf("  Device %d:\n", id) //nolint:forbidigo
				renderResourcesSerial(serial, "    ")
			}
		}

		return nil
	})
}

func renderResourcesSystem(system incusapi.ResourcesSystem) {
	fmt.Print("System:\n") //nolint:forbidigo

	if system.UUID != "" {
		fmt.Printf("  "+"UUID: %v\n", system.UUID) //nolint:forbidigo
	}

	if system.Vendor != "" {
		fmt.Printf("  "+"Vendor: %v\n", system.Vendor) //nolint:forbidigo
	}

	if system.Product != "" {
		fmt.Printf("  "+"Product: %v\n", system.Product) //nolint:forbidigo
	}

	if system.Family != "" {
		fmt.Printf("  "+"Family: %v\n", system.Family) //nolint:forbidigo
	}

	if system.Version != "" {
		fmt.Printf("  "+"Version: %v\n", system.Version) //nolint:forbidigo
	}

	if system.Sku != "" {
		fmt.Printf("  "+"SKU: %v\n", system.Sku) //nolint:forbidigo
	}

	if system.Serial != "" {
		fmt.Printf("  "+"Serial: %v\n", system.Serial) //nolint:forbidigo
	}

	if system.Type != "" {
		fmt.Printf("  "+"Type: %s\n", system.Type) //nolint:forbidigo
	}

	// System: Chassis.
	if system.Chassis != nil {
		fmt.Print("  Chassis:\n") //nolint:forbidigo

		if system.Chassis.Vendor != "" {
			fmt.Printf("      Vendor: %s\n", system.Chassis.Vendor) //nolint:forbidigo
		}

		if system.Chassis.Type != "" {
			fmt.Printf("      Type: %s\n", system.Chassis.Type) //nolint:forbidigo
		}

		if system.Chassis.Version != "" {
			fmt.Printf("      Version: %s\n", system.Chassis.Version) //nolint:forbidigo
		}

		if system.Chassis.Serial != "" {
			fmt.Printf("      Serial: %s\n", system.Chassis.Serial) //nolint:forbidigo
		}
	}

	// System: Motherboard.
	if system.Motherboard != nil {
		fmt.Print("  Motherboard:\n") //nolint:forbidigo

		if system.Motherboard.Vendor != "" {
			fmt.Printf("      Vendor: %s\n", system.Motherboard.Vendor) //nolint:forbidigo
		}

		if system.Motherboard.Product != "" {
			fmt.Printf("      Product: %s\n", system.Motherboard.Product) //nolint:forbidigo
		}

		if system.Motherboard.Serial != "" {
			fmt.Printf("      Serial: %s\n", system.Motherboard.Serial) //nolint:forbidigo
		}

		if system.Motherboard.Version != "" {
			fmt.Printf("      Version: %s\n", system.Motherboard.Version) //nolint:forbidigo
		}
	}

	// System: Firmware.
	if system.Firmware != nil {
		fmt.Print("  Firmware:\n") //nolint:forbidigo

		if system.Firmware.Vendor != "" {
			fmt.Printf("      Vendor: %s\n", system.Firmware.Vendor) //nolint:forbidigo
		}

		if system.Firmware.Version != "" {
			fmt.Printf("      Version: %s\n", system.Firmware.Version) //nolint:forbidigo
		}

		if system.Firmware.Date != "" {
			fmt.Printf("      Date: %s\n", system.Firmware.Date) //nolint:forbidigo
		}
	}
}

func renderResourcesCPU(cpu incusapi.ResourcesCPUSocket, prefix string) {
	if cpu.Vendor != "" {
		fmt.Printf(prefix+"Vendor: %v\n", cpu.Vendor) //nolint:forbidigo
	}

	if cpu.Name != "" {
		fmt.Printf(prefix+"Name: %v\n", cpu.Name) //nolint:forbidigo
	}

	if cpu.Cache != nil {
		fmt.Print(prefix + "Caches:\n") //nolint:forbidigo

		for _, cache := range cpu.Cache {
			fmt.Printf(prefix+"  - Level %d (type: %s): %s\n", cache.Level, cache.Type, units.GetByteSizeStringIEC(int64(cache.Size), 0)) //nolint:forbidigo
		}
	}

	fmt.Print(prefix + "Cores:\n") //nolint:forbidigo

	for _, core := range cpu.Cores {
		fmt.Printf(prefix+"  - Core %d\n", core.Core)               //nolint:forbidigo
		fmt.Printf(prefix+"    Frequency: %vMhz\n", core.Frequency) //nolint:forbidigo
		fmt.Print(prefix + "    Threads:\n")                        //nolint:forbidigo

		for _, thread := range core.Threads {
			fmt.Printf(prefix+"      - %d (id: %d, online: %v, NUMA node: %v)\n", thread.Thread, thread.ID, thread.Online, thread.NUMANode) //nolint:forbidigo
		}
	}

	if cpu.Frequency > 0 {
		if cpu.FrequencyTurbo > 0 && cpu.FrequencyMinimum > 0 {
			fmt.Printf(prefix+"Frequency: %vMhz (min: %vMhz, max: %vMhz)\n", cpu.Frequency, cpu.FrequencyMinimum, cpu.FrequencyTurbo) //nolint:forbidigo
		} else {
			fmt.Printf(prefix+"Frequency: %vMhz\n", cpu.Frequency) //nolint:forbidigo
		}
	}
}

func renderResourcesGPU(gpu incusapi.ResourcesGPUCard, prefix string, initial bool) {
	if initial {
		fmt.Print(prefix) //nolint:forbidigo
	}

	fmt.Printf("NUMA node: %v\n", gpu.NUMANode) //nolint:forbidigo

	if gpu.Vendor != "" {
		fmt.Printf(prefix+"Vendor: %v (%v)\n", gpu.Vendor, gpu.VendorID) //nolint:forbidigo
	}

	if gpu.Product != "" {
		fmt.Printf(prefix+"Product: %v (%v)\n", gpu.Product, gpu.ProductID) //nolint:forbidigo
	}

	if gpu.PCIAddress != "" {
		fmt.Printf(prefix+"PCI address: %v\n", gpu.PCIAddress) //nolint:forbidigo
	}

	if gpu.Driver != "" {
		fmt.Printf(prefix+"Driver: %v (%v)\n", gpu.Driver, gpu.DriverVersion) //nolint:forbidigo
	}

	if gpu.DRM != nil {
		fmt.Print(prefix + "DRM:\n")                   //nolint:forbidigo
		fmt.Printf(prefix+"  "+"ID: %d\n", gpu.DRM.ID) //nolint:forbidigo

		if gpu.DRM.CardName != "" {
			fmt.Printf(prefix+"  "+"Card: %s (%s)\n", gpu.DRM.CardName, gpu.DRM.CardDevice) //nolint:forbidigo
		}

		if gpu.DRM.ControlName != "" {
			fmt.Printf(prefix+"  "+"Control: %s (%s)\n", gpu.DRM.ControlName, gpu.DRM.ControlDevice) //nolint:forbidigo
		}

		if gpu.DRM.RenderName != "" {
			fmt.Printf(prefix+"  "+"Render: %s (%s)\n", gpu.DRM.RenderName, gpu.DRM.RenderDevice) //nolint:forbidigo
		}
	}

	if gpu.Nvidia != nil {
		fmt.Print(prefix + "NVIDIA information:\n")                           //nolint:forbidigo
		fmt.Printf(prefix+"  "+"Architecture: %v\n", gpu.Nvidia.Architecture) //nolint:forbidigo
		fmt.Printf(prefix+"  "+"Brand: %v\n", gpu.Nvidia.Brand)               //nolint:forbidigo
		fmt.Printf(prefix+"  "+"Model: %v\n", gpu.Nvidia.Model)               //nolint:forbidigo
		fmt.Printf(prefix+"  "+"CUDA Version: %v\n", gpu.Nvidia.CUDAVersion)  //nolint:forbidigo
		fmt.Printf(prefix+"  "+"NVRM Version: %v\n", gpu.Nvidia.NVRMVersion)  //nolint:forbidigo
		fmt.Printf(prefix+"  "+"UUID: %v\n", gpu.Nvidia.UUID)                 //nolint:forbidigo
	}

	if gpu.SRIOV != nil {
		fmt.Print(prefix + "SR-IOV information:\n")                                 //nolint:forbidigo
		fmt.Printf(prefix+"  "+"Current number of VFs: %d\n", gpu.SRIOV.CurrentVFs) //nolint:forbidigo
		fmt.Printf(prefix+"  "+"Maximum number of VFs: %d\n", gpu.SRIOV.MaximumVFs) //nolint:forbidigo

		if len(gpu.SRIOV.VFs) > 0 {
			fmt.Printf(prefix+"  "+"VFs: %d\n", gpu.SRIOV.MaximumVFs) //nolint:forbidigo

			for _, vf := range gpu.SRIOV.VFs {
				fmt.Print(prefix + "  - ") //nolint:forbidigo
				renderResourcesGPU(vf, prefix+"    ", false)
			}
		}
	}

	if gpu.Mdev != nil {
		fmt.Print(prefix + "Mdev profiles:\n") //nolint:forbidigo

		keys := make([]string, 0, len(gpu.Mdev))
		for k := range gpu.Mdev {
			keys = append(keys, k)
		}

		slices.Sort(keys)

		for _, k := range keys {
			v := gpu.Mdev[k]

			fmt.Println(prefix + "  - " + fmt.Sprintf("%s (%s) (%d available)", k, v.Name, v.Available)) //nolint:forbidigo

			if v.Description != "" {
				for line := range strings.SplitSeq(v.Description, "\n") {
					fmt.Printf(prefix+"      %s\n", line) //nolint:forbidigo
				}
			}
		}
	}
}

func renderResourcesNIC(nic incusapi.ResourcesNetworkCard, prefix string, initial bool) {
	if initial {
		fmt.Print(prefix) //nolint:forbidigo
	}

	fmt.Printf("NUMA node: %v\n", nic.NUMANode) //nolint:forbidigo

	if nic.Vendor != "" {
		fmt.Printf(prefix+"Vendor: %v (%v)\n", nic.Vendor, nic.VendorID) //nolint:forbidigo
	}

	if nic.Product != "" {
		fmt.Printf(prefix+"Product: %v (%v)\n", nic.Product, nic.ProductID) //nolint:forbidigo
	}

	if nic.PCIAddress != "" {
		fmt.Printf(prefix+"PCI address: %v\n", nic.PCIAddress) //nolint:forbidigo
	}

	if nic.Driver != "" {
		fmt.Printf(prefix+"Driver: %v (%v)\n", nic.Driver, nic.DriverVersion) //nolint:forbidigo
	}

	if len(nic.Ports) > 0 {
		fmt.Print(prefix + "Ports:\n") //nolint:forbidigo

		for _, port := range nic.Ports {
			renderResourcesNICPort(port, prefix)
		}
	}

	if nic.SRIOV != nil {
		fmt.Print(prefix + "SR-IOV information:\n")                                 //nolint:forbidigo
		fmt.Printf(prefix+"  "+"Current number of VFs: %d\n", nic.SRIOV.CurrentVFs) //nolint:forbidigo
		fmt.Printf(prefix+"  "+"Maximum number of VFs: %d\n", nic.SRIOV.MaximumVFs) //nolint:forbidigo

		if len(nic.SRIOV.VFs) > 0 {
			fmt.Printf(prefix+"  "+"VFs: %d\n", nic.SRIOV.MaximumVFs) //nolint:forbidigo

			for _, vf := range nic.SRIOV.VFs {
				fmt.Print(prefix + "  - ") //nolint:forbidigo
				renderResourcesNIC(vf, prefix+"    ", false)
			}
		}
	}
}

func renderResourcesNICPort(port incusapi.ResourcesNetworkCardPort, prefix string) {
	fmt.Printf(prefix+"  "+"- Port %d (%s)\n", port.Port, port.Protocol) //nolint:forbidigo
	fmt.Printf(prefix+"    "+"ID: %s\n", port.ID)                        //nolint:forbidigo

	if port.Address != "" {
		fmt.Printf(prefix+"    "+"Address: %s\n", port.Address) //nolint:forbidigo
	}

	if port.SupportedModes != nil {
		fmt.Printf(prefix+"    "+"Supported modes: %s\n", strings.Join(port.SupportedModes, ", ")) //nolint:forbidigo
	}

	if port.SupportedPorts != nil {
		fmt.Printf(prefix+"    "+"Supported ports: %s\n", strings.Join(port.SupportedPorts, ", ")) //nolint:forbidigo
	}

	if port.PortType != "" {
		fmt.Printf(prefix+"    "+"Port type: %s\n", port.PortType) //nolint:forbidigo
	}

	if port.TransceiverType != "" {
		fmt.Printf(prefix+"    "+"Transceiver type: %s\n", port.TransceiverType) //nolint:forbidigo
	}

	fmt.Printf(prefix+"    "+"Auto negotiation: %v\n", port.AutoNegotiation) //nolint:forbidigo
	fmt.Printf(prefix+"    "+"Link detected: %v\n", port.LinkDetected)       //nolint:forbidigo

	if port.LinkSpeed > 0 {
		fmt.Printf(prefix+"    "+"Link speed: %dMbit/s (%s duplex)\n", port.LinkSpeed, port.LinkDuplex) //nolint:forbidigo
	}

	if port.Infiniband != nil {
		fmt.Print(prefix + "    " + "Infiniband:\n") //nolint:forbidigo

		if port.Infiniband.IsSMName != "" {
			fmt.Printf(prefix+"    "+"  "+"IsSM: %s (%s)\n", port.Infiniband.IsSMName, port.Infiniband.IsSMDevice) //nolint:forbidigo
		}

		if port.Infiniband.MADName != "" {
			fmt.Printf(prefix+"    "+"  "+"MAD: %s (%s)\n", port.Infiniband.MADName, port.Infiniband.MADDevice) //nolint:forbidigo
		}

		if port.Infiniband.VerbName != "" {
			fmt.Printf(prefix+"    "+"  "+"Verb: %s (%s)\n", port.Infiniband.VerbName, port.Infiniband.VerbDevice) //nolint:forbidigo
		}
	}
}

func renderResourcesDisk(disk incusapi.ResourcesStorageDisk, prefix string, initial bool) {
	if initial {
		fmt.Print(prefix) //nolint:forbidigo
	}

	fmt.Printf("NUMA node: %v\n", disk.NUMANode) //nolint:forbidigo

	fmt.Printf(prefix+"ID: %s\n", disk.ID)         //nolint:forbidigo
	fmt.Printf(prefix+"Device: %s\n", disk.Device) //nolint:forbidigo

	if disk.Model != "" {
		fmt.Printf(prefix+"Model: %s\n", disk.Model) //nolint:forbidigo
	}

	if disk.Type != "" {
		fmt.Printf(prefix+"Type: %s\n", disk.Type) //nolint:forbidigo
	}

	fmt.Printf(prefix+"Size: %s\n", units.GetByteSizeStringIEC(int64(disk.Size), 2)) //nolint:forbidigo

	if disk.WWN != "" {
		fmt.Printf(prefix+"WWN: %s\n", disk.WWN) //nolint:forbidigo
	}

	fmt.Printf(prefix+"Read-Only: %v\n", disk.ReadOnly)  //nolint:forbidigo
	fmt.Printf(prefix+"Removable: %v\n", disk.Removable) //nolint:forbidigo

	if len(disk.Partitions) != 0 {
		fmt.Print(prefix + "Partitions:\n") //nolint:forbidigo

		for _, partition := range disk.Partitions {
			fmt.Printf(prefix+"  "+"- Partition %d\n", partition.Partition)                              //nolint:forbidigo
			fmt.Printf(prefix+"    "+"ID: %s\n", partition.ID)                                           //nolint:forbidigo
			fmt.Printf(prefix+"    "+"Device: %s\n", partition.Device)                                   //nolint:forbidigo
			fmt.Printf(prefix+"    "+"Read-Only: %v\n", partition.ReadOnly)                              //nolint:forbidigo
			fmt.Printf(prefix+"    "+"Size: %s\n", units.GetByteSizeStringIEC(int64(partition.Size), 2)) //nolint:forbidigo
		}
	}
}

func renderResourcesUSB(usb incusapi.ResourcesUSBDevice, prefix string) {
	fmt.Printf(prefix+"Vendor: %v\n", usb.Vendor)                //nolint:forbidigo
	fmt.Printf(prefix+"Vendor ID: %v\n", usb.VendorID)           //nolint:forbidigo
	fmt.Printf(prefix+"Product: %v\n", usb.Product)              //nolint:forbidigo
	fmt.Printf(prefix+"Product ID: %v\n", usb.ProductID)         //nolint:forbidigo
	fmt.Printf(prefix+"Bus Address: %v\n", usb.BusAddress)       //nolint:forbidigo
	fmt.Printf(prefix+"Device Address: %v\n", usb.DeviceAddress) //nolint:forbidigo

	if len(usb.Serial) > 0 {
		fmt.Printf(prefix+"Serial Number: %v\n", usb.Serial) //nolint:forbidigo
	}
}

func renderResourcesPCI(pci incusapi.ResourcesPCIDevice, prefix string) {
	fmt.Printf(prefix+"Address: %v\n", pci.PCIAddress)     //nolint:forbidigo
	fmt.Printf(prefix+"Vendor: %v\n", pci.Vendor)          //nolint:forbidigo
	fmt.Printf(prefix+"Vendor ID: %v\n", pci.VendorID)     //nolint:forbidigo
	fmt.Printf(prefix+"Product: %v\n", pci.Product)        //nolint:forbidigo
	fmt.Printf(prefix+"Product ID: %v\n", pci.ProductID)   //nolint:forbidigo
	fmt.Printf(prefix+"NUMA node: %v\n", pci.NUMANode)     //nolint:forbidigo
	fmt.Printf(prefix+"IOMMU group: %v\n", pci.IOMMUGroup) //nolint:forbidigo
	fmt.Printf(prefix+"Driver: %v\n", pci.Driver)          //nolint:forbidigo
}

func renderResourcesSerial(serial incusapi.ResourcesSerialDevice, prefix string) {
	fmt.Printf(prefix+"Id: %v\n", serial.ID)                 //nolint:forbidigo
	fmt.Printf(prefix+"Device: %v\n", serial.Device)         //nolint:forbidigo
	fmt.Printf(prefix+"DeviceID: %v\n", serial.DeviceID)     //nolint:forbidigo
	fmt.Printf(prefix+"DevicePath: %v\n", serial.DevicePath) //nolint:forbidigo
	fmt.Printf(prefix+"Vendor: %v\n", serial.Vendor)         //nolint:forbidigo
	fmt.Printf(prefix+"Vendor ID: %v\n", serial.VendorID)    //nolint:forbidigo
	fmt.Printf(prefix+"Product: %v\n", serial.Product)       //nolint:forbidigo
	fmt.Printf(prefix+"Product ID: %v\n", serial.ProductID)  //nolint:forbidigo
	fmt.Printf(prefix+"Driver: %v\n", serial.Driver)         //nolint:forbidigo
}

// systemInfoStorageCommand returns an info command for the system/storage endpoint.
func systemInfoStorageCommand(c *cmdAdminOS, endpoint string, description string) *cobra.Command {
	return makeInfoCommand[api.SystemStorage](c, endpoint, description, func(storage api.SystemStorage) error {
		// Drives table.
		fmt.Println("Drives:") //nolint:forbidigo

		driveRows := make([][]string, 0, len(storage.State.Drives))
		for _, drive := range storage.State.Drives {
			boot := ""
			if drive.Boot {
				boot = "yes"
			}

			encrypted := ""
			if drive.Encrypted {
				encrypted = "yes"
			}

			driveRows = append(driveRows, []string{
				drive.ID,
				drive.ModelName,
				drive.SerialNumber,
				drive.Bus,
				units.GetByteSizeStringIEC(int64(drive.CapacityInBytes), 2),
				boot,
				encrypted,
				drive.MemberPool,
			})
		}

		driveHeader := []string{"ID", "MODEL", "SERIAL", "BUS", "CAPACITY", "BOOT", "ENCRYPTED", "MEMBER POOL"}

		err := cli.RenderTable(os.Stdout, "table", driveHeader, driveRows, nil)
		if err != nil {
			return err
		}

		// Pools table.
		if len(storage.State.Pools) > 0 {
			fmt.Println("\nPools:") //nolint:forbidigo

			poolRows := make([][]string, 0, len(storage.State.Pools))
			for _, pool := range storage.State.Pools {
				poolRows = append(poolRows, []string{
					pool.Name,
					pool.Type,
					pool.State,
					pool.EncryptionKeyStatus,
					units.GetByteSizeStringIEC(int64(pool.RawPoolSizeInBytes), 2),
					units.GetByteSizeStringIEC(int64(pool.UsablePoolSizeInBytes), 2),
					units.GetByteSizeStringIEC(int64(pool.PoolAllocatedSpaceInBytes), 2),
					strings.Join(pool.Devices, "\n"),
				})
			}

			poolHeader := []string{"NAME", "TYPE", "STATE", "ENCRYPTION KEY", "RAW SIZE", "USABLE SIZE", "ALLOCATED", "DEVICES"}

			return cli.RenderTable(os.Stdout, "table", poolHeader, poolRows, nil)
		}

		return nil
	})
}
