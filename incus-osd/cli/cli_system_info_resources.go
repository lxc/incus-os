package cli

import (
	"fmt"
	"slices"
	"strings"

	incusapi "github.com/lxc/incus/v7/shared/api"
	"github.com/lxc/incus/v7/shared/units"
	"github.com/spf13/cobra"
)

// renderResourcesInfo decodes a system/resources response and renders it with formatted sections.
func renderResourcesInfo(resp *incusapi.Response) error {
	var resources incusapi.Resources

	err := resp.MetadataAsStruct(&resources)
	if err != nil {
		return err
	}

	renderResourcesSystem(resources.System)

	// Load.
	_, _ = fmt.Print("\nLoad:\n") //nolint:forbidigo

	if resources.Load.Processes > 0 {
		_, _ = fmt.Printf("  Processes: %d\n", resources.Load.Processes)                                                                      //nolint:forbidigo
		_, _ = fmt.Printf("  Average: %.2f %.2f %.2f\n", resources.Load.Average1Min, resources.Load.Average5Min, resources.Load.Average10Min) //nolint:forbidigo
	}

	// CPU.
	if len(resources.CPU.Sockets) == 1 {
		_, _ = fmt.Print("\nCPU:\n")                                          //nolint:forbidigo
		_, _ = fmt.Printf("  Architecture: %s\n", resources.CPU.Architecture) //nolint:forbidigo
		renderResourcesCPU(resources.CPU.Sockets[0], "  ")
	} else if len(resources.CPU.Sockets) > 1 {
		_, _ = fmt.Print("CPUs:\n")                                           //nolint:forbidigo
		_, _ = fmt.Printf("  Architecture: %s\n", resources.CPU.Architecture) //nolint:forbidigo

		for _, socket := range resources.CPU.Sockets {
			_, _ = fmt.Printf("  Socket %d:\n", socket.Socket) //nolint:forbidigo
			renderResourcesCPU(socket, "    ")
		}
	}

	// Memory.
	_, _ = fmt.Print("\nMemory:\n") //nolint:forbidigo

	if resources.Memory.HugepagesTotal > 0 {
		_, _ = fmt.Print("  Hugepages:\n")                                                                                                        //nolint:forbidigo
		_, _ = fmt.Printf("    Free: %v\n", units.GetByteSizeStringIEC(int64(resources.Memory.HugepagesTotal-resources.Memory.HugepagesUsed), 2)) //nolint:forbidigo,gosec
		_, _ = fmt.Printf("    Used: %v\n", units.GetByteSizeStringIEC(int64(resources.Memory.HugepagesUsed), 2))                                 //nolint:forbidigo,gosec
		_, _ = fmt.Printf("    Total: %v\n", units.GetByteSizeStringIEC(int64(resources.Memory.HugepagesTotal), 2))                               //nolint:forbidigo,gosec
	}

	if len(resources.Memory.Nodes) > 1 {
		_, _ = fmt.Print("  NUMA nodes:\n") //nolint:forbidigo

		for _, node := range resources.Memory.Nodes {
			_, _ = fmt.Printf("    Node %d:\n", node.NUMANode) //nolint:forbidigo

			if node.HugepagesTotal > 0 {
				_, _ = fmt.Print("      Hugepages:" + "\n")                                                                              //nolint:forbidigo
				_, _ = fmt.Printf("        Free: %v"+"\n", units.GetByteSizeStringIEC(int64(node.HugepagesTotal-node.HugepagesUsed), 2)) //nolint:forbidigo,gosec
				_, _ = fmt.Printf("        Used: %v"+"\n", units.GetByteSizeStringIEC(int64(node.HugepagesUsed), 2))                     //nolint:forbidigo,gosec
				_, _ = fmt.Printf("        Total: %v"+"\n", units.GetByteSizeStringIEC(int64(node.HugepagesTotal), 2))                   //nolint:forbidigo,gosec
			}

			_, _ = fmt.Printf("      Free: %v"+"\n", units.GetByteSizeStringIEC(int64(node.Total-node.Used), 2)) //nolint:forbidigo,gosec
			_, _ = fmt.Printf("      Used: %v"+"\n", units.GetByteSizeStringIEC(int64(node.Used), 2))            //nolint:forbidigo,gosec
			_, _ = fmt.Printf("      Total: %v"+"\n", units.GetByteSizeStringIEC(int64(node.Total), 2))          //nolint:forbidigo,gosec
		}
	}

	_, _ = fmt.Printf("  "+"Free: %v\n", units.GetByteSizeStringIEC(int64(resources.Memory.Total-resources.Memory.Used), 2)) //nolint:forbidigo,gosec
	_, _ = fmt.Printf("  "+"Used: %v\n", units.GetByteSizeStringIEC(int64(resources.Memory.Used), 2))                        //nolint:forbidigo,gosec
	_, _ = fmt.Printf("  "+"Total: %v\n", units.GetByteSizeStringIEC(int64(resources.Memory.Total), 2))                      //nolint:forbidigo,gosec

	// GPUs.
	if len(resources.GPU.Cards) == 1 {
		_, _ = fmt.Print("\nGPU:\n") //nolint:forbidigo

		renderResourcesGPU(resources.GPU.Cards[0], "  ", true)
	} else if len(resources.GPU.Cards) > 1 {
		_, _ = fmt.Print("\nGPUs:\n") //nolint:forbidigo

		for id, gpu := range resources.GPU.Cards {
			_, _ = fmt.Printf("  Card %d:\n", id) //nolint:forbidigo

			renderResourcesGPU(gpu, "    ", true)
		}
	}

	// NICs.
	if len(resources.Network.Cards) == 1 {
		_, _ = fmt.Print("\nNIC:\n") //nolint:forbidigo

		renderResourcesNIC(resources.Network.Cards[0], "  ", true)
	} else if len(resources.Network.Cards) > 1 {
		_, _ = fmt.Print("\nNICs:\n") //nolint:forbidigo

		for id, nic := range resources.Network.Cards {
			_, _ = fmt.Printf("  Card %d:\n", id) //nolint:forbidigo

			renderResourcesNIC(nic, "    ", true)
		}
	}

	// Storage.
	if len(resources.Storage.Disks) == 1 {
		_, _ = fmt.Print("\nDisk:\n") //nolint:forbidigo

		renderResourcesDisk(resources.Storage.Disks[0], "  ", true)
	} else if len(resources.Storage.Disks) > 1 {
		_, _ = fmt.Print("\nDisks:\n") //nolint:forbidigo

		for id, disk := range resources.Storage.Disks {
			_, _ = fmt.Printf("  Disk %d:\n", id) //nolint:forbidigo

			renderResourcesDisk(disk, "    ", true)
		}
	}

	// USB.
	if len(resources.USB.Devices) == 1 {
		_, _ = fmt.Print("\nUSB device:\n") //nolint:forbidigo

		renderResourcesUSB(resources.USB.Devices[0], "  ")
	} else if len(resources.USB.Devices) > 1 {
		_, _ = fmt.Print("\nUSB devices:\n") //nolint:forbidigo

		for id, usb := range resources.USB.Devices {
			_, _ = fmt.Printf("  Device %d:\n", id) //nolint:forbidigo

			renderResourcesUSB(usb, "    ")
		}
	}

	// PCI.
	if len(resources.PCI.Devices) == 1 {
		_, _ = fmt.Print("\nPCI device:\n") //nolint:forbidigo

		renderResourcesPCI(resources.PCI.Devices[0], "  ")
	} else if len(resources.PCI.Devices) > 1 {
		_, _ = fmt.Print("\nPCI devices:\n") //nolint:forbidigo

		for id, pci := range resources.PCI.Devices {
			_, _ = fmt.Printf("  Device %d:\n", id) //nolint:forbidigo

			renderResourcesPCI(pci, "    ")
		}
	}

	// Serial.
	if len(resources.Serial.Devices) == 1 {
		_, _ = fmt.Print("\nSerial device:\n") //nolint:forbidigo

		renderResourcesSerial(resources.Serial.Devices[0], "  ")
	} else if len(resources.Serial.Devices) > 1 {
		_, _ = fmt.Print("\nSerial devices:\n") //nolint:forbidigo

		for id, serial := range resources.Serial.Devices {
			_, _ = fmt.Printf("  Device %d:\n", id) //nolint:forbidigo

			renderResourcesSerial(serial, "    ")
		}
	}

	return nil
}

