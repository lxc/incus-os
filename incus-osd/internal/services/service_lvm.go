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
	"sync"
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

// Debug returns the full PV, VG and LV reports read from the on-disk metadata.
func (n *LVM) Debug(ctx context.Context) (any, error) {
	info := &api.DebugLVM{}

	reports := []struct {
		command string
		key     string
		fields  string
		dest    *[]map[string]any
	}{
		{"pvs", "pv", "pv_all,vg_name,vg_uuid", &info.PVs},
		{"vgs", "vg", "vg_all", &info.VGs},
		{"lvs", "lv", "lv_all,vg_name,vg_uuid", &info.LVs},
	}

	// Read-only mode takes no lock and only reads the on-disk metadata.
	for _, report := range reports {
		output, err := subprocess.RunCommandContext(ctx, report.command, "--readonly", "--reportformat", "json_std", "--units", "b", "--nosuffix", "-o", report.fields)
		if err != nil {
			return nil, err
		}

		rows, err := n.reportRows(output, report.key)
		if err != nil {
			return nil, err
		}

		*report.dest = rows
	}

	return info, nil
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

	// Stop the lockspace check.
	_ = jobScheduler.RemoveJob(LVMLockspaceJob)

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
	err = n.lockStart(ctx)
	if err != nil {
		return err
	}

	// Check the lockspaces periodically.
	return jobScheduler.RegisterJob(LVMLockspaceJob, "1m", CheckLockspaces, n.state)
}

