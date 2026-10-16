package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/lxc/incus/v7/shared/subprocess"

	"github.com/lxc/incus-os/incus-osd/api"
	"github.com/lxc/incus-os/incus-osd/internal/scheduling"
	"github.com/lxc/incus-os/incus-osd/internal/state"
	"github.com/lxc/incus-os/incus-osd/internal/systemd"
)

// LVM represents the system LVM service.
type LVM struct {
	common

	state *state.State
}

// Get returns the current service state.
func (n *LVM) Get(ctx context.Context) (any, error) {
	// Get PV state.
	type pvsRaw struct {
		Report []struct {
			PV []api.ServiceLVMPV `json:"pv"`
		} `json:"report"`
	}

	pvsOutput, err := subprocess.RunCommandContext(ctx, "pvs", "--reportformat", "json_std")
	if err != nil {
		return nil, err
	}

	pvs := pvsRaw{}

	err = json.Unmarshal([]byte(pvsOutput), &pvs)
	if err != nil {
		return nil, err
	}

	if len(pvs.Report) > 0 {
		n.state.Services.LVM.State.PVs = pvs.Report[0].PV
	}

	// Get VG state.
	type vgsRaw struct {
		Report []struct {
			VG []api.ServiceLVMVG `json:"vg"`
		} `json:"report"`
		Log []api.ServiceLVMLog `json:"log"`
	}

	vgsOutput, err := subprocess.RunCommandContext(ctx, "vgs", "--reportformat", "json_std")
	if err != nil {
		return nil, err
	}

	vgs := vgsRaw{}

	err = json.Unmarshal([]byte(vgsOutput), &vgs)
	if err != nil {
		return nil, err
	}

	if len(vgs.Report) > 0 {
		n.state.Services.LVM.State.VGs = vgs.Report[0].VG
	}

	n.state.Services.LVM.State.Log = vgs.Log

	return n.state.Services.LVM, nil
}

// Update updates the service configuration.
func (n *LVM) Update(ctx context.Context, req any) error {
	newState, ok := req.(*api.ServiceLVM)
	if !ok {
		return fmt.Errorf("request type \"%T\" isn't expected ServiceLVM", req)
	}

	// Save the state on return.
	defer n.state.Save()

	// Disable the service if requested.
	if n.state.Services.LVM.Config.Enabled && !newState.Config.Enabled {
		err := n.Stop(ctx)
		if err != nil {
			return err
		}
	}

	// Enable the service if requested.
	if !n.state.Services.LVM.Config.Enabled && newState.Config.Enabled {
		// Update the configuration.
		n.state.Services.LVM.Config = newState.Config

		// Start the service.
		err := n.Start(ctx)
		if err != nil {
			return err
		}
	} else {
		// Update the configuration.
		n.state.Services.LVM.Config = newState.Config

		// Re-configure the service.
		err := n.configure(ctx)
		if err != nil {
			return err
		}
	}

	return nil
}

// Stop stops the service.
func (n *LVM) Stop(ctx context.Context) error {
	if !n.state.Services.LVM.Config.Enabled {
		return nil
	}

	// Stop the lock manager.
	_, err := subprocess.RunCommandContext(ctx, "vgchange", "--lock-stop")
	if err != nil {
		return err
	}

	// Stop the units.
	err = systemd.StopUnit(ctx, "lvmlockd.service", "sanlock.service", "wdmd.service")
	if err != nil {
		return err
	}

	return nil
}

// Start starts the service.
func (n *LVM) Start(ctx context.Context) error {
	if !n.state.Services.LVM.Config.Enabled {
		return nil
	}

	// Generate configuration.
	err := n.configure(ctx)
	if err != nil {
		return err
	}

	// Start the units.
	err = systemd.StartUnit(ctx, "lvmlockd.service", "sanlock.service", "wdmd.service")
	if err != nil {
		return err
	}

	// Start the lock manager.
	return n.lockStart(ctx)
}