func renderResourcesSystem(system incusapi.ResourcesSystem) {
	_, _ = fmt.Print("System:\n") //nolint:forbidigo

	if system.UUID != "" {
		_, _ = fmt.Printf("  "+"UUID: %v\n", system.UUID) //nolint:forbidigo
	}

	if system.Vendor != "" {
		_, _ = fmt.Printf("  "+"Vendor: %v\n", system.Vendor) //nolint:forbidigo
	}

	if system.Product != "" {
		_, _ = fmt.Printf("  "+"Product: %v\n", system.Product) //nolint:forbidigo
	}

	if system.Family != "" {
		_, _ = fmt.Printf("  "+"Family: %v\n", system.Family) //nolint:forbidigo
	}

	if system.Version != "" {
		_, _ = fmt.Printf("  "+"Version: %v\n", system.Version) //nolint:forbidigo
	}

	if system.Sku != "" {
		_, _ = fmt.Printf("  "+"SKU: %v\n", system.Sku) //nolint:forbidigo
	}

	if system.Serial != "" {
		_, _ = fmt.Printf("  "+"Serial: %v\n", system.Serial) //nolint:forbidigo
	}

	if system.Type != "" {
		_, _ = fmt.Printf("  "+"Type: %s\n", system.Type) //nolint:forbidigo
	}

	// System: Chassis.
	if system.Chassis != nil {
		_, _ = fmt.Print("  Chassis:\n") //nolint:forbidigo

		if system.Chassis.Vendor != "" {
			_, _ = fmt.Printf("      Vendor: %s\n", system.Chassis.Vendor) //nolint:forbidigo
		}

		if system.Chassis.Type != "" {
			_, _ = fmt.Printf("      Type: %s\n", system.Chassis.Type) //nolint:forbidigo
		}

		if system.Chassis.Version != "" {
			_, _ = fmt.Printf("      Version: %s\n", system.Chassis.Version) //nolint:forbidigo
		}

		if system.Chassis.Serial != "" {
			_, _ = fmt.Printf("      Serial: %s\n", system.Chassis.Serial) //nolint:forbidigo
		}
	}

	// System: Motherboard.
	if system.Motherboard != nil {
		_, _ = fmt.Print("  Motherboard:\n") //nolint:forbidigo

		if system.Motherboard.Vendor != "" {
			_, _ = fmt.Printf("      Vendor: %s\n", system.Motherboard.Vendor) //nolint:forbidigo
		}

		if system.Motherboard.Product != "" {
			_, _ = fmt.Printf("      Product: %s\n", system.Motherboard.Product) //nolint:forbidigo
		}

		if system.Motherboard.Serial != "" {
			_, _ = fmt.Printf("      Serial: %s\n", system.Motherboard.Serial) //nolint:forbidigo
		}

		if system.Motherboard.Version != "" {
			_, _ = fmt.Printf("      Version: %s\n", system.Motherboard.Version) //nolint:forbidigo
		}
	}

	// System: Firmware.
	if system.Firmware != nil {
		_, _ = fmt.Print("  Firmware:\n") //nolint:forbidigo

		if system.Firmware.Vendor != "" {
			_, _ = fmt.Printf("      Vendor: %s\n", system.Firmware.Vendor) //nolint:forbidigo
		}

		if system.Firmware.Version != "" {
			_, _ = fmt.Printf("      Version: %s\n", system.Firmware.Version) //nolint:forbidigo
		}

		if system.Firmware.Date != "" {
			_, _ = fmt.Printf("      Date: %s\n", system.Firmware.Date) //nolint:forbidigo
		}
	}
}