// Reset will forcefully reset the service.
func (n *LVM) Reset(ctx context.Context) error {
	if !n.state.Services.LVM.Config.Enabled {
		return errors.New("LVM isn't currently enabled")
	}

	// Kill the lock managers.
	err := systemd.KillUnit(ctx, "SIGKILL", "lvmlockd.service", "sanlock.service")
	if err != nil {
		return err
	}

	// Stop wdmd cleanly, an unclean close leaves the watchdog armed and resets the system.
	err = systemd.StopUnit(ctx, "wdmd.service")
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
	lvmlockctl_kill_command = "%s"
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
`, lvmKillCommand, n.state.Services.LVM.Config.SystemID)

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

// lvmKillCommand is the script lvmlockctl --kill runs to notify us of a lost lockspace.
const lvmKillCommand = "/usr/lib/incus-os/lvmlockctl-kill"

var (
	lvmKilledVGRegexp = regexp.MustCompile(`^VG (\S+) lock (?:skipped|failed): storage failed for sanlock leases$`)
	lvmRecoveryMu     sync.Mutex
)

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

// LockspaceLost is called by the lvmlockctl kill command and recovers the lockspaces in the background.
func LockspaceLost(ctx context.Context, s *state.State, vg string) error {
	srv, err := Load(ctx, s, "lvm")
	if err != nil {
		return err
	}

	lvm, ok := srv.(*LVM)
	if !ok || !lvm.state.Services.LVM.Config.Enabled {
		return errors.New("LVM isn't currently enabled")
	}

	slog.WarnContext(ctx, "LVM lockspace lost its lease, recovering once storage is back", "vg", vg)

	go lvm.recoverLockspacesRetry(context.WithoutCancel(ctx))

	return nil
}

// recoverLockspacesRetry runs the lockspace recovery until it succeeds, the VG commands block or fail while storage is down.
func (n *LVM) recoverLockspacesRetry(ctx context.Context) {
	var err error

	for range 60 {
		err = n.recoverLockspaces(ctx)
		if err == nil {
			return
		}

		select {
		case <-ctx.Done():
			return
		case <-time.After(5 * time.Second):
		}
	}

	slog.ErrorContext(ctx, "Failed recovering LVM lockspaces", "err", err)
}

// recoverLockspaces drops killed lockspaces and starts every shared VG that has none.
// Active LVs of a killed lockspace are relocked with the mode they held; if any lock can't be
// re-acquired, another host may own the LV, so the system is rebooted rather than left writing.
func (n *LVM) recoverLockspaces(ctx context.Context) error {
	lvmRecoveryMu.Lock()
	defer lvmRecoveryMu.Unlock()

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

	infoOutput, err := subprocess.RunCommandContext(ctx, "lvmlockctl", "--info")
	if err != nil {
		return err
	}

	lockspaces := lvmLockspaces(infoOutput)

	killed := lvmKilledVGs(vgs.Log)
	if len(killed) > 0 {
		sanlockOutput, err := subprocess.RunCommandContext(ctx, "sanlock", "client", "status")
		if err != nil {
			return err
		}

		sanlockLockspaces := sanlockLockspaces(sanlockOutput)

		for _, vg := range killed {
			// Without the host lease, other hosts may already own our LVs: stop writing.
			if !sanlockLockspaces[vg] {
				slog.ErrorContext(ctx, "LVM lockspace lost with its sanlock lease, resetting to protect shared storage", "vg", vg)

				n.fence()

				return nil
			}
		}
	}

	// A killed lockspace (sanlock lease lost) is only cleared by dropping it.
	heldLocks := map[string]map[string]string{}

	for _, vg := range killed {
		heldLocks[vg] = lockspaces[vg]

		slog.WarnContext(ctx, "Dropping lost LVM lockspace", "vg", vg)

		_, err = subprocess.RunCommandContext(ctx, "lvmlockctl", "--drop", vg)
		if err != nil {
			slog.ErrorContext(ctx, "Failed dropping LVM lockspace", "vg", vg, "err", err)
		}

		// A dropped lockspace may still be listed while lvmlockd removes it.
		delete(lockspaces, vg)
	}

	for _, vg := range shared {
		_, started := lockspaces[vg]
		if started {
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
			err = n.relockActiveLVs(ctx, vg, heldLocks[vg])
			if err != nil {
				slog.ErrorContext(ctx, "Failed re-locking active LVs, resetting to protect shared storage", "vg", vg, "err", err)

				n.fence()

				return nil
			}
		}
	}

	return nil
}

// fence resets the system immediately like the sanlock watchdog would, a clean reboot could keep writing for a long time.
func (*LVM) fence() {
	_ = os.WriteFile("/proc/sysrq-trigger", []byte("b"), 0o600)
}

// relockActiveLVs re-acquires the lock of every active LV in the VG with the mode it previously held.
func (*LVM) relockActiveLVs(ctx context.Context, vg string, modes map[string]string) error {
	type lvsRaw struct {
		Report []struct {
			LV []struct {
				LVName string `json:"lv_name"`
				LVUUID string `json:"lv_uuid"`
			} `json:"lv"`
		} `json:"report"`
	}

	lvsOutput, err := subprocess.RunCommandContext(ctx, "lvs", "--reportformat", "json_std", "-o", "lv_name,lv_uuid", "-S", "lv_active=active", vg)
	if err != nil {
		return err
	}

	lvs := lvsRaw{}

	err = json.Unmarshal([]byte(lvsOutput), &lvs)
	if err != nil {
		return err
	}

	for _, report := range lvs.Report {
		for _, lv := range report.LV {
			// Incus activates instance volumes exclusively, so that's the default for an unknown lock.
			activations := []string{"ey", "sy"}

			switch modes[lv.LVUUID] {
			case "ex":
				activations = []string{"ey"}
			case "sh":
				activations = []string{"sy"}
			default:
				slog.WarnContext(ctx, "Unknown previous lock mode for active LV", "vg", vg, "lv", lv.LVName)
			}

			for i, activation := range activations {
				_, err = subprocess.RunCommandContext(ctx, "lvchange", "--activate", activation, "--ignoreactivationskip", vg+"/"+lv.LVName)
				if err == nil {
					slog.InfoContext(ctx, "Re-locked active LV", "vg", vg, "lv", lv.LVName, "mode", activation)

					break
				}

				if i == len(activations)-1 {
					return fmt.Errorf("failed re-locking %s/%s: %w", vg, lv.LVName, err)
				}
			}
		}
	}

	return nil
}

// reportRows extracts the rows of the named report from json_std output.
func (*LVM) reportRows(output string, key string) ([]map[string]any, error) {
	raw := struct {
		Report []map[string][]map[string]any `json:"report"`
	}{}

	err := json.Unmarshal([]byte(output), &raw)
	if err != nil {
		return nil, err
	}

	rows := []map[string]any{}

	for _, report := range raw.Report {
		rows = append(rows, report[key]...)
	}

	return rows, nil
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

// lvmLockspaces parses "lvmlockctl --info" into the started lockspaces and the LV locks (uuid to mode) each holds.
func lvmLockspaces(output string) map[string]map[string]string {
	lockspaces := map[string]map[string]string{}
	current := ""

	for line := range strings.SplitSeq(output, "\n") {
		fields := strings.Fields(line)

		switch {
		case len(fields) >= 3 && fields[0] == "LS":
			current = strings.TrimPrefix(fields[2], "lvm_")
			lockspaces[current] = map[string]string{}
		case len(fields) == 4 && fields[0] == "LK" && fields[1] == "LV" && current != "":
			lockspaces[current][fields[3]] = fields[2]
		case len(fields) > 0 && fields[0] == "VG":
			current = ""
		default:
		}
	}

	return lockspaces
}

// sanlockLockspaces parses "sanlock client status" into the set of VGs whose lockspace sanlock still holds.
func sanlockLockspaces(output string) map[string]bool {
	lockspaces := map[string]bool{}

	for line := range strings.SplitSeq(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 || fields[0] != "s" {
			continue
		}

		name, _, _ := strings.Cut(fields[1], ":")

		vg, ok := strings.CutPrefix(name, "lvm_")
		if ok {
			lockspaces[vg] = true
		}
	}

	return lockspaces
}
