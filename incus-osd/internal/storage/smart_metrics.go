package storage

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/lxc/incus-os/incus-osd/api"
)

// SMARTMetricsJob is the name of the periodic SMART metrics export job.
const SMARTMetricsJob string = "smart_metrics"

// smartMetricsPath is where the node_exporter textfile collector picks up SMART metrics.
const smartMetricsPath = "/var/lib/prometheus/node-exporter/smartmon.prom"

// WriteSMARTMetrics exports the SMART data of all drives for the node_exporter textfile collector.
func WriteSMARTMetrics(ctx context.Context) error {
	info, _, err := GetStorageInfo(ctx)
	if err != nil {
		return err
	}

	err = os.MkdirAll(filepath.Dir(smartMetricsPath), 0o755)
	if err != nil {
		return err
	}

	// Write to a temporary file first so the collector never reads a partial file.
	tmpPath := smartMetricsPath + ".tmp"

	err = os.WriteFile(tmpPath, []byte(renderSMARTMetrics(info.Drives)), 0o644)
	if err != nil {
		return err
	}

	return os.Rename(tmpPath, smartMetricsPath)
}

// renderSMARTMetrics formats the drives' SMART data using the Prometheus text exposition format.
func renderSMARTMetrics(drives []api.SystemStorageDrive) string {
	type family struct {
		name    string
		help    string
		samples []string
	}

	families := []*family{}
	byName := map[string]*family{}

	add := func(name string, help string, labels string, value int) {
		f, ok := byName[name]
		if !ok {
			f = &family{name: name, help: help}
			byName[name] = f
			families = append(families, f)
		}

		f.samples = append(f.samples, fmt.Sprintf("%s{%s} %d", name, labels, value))
	}

	boolToInt := func(v bool) int {
		if v {
			return 1
		}

		return 0
	}

	for _, drive := range drives {
		labels := fmt.Sprintf("disk=%s,type=%s", quoteLabel(drive.ID), quoteLabel(drive.Bus))

		add("smartmon_device_info", "Drive information", fmt.Sprintf("%s,model_family=%s,model_name=%s,serial_number=%s", labels, quoteLabel(drive.ModelFamily), quoteLabel(drive.ModelName), quoteLabel(drive.SerialNumber)), 1)
		add("smartmon_device_active", "Drive is present and active", labels, 1)
		add("smartmon_device_smart_available", "SMART is supported by the drive", labels, boolToInt(drive.SMART != nil))

		if drive.SMART == nil {
			continue
		}

		add("smartmon_device_smart_enabled", "SMART is enabled on the drive", labels, boolToInt(drive.SMART.Enabled))
		add("smartmon_device_smart_healthy", "SMART overall health self-assessment passed", labels, boolToInt(drive.SMART.Passed))

		// Optional attributes are only reported when the drive exposes them.
		optional := []struct {
			name  string
			help  string
			value int
		}{
			{"smartmon_power_on_hours_raw_value", "Hours the drive has been powered on", drive.SMART.PowerOnHours},
			{"smartmon_temperature_celsius_raw_value", "Drive temperature in Celsius", drive.SMART.TemperatureCelsius},
			{"smartmon_percentage_used_raw_value", "NVMe estimated percentage of rated life used", drive.SMART.PercentageUsed},
			{"smartmon_available_spare_raw_value", "NVMe remaining spare capacity percentage", drive.SMART.AvailableSpare},
			{"smartmon_data_units_read_raw_value", "NVMe data units read (512000 bytes each)", drive.SMART.DataUnitsRead},
			{"smartmon_data_units_written_raw_value", "NVMe data units written (512000 bytes each)", drive.SMART.DataUnitsWritten},
			{"smartmon_raw_read_error_rate_raw_value", "ATA raw read error rate", drive.SMART.RawReadErrorRate},
			{"smartmon_seek_error_rate_raw_value", "ATA seek error rate", drive.SMART.SeekErrorRate},
			{"smartmon_reallocated_sector_ct_raw_value", "ATA reallocated sectors count", drive.SMART.ReallocatedSectors},
		}

		for _, attr := range optional {
			if attr.value > 0 {
				add(attr.name, attr.help, labels, attr.value)
			}
		}
	}

	lines := []string{}

	for _, f := range families {
		lines = append(lines, "# HELP "+f.name+" "+f.help, "# TYPE "+f.name+" gauge")
		lines = append(lines, f.samples...)
	}

	return strings.Join(lines, "\n") + "\n"
}

// quoteLabel escapes a label value for the Prometheus text exposition format.
func quoteLabel(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, `"`, `\"`)
	value = strings.ReplaceAll(value, "\n", `\n`)

	return `"` + value + `"`
}