func renderResourcesCPU(cpu incusapi.ResourcesCPUSocket, prefix string) {
	if cpu.Vendor != "" {
		_, _ = fmt.Printf(prefix+"Vendor: %v\n", cpu.Vendor) //nolint:forbidigo
	}

	if cpu.Name != "" {
		_, _ = fmt.Printf(prefix+"Name: %v\n", cpu.Name) //nolint:forbidigo
	}

	if cpu.Cache != nil {
		_, _ = fmt.Print(prefix + "Caches:\n") //nolint:forbidigo

		for _, cache := range cpu.Cache {
			_, _ = fmt.Printf(prefix+"  - Level %d (type: %s): %s\n", cache.Level, cache.Type, units.GetByteSizeStringIEC(int64(cache.Size), 0)) //nolint:forbidigo,gosec
		}
	}

	_, _ = fmt.Print(prefix + "Cores:\n") //nolint:forbidigo

	for _, core := range cpu.Cores {
		_, _ = fmt.Printf(prefix+"  - Core %d\n", core.Core)               //nolint:forbidigo
		_, _ = fmt.Printf(prefix+"    Frequency: %vMhz\n", core.Frequency) //nolint:forbidigo
		_, _ = fmt.Print(prefix + "    Threads:\n")                        //nolint:forbidigo

		for _, thread := range core.Threads {
			_, _ = fmt.Printf(prefix+"      - %d (id: %d, online: %v, NUMA node: %v)\n", thread.Thread, thread.ID, thread.Online, thread.NUMANode) //nolint:forbidigo
		}
	}

	if cpu.Frequency > 0 {
		if cpu.FrequencyTurbo > 0 && cpu.FrequencyMinimum > 0 {
			_, _ = fmt.Printf(prefix+"Frequency: %vMhz (min: %vMhz, max: %vMhz)\n", cpu.Frequency, cpu.FrequencyMinimum, cpu.FrequencyTurbo) //nolint:forbidigo
		} else {
			_, _ = fmt.Printf(prefix+"Frequency: %vMhz\n", cpu.Frequency) //nolint:forbidigo
		}
	}
}