// Reset will forcefully reset the service.
func (n *LVM) Reset(ctx context.Context) error {
	if !n.state.Services.LVM.Config.Enabled {
		return errors.New("LVM isn't currently enabled")
	}

	// Kill all relevant units.
	err := systemd.KillUnit(ctx, "SIGKILL", "lvmlockd.service", "sanlock.service", "wdmd.service")
	if err != nil {
		return err
	}

	// Clear wdmd shared memory file if present.
	_ = os.Remove("/dev/shm/wdmd")

	// Start the services back up.
	return n.Start(ctx)
}

// ShouldStart returns true if the service should be started on boot.
func (n *LVM) ShouldStart() bool {
	return n.state.Services.LVM.Config.Enabled
}

// Struct returns the API struct for the LVM service.
func (*LVM) Struct() any {
	return &api.ServiceLVM{}
}

func (n *LVM) configure(_ context.Context) error {
	// Apply configuration.
	err := os.MkdirAll("/etc/lvm/", 0o700)
	if err != nil {
		return err
	}

	// Metadata archiving is disabled as /etc/lvm is wiped on every boot and the archive otherwise grows unbounded.
	lvmlocal := fmt.Sprintf(`global {
	use_lvmlockd = 1
}

local {
	host_id = %d
}

devices {
	filter = [ "r|/dev/zd*|/dev/rbd*|/dev/nbd*|" ]
}

backup {
	archive = 0
}
`, n.state.Services.LVM.Config.SystemID)

	err = os.WriteFile("/etc/lvm/lvmlocal.conf", []byte(lvmlocal), 0o600)
	if err != nil {
		return err
	}

	// Disable writing log output to files, although an empty file will still be created.
	err = os.WriteFile("/etc/default/sanlock", []byte(`sanlock_opts='-L -1'
`), 0o644)
	if err != nil {
		return err
	}

	return nil
}

// lockStart starts the lockspaces, retrying while lvmlockd and sanlock finish coming up.
func (*LVM) lockStart(ctx context.Context, vgs ...string) error {
	var err error

	args := append([]string{"--lock-start"}, vgs...)

	for range 30 {
		_, err = subprocess.RunCommandContext(ctx, "vgchange", args...)
		if err == nil {
			return nil
		}

		// Only the lock manager not being reachable yet is worth waiting for.
		if !strings.Contains(err.Error(), "is not running") && !strings.Contains(err.Error(), "did not find one running") {
			return err
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Second):
		}
	}

	return err
}

// LVMLockspaceJob is the name of the scheduled lockspace check.
const LVMLockspaceJob scheduling.JobName = "lvm_lockspace_check"

var lvmKilledVGRegexp = regexp.MustCompile(`^VG (\S+) lock (?:skipped|failed): storage failed for sanlock leases$`)

// CheckLockspaces recovers shared VGs whose sanlock lockspace was lost or never started.
func CheckLockspaces(ctx context.Context, s *state.State) error {
	srv, err := Load(ctx, s, "lvm")
	if err != nil {
		return err
	}

	lvm, ok := srv.(*LVM)
	if !ok || !lvm.state.Services.LVM.Config.Enabled {
		return nil
	}

	return lvm.recoverLockspaces(ctx)
}