func renderResourcesGPU(gpu incusapi.ResourcesGPUCard, prefix string, initial bool) {
	if initial {
		_, _ = fmt.Print(prefix) //nolint:forbidigo
	}

	_, _ = fmt.Printf("NUMA node: %v\n", gpu.NUMANode) //nolint:forbidigo

	if gpu.Vendor != "" {
		_, _ = fmt.Printf(prefix+"Vendor: %v (%v)\n", gpu.Vendor, gpu.VendorID) //nolint:forbidigo
	}

	if gpu.Product != "" {
		_, _ = fmt.Printf(prefix+"Product: %v (%v)\n", gpu.Product, gpu.ProductID) //nolint:forbidigo
	}

	if gpu.PCIAddress != "" {
		_, _ = fmt.Printf(prefix+"PCI address: %v\n", gpu.PCIAddress) //nolint:forbidigo
	}

	if gpu.Driver != "" {
		_, _ = fmt.Printf(prefix+"Driver: %v (%v)\n", gpu.Driver, gpu.DriverVersion) //nolint:forbidigo
	}

	if gpu.DRM != nil {
		_, _ = fmt.Print(prefix + "DRM:\n")                   //nolint:forbidigo
		_, _ = fmt.Printf(prefix+"  "+"ID: %d\n", gpu.DRM.ID) //nolint:forbidigo

		if gpu.DRM.CardName != "" {
			_, _ = fmt.Printf(prefix+"  "+"Card: %s (%s)\n", gpu.DRM.CardName, gpu.DRM.CardDevice) //nolint:forbidigo
		}

		if gpu.DRM.ControlName != "" {
			_, _ = fmt.Printf(prefix+"  "+"Control: %s (%s)\n", gpu.DRM.ControlName, gpu.DRM.ControlDevice) //nolint:forbidigo
		}

		if gpu.DRM.RenderName != "" {
			_, _ = fmt.Printf(prefix+"  "+"Render: %s (%s)\n", gpu.DRM.RenderName, gpu.DRM.RenderDevice) //nolint:forbidigo
		}
	}

	if gpu.Nvidia != nil {
		_, _ = fmt.Print(prefix + "NVIDIA information:\n")                           //nolint:forbidigo
		_, _ = fmt.Printf(prefix+"  "+"Architecture: %v\n", gpu.Nvidia.Architecture) //nolint:forbidigo
		_, _ = fmt.Printf(prefix+"  "+"Brand: %v\n", gpu.Nvidia.Brand)               //nolint:forbidigo
		_, _ = fmt.Printf(prefix+"  "+"Model: %v\n", gpu.Nvidia.Model)               //nolint:forbidigo
		_, _ = fmt.Printf(prefix+"  "+"CUDA Version: %v\n", gpu.Nvidia.CUDAVersion)  //nolint:forbidigo
		_, _ = fmt.Printf(prefix+"  "+"NVRM Version: %v\n", gpu.Nvidia.NVRMVersion)  //nolint:forbidigo
		_, _ = fmt.Printf(prefix+"  "+"UUID: %v\n", gpu.Nvidia.UUID)                 //nolint:forbidigo
	}

	if gpu.SRIOV != nil {
		_, _ = fmt.Print(prefix + "SR-IOV information:\n")                                 //nolint:forbidigo
		_, _ = fmt.Printf(prefix+"  "+"Current number of VFs: %d\n", gpu.SRIOV.CurrentVFs) //nolint:forbidigo
		_, _ = fmt.Printf(prefix+"  "+"Maximum number of VFs: %d\n", gpu.SRIOV.MaximumVFs) //nolint:forbidigo

		if len(gpu.SRIOV.VFs) > 0 {
			_, _ = fmt.Printf(prefix+"  "+"VFs: %d\n", gpu.SRIOV.MaximumVFs) //nolint:forbidigo

			for _, vf := range gpu.SRIOV.VFs {
				_, _ = fmt.Print(prefix + "  - ") //nolint:forbidigo
				renderResourcesGPU(vf, prefix+"    ", false)
			}
		}
	}

	if gpu.Mdev != nil {
		_, _ = fmt.Print(prefix + "Mdev profiles:\n") //nolint:forbidigo

		keys := make([]string, 0, len(gpu.Mdev))
		for k := range gpu.Mdev {
			keys = append(keys, k)
		}

		slices.Sort(keys)

		for _, k := range keys {
			v := gpu.Mdev[k]

			_, _ = fmt.Println(prefix + "  - " + fmt.Sprintf("%s (%s) (%d available)", k, v.Name, v.Available)) //nolint:forbidigo

			if v.Description != "" {
				for line := range strings.SplitSeq(v.Description, "\n") {
					_, _ = fmt.Printf(prefix+"      %s\n", line) //nolint:forbidigo
				}
			}
		}
	}
}

func renderResourcesNIC(nic incusapi.ResourcesNetworkCard, prefix string, initial bool) {
	if initial {
		_, _ = fmt.Print(prefix) //nolint:forbidigo
	}

	_, _ = fmt.Printf("NUMA node: %v\n", nic.NUMANode) //nolint:forbidigo

	if nic.Vendor != "" {
		_, _ = fmt.Printf(prefix+"Vendor: %v (%v)\n", nic.Vendor, nic.VendorID) //nolint:forbidigo
	}

	if nic.Product != "" {
		_, _ = fmt.Printf(prefix+"Product: %v (%v)\n", nic.Product, nic.ProductID) //nolint:forbidigo
	}

	if nic.PCIAddress != "" {
		_, _ = fmt.Printf(prefix+"PCI address: %v\n", nic.PCIAddress) //nolint:forbidigo
	}

	if nic.Driver != "" {
		_, _ = fmt.Printf(prefix+"Driver: %v (%v)\n", nic.Driver, nic.DriverVersion) //nolint:forbidigo
	}

	if len(nic.Ports) > 0 {
		_, _ = fmt.Print(prefix + "Ports:\n") //nolint:forbidigo

		for _, port := range nic.Ports {
			renderResourcesNICPort(port, prefix)
		}
	}

	if nic.SRIOV != nil {
		_, _ = fmt.Print(prefix + "SR-IOV information:\n")                                 //nolint:forbidigo
		_, _ = fmt.Printf(prefix+"  "+"Current number of VFs: %d\n", nic.SRIOV.CurrentVFs) //nolint:forbidigo
		_, _ = fmt.Printf(prefix+"  "+"Maximum number of VFs: %d\n", nic.SRIOV.MaximumVFs) //nolint:forbidigo

		if len(nic.SRIOV.VFs) > 0 {
			_, _ = fmt.Printf(prefix+"  "+"VFs: %d\n", nic.SRIOV.MaximumVFs) //nolint:forbidigo

			for _, vf := range nic.SRIOV.VFs {
				_, _ = fmt.Print(prefix + "  - ") //nolint:forbidigo
				renderResourcesNIC(vf, prefix+"    ", false)
			}
		}
	}
}