// recoverLockspaces drops killed lockspaces and starts every shared VG that has none.
func (n *LVM) recoverLockspaces(ctx context.Context) error {
	type vgsRaw struct {
		Report []struct {
			VG []struct {
				VGName     string `json:"vg_name"`
				VGLockType string `json:"vg_lock_type"`
			} `json:"vg"`
		} `json:"report"`
		Log []api.ServiceLVMLog `json:"log"`
	}

	vgsOutput, err := subprocess.RunCommandContext(ctx, "vgs", "--reportformat", "json_std", "-o", "vg_name,vg_lock_type")
	if err != nil {
		return err
	}

	vgs := vgsRaw{}

	err = json.Unmarshal([]byte(vgsOutput), &vgs)
	if err != nil {
		return err
	}

	shared := []string{}

	for _, report := range vgs.Report {
		for _, vg := range report.VG {
			if vg.VGLockType == "sanlock" {
				shared = append(shared, vg.VGName)
			}
		}
	}

	if len(shared) == 0 {
		return nil
	}

	// A killed lockspace (sanlock lease lost) is only cleared by dropping it.
	killed := lvmKilledVGs(vgs.Log)
	for _, vg := range killed {
		slog.WarnContext(ctx, "Dropping lost LVM lockspace", "vg", vg)

		_, err = subprocess.RunCommandContext(ctx, "lvmlockctl", "--drop", vg)
		if err != nil {
			slog.ErrorContext(ctx, "Failed dropping LVM lockspace", "vg", vg, "err", err)
		}
	}

	infoOutput, err := subprocess.RunCommandContext(ctx, "lvmlockctl", "--info")
	if err != nil {
		return err
	}

	started := lvmStartedLockspaces(infoOutput)

	// A dropped lockspace may still be listed while lvmlockd removes it.
	for _, vg := range killed {
		delete(started, vg)
	}

	for _, vg := range shared {
		if started[vg] {
			continue
		}

		slog.WarnContext(ctx, "Starting LVM lockspace", "vg", vg)

		err = n.lockStart(ctx, vg)
		if err != nil {
			slog.ErrorContext(ctx, "Failed starting LVM lockspace", "vg", vg, "err", err)

			continue
		}

		// LVs left active by a killed lockspace have no lock anymore, take them again.
		if slices.Contains(killed, vg) {
			n.relockActiveLVs(ctx, vg)
		}
	}

	return nil
}

// relockActiveLVs re-acquires the shared lock of every active LV in the VG.
func (*LVM) relockActiveLVs(ctx context.Context, vg string) {
	type lvsRaw struct {
		Report []struct {
			LV []struct {
				LVName string `json:"lv_name"`
			} `json:"lv"`
		} `json:"report"`
	}

	lvsOutput, err := subprocess.RunCommandContext(ctx, "lvs", "--reportformat", "json_std", "-o", "lv_name", "-S", "lv_active=active", vg)
	if err != nil {
		slog.ErrorContext(ctx, "Failed listing active LVs", "vg", vg, "err", err)

		return
	}

	lvs := lvsRaw{}

	err = json.Unmarshal([]byte(lvsOutput), &lvs)
	if err != nil {
		slog.ErrorContext(ctx, "Failed parsing active LVs", "vg", vg, "err", err)

		return
	}

	for _, report := range lvs.Report {
		for _, lv := range report.LV {
			_, err = subprocess.RunCommandContext(ctx, "lvchange", "--activate", "sy", "--ignoreactivationskip", vg+"/"+lv.LVName)
			if err != nil {
				slog.WarnContext(ctx, "Failed re-locking active LV", "vg", vg, "lv", lv.LVName, "err", err)
			}
		}
	}
}

// lvmKilledVGs returns the VGs whose lockspace lvmlockd reports as killed.
func lvmKilledVGs(logs []api.ServiceLVMLog) []string {
	vgs := []string{}

	for _, entry := range logs {
		match := lvmKilledVGRegexp.FindStringSubmatch(entry.LogMessage)
		if match == nil {
			continue
		}

		if !slices.Contains(vgs, match[1]) {
			vgs = append(vgs, match[1])
		}
	}

	return vgs
}

// lvmStartedLockspaces parses "lvmlockctl --info" into the set of VGs with a started lockspace.
func lvmStartedLockspaces(output string) map[string]bool {
	started := map[string]bool{}

	for line := range strings.SplitSeq(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) != 3 || fields[0] != "LS" {
			continue
		}

		started[strings.TrimPrefix(fields[2], "lvm_")] = true
	}

	return started
}