func renderResourcesNICPort(port incusapi.ResourcesNetworkCardPort, prefix string) {
	_, _ = fmt.Printf(prefix+"  "+"- Port %d (%s)\n", port.Port, port.Protocol) //nolint:forbidigo
	_, _ = fmt.Printf(prefix+"    "+"ID: %s\n", port.ID)                        //nolint:forbidigo

	if port.Address != "" {
		_, _ = fmt.Printf(prefix+"    "+"Address: %s\n", port.Address) //nolint:forbidigo
	}

	if port.SupportedModes != nil {
		_, _ = fmt.Printf(prefix+"    "+"Supported modes: %s\n", strings.Join(port.SupportedModes, ", ")) //nolint:forbidigo
	}

	if port.SupportedPorts != nil {
		_, _ = fmt.Printf(prefix+"    "+"Supported ports: %s\n", strings.Join(port.SupportedPorts, ", ")) //nolint:forbidigo
	}

	if port.PortType != "" {
		_, _ = fmt.Printf(prefix+"    "+"Port type: %s\n", port.PortType) //nolint:forbidigo
	}

	if port.TransceiverType != "" {
		_, _ = fmt.Printf(prefix+"    "+"Transceiver type: %s\n", port.TransceiverType) //nolint:forbidigo
	}

	_, _ = fmt.Printf(prefix+"    "+"Auto negotiation: %v\n", port.AutoNegotiation) //nolint:forbidigo
	_, _ = fmt.Printf(prefix+"    "+"Link detected: %v\n", port.LinkDetected)       //nolint:forbidigo

	if port.LinkSpeed > 0 {
		_, _ = fmt.Printf(prefix+"    "+"Link speed: %dMbit/s (%s duplex)\n", port.LinkSpeed, port.LinkDuplex) //nolint:forbidigo
	}

	if port.Infiniband != nil {
		_, _ = fmt.Print(prefix + "    " + "Infiniband:\n") //nolint:forbidigo

		if port.Infiniband.IsSMName != "" {
			_, _ = fmt.Printf(prefix+"    "+"  "+"IsSM: %s (%s)\n", port.Infiniband.IsSMName, port.Infiniband.IsSMDevice) //nolint:forbidigo
		}

		if port.Infiniband.MADName != "" {
			_, _ = fmt.Printf(prefix+"    "+"  "+"MAD: %s (%s)\n", port.Infiniband.MADName, port.Infiniband.MADDevice) //nolint:forbidigo
		}

		if port.Infiniband.VerbName != "" {
			_, _ = fmt.Printf(prefix+"    "+"  "+"Verb: %s (%s)\n", port.Infiniband.VerbName, port.Infiniband.VerbDevice) //nolint:forbidigo
		}
	}
}

func renderResourcesDisk(disk incusapi.ResourcesStorageDisk, prefix string, initial bool) {
	if initial {
		_, _ = fmt.Print(prefix) //nolint:forbidigo
	}

	_, _ = fmt.Printf("NUMA node: %v\n", disk.NUMANode) //nolint:forbidigo

	_, _ = fmt.Printf(prefix+"ID: %s\n", disk.ID)         //nolint:forbidigo
	_, _ = fmt.Printf(prefix+"Device: %s\n", disk.Device) //nolint:forbidigo

	if disk.Model != "" {
		_, _ = fmt.Printf(prefix+"Model: %s\n", disk.Model) //nolint:forbidigo
	}

	if disk.Type != "" {
		_, _ = fmt.Printf(prefix+"Type: %s\n", disk.Type) //nolint:forbidigo
	}

	_, _ = fmt.Printf(prefix+"Size: %s\n", units.GetByteSizeStringIEC(int64(disk.Size), 2)) //nolint:forbidigo,gosec

	if disk.WWN != "" {
		_, _ = fmt.Printf(prefix+"WWN: %s\n", disk.WWN) //nolint:forbidigo
	}

	_, _ = fmt.Printf(prefix+"Read-Only: %v\n", disk.ReadOnly)  //nolint:forbidigo
	_, _ = fmt.Printf(prefix+"Removable: %v\n", disk.Removable) //nolint:forbidigo

	if len(disk.Partitions) != 0 {
		_, _ = fmt.Print(prefix + "Partitions:\n") //nolint:forbidigo

		for _, partition := range disk.Partitions {
			_, _ = fmt.Printf(prefix+"  "+"- Partition %d\n", partition.Partition)                              //nolint:forbidigo
			_, _ = fmt.Printf(prefix+"    "+"ID: %s\n", partition.ID)                                           //nolint:forbidigo
			_, _ = fmt.Printf(prefix+"    "+"Device: %s\n", partition.Device)                                   //nolint:forbidigo
			_, _ = fmt.Printf(prefix+"    "+"Read-Only: %v\n", partition.ReadOnly)                              //nolint:forbidigo
			_, _ = fmt.Printf(prefix+"    "+"Size: %s\n", units.GetByteSizeStringIEC(int64(partition.Size), 2)) //nolint:forbidigo,gosec
		}
	}
}

func renderResourcesUSB(usb incusapi.ResourcesUSBDevice, prefix string) {
	_, _ = fmt.Printf(prefix+"Vendor: %v\n", usb.Vendor)                //nolint:forbidigo
	_, _ = fmt.Printf(prefix+"Vendor ID: %v\n", usb.VendorID)           //nolint:forbidigo
	_, _ = fmt.Printf(prefix+"Product: %v\n", usb.Product)              //nolint:forbidigo
	_, _ = fmt.Printf(prefix+"Product ID: %v\n", usb.ProductID)         //nolint:forbidigo
	_, _ = fmt.Printf(prefix+"Bus Address: %v\n", usb.BusAddress)       //nolint:forbidigo
	_, _ = fmt.Printf(prefix+"Device Address: %v\n", usb.DeviceAddress) //nolint:forbidigo

	if len(usb.Serial) > 0 {
		_, _ = fmt.Printf(prefix+"Serial Number: %v\n", usb.Serial) //nolint:forbidigo
	}
}

func renderResourcesPCI(pci incusapi.ResourcesPCIDevice, prefix string) {
	_, _ = fmt.Printf(prefix+"Address: %v\n", pci.PCIAddress)     //nolint:forbidigo
	_, _ = fmt.Printf(prefix+"Vendor: %v\n", pci.Vendor)          //nolint:forbidigo
	_, _ = fmt.Printf(prefix+"Vendor ID: %v\n", pci.VendorID)     //nolint:forbidigo
	_, _ = fmt.Printf(prefix+"Product: %v\n", pci.Product)        //nolint:forbidigo
	_, _ = fmt.Printf(prefix+"Product ID: %v\n", pci.ProductID)   //nolint:forbidigo
	_, _ = fmt.Printf(prefix+"NUMA node: %v\n", pci.NUMANode)     //nolint:forbidigo
	_, _ = fmt.Printf(prefix+"IOMMU group: %v\n", pci.IOMMUGroup) //nolint:forbidigo
	_, _ = fmt.Printf(prefix+"Driver: %v\n", pci.Driver)          //nolint:forbidigo
}

func renderResourcesSerial(serial incusapi.ResourcesSerialDevice, prefix string) {
	_, _ = fmt.Printf(prefix+"Id: %v\n", serial.ID)                 //nolint:forbidigo
	_, _ = fmt.Printf(prefix+"Device: %v\n", serial.Device)         //nolint:forbidigo
	_, _ = fmt.Printf(prefix+"DeviceID: %v\n", serial.DeviceID)     //nolint:forbidigo
	_, _ = fmt.Printf(prefix+"DevicePath: %v\n", serial.DevicePath) //nolint:forbidigo
	_, _ = fmt.Printf(prefix+"Vendor: %v\n", serial.Vendor)         //nolint:forbidigo
	_, _ = fmt.Printf(prefix+"Vendor ID: %v\n", serial.VendorID)    //nolint:forbidigo
	_, _ = fmt.Printf(prefix+"Product: %v\n", serial.Product)       //nolint:forbidigo
	_, _ = fmt.Printf(prefix+"Product ID: %v\n", serial.ProductID)  //nolint:forbidigo
	_, _ = fmt.Printf(prefix+"Driver: %v\n", serial.Driver)         //nolint:forbidigo
}

// systemInfoResourcesCommand returns an info command for the system/resources endpoint.
func systemInfoResourcesCommand(c *cmdAdminOS, endpoint string, description string) *cobra.Command {
	return (&cmdGenericInfo{
		os:          c,
		endpoint:    endpoint,
		description: description,
		handler:     renderResourcesInfo,
	}).command()
}
