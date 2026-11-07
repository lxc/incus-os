package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
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

// Action runs a service-specific action.
func (n *LVM) Action(ctx context.Context, data api.ServiceAction) error {
	switch data.Action {
	case "clear-outage":
		return n.clearOutage(ctx)
	case "declare-outage":
		return n.declareOutage(ctx, data.Config)
	default:
		return errors.New("unsupported action '" + data.Action + "'")
	}
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
	n.state.Services.LVM.State.Outage = lvmOutage.status()

	// The reports below block while storage is down.
	if n.state.Services.LVM.State.Outage != nil {
		return n.state.Services.LVM, nil
	}

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

	if newState.Config.IOTimeout < 0 || newState.Config.WatchdogTimeout < 0 {
		return errors.New("timeouts can't be negative")
	}

	if lvmOutage.active() {
		return errors.New("the service can't be changed while recovering from a storage outage")
	}

	// Save the state on return.
	defer n.state.Save()

	running := n.state.Services.LVM.Config.Enabled

	// Stop the service if it's being disabled or reconfigured, the lockspaces can't be changed live.
	if running && newState.Config != n.state.Services.LVM.Config {
		err := n.Stop(ctx)
		if err != nil {
			return err
		}

		running = false
	}

	// Update the configuration.
	n.state.Services.LVM.Config = newState.Config

	// Refresh the configuration files of an already running service.
	if running {
		return n.configure(ctx)
	}

	return n.Start(ctx)
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
	filter = [ "r|^/dev/drbd|", "r|^/dev/mapper/[lL]instor|", "r|^/dev/zd|", "r|^/dev/rbd|", "r|^/dev/nbd|" ]
	issue_discards = 1
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

	// Lease timing, lvmlockd leaves the I/O timeout to sanlock which renews every 2x and expires after 8x.
	err = os.MkdirAll("/etc/sanlock/", 0o755)
	if err != nil {
		return err
	}

	sanlockConf := ""

	if n.state.Services.LVM.Config.IOTimeout > 0 {
		sanlockConf += fmt.Sprintf("io_timeout = %d\n", n.state.Services.LVM.Config.IOTimeout)
	}

	if n.state.Services.LVM.Config.WatchdogTimeout > 0 {
		sanlockConf += fmt.Sprintf("watchdog_fire_timeout = %d\n", n.state.Services.LVM.Config.WatchdogTimeout)
	}

	return os.WriteFile("/etc/sanlock/sanlock.conf", []byte(sanlockConf), 0o644)
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

// lvmOutageDefaultTimeout is how long a declaration stays armed by default.
const lvmOutageDefaultTimeout = time.Hour

var (
	lvmKilledVGRegexp = regexp.MustCompile(`^VG (\S+) lock (?:skipped|failed): storage failed for sanlock leases$`)
	lvmOutage         = lvmOutageState{lost: map[string]bool{}, quiesced: map[string]*lvmQuiescedVG{}, ctrlLoss: map[string]string{}}
	lvmRecoveryMu     sync.Mutex
)

// lvmQuiescedVG is a VG that gave up its lockspace with its I/O held back.
type lvmQuiescedVG struct {
	modes   map[string]string
	devices []string

	// Closed once the suspends and the lockspace drop are over, which usually takes storage returning.
	settled chan struct{}
}

// lvmOutageState tracks a declared storage outage. It's never held across commands, those can block on storage.
type lvmOutageState struct {
	mu       sync.Mutex
	until    time.Time
	lost     map[string]bool
	quiesced map[string]*lvmQuiescedVG

	// Original ctrl_loss_tmo sysfs values of the NVMe-oF controllers.
	ctrlLoss map[string]string
}

// active returns true if any VG is waiting to recover from the outage.
func (o *lvmOutageState) active() bool {
	o.mu.Lock()
	defer o.mu.Unlock()

	return len(o.quiesced) > 0
}

// armed returns true if a lost lockspace should currently be given up.
func (o *lvmOutageState) armed() bool {
	o.mu.Lock()
	defer o.mu.Unlock()

	return time.Now().Before(o.until)
}

// declare arms the outage until the provided time.
func (o *lvmOutageState) declare(until time.Time) {
	o.mu.Lock()
	defer o.mu.Unlock()

	o.until = until
}

// clear disarms the outage, this isn't possible once VGs depend on it to recover.
func (o *lvmOutageState) clear() error {
	o.mu.Lock()
	defer o.mu.Unlock()

	if len(o.quiesced) > 0 {
		return errors.New("volumes are suspended and only recover once storage is back")
	}

	o.until = time.Time{}

	return nil
}

// lockspaceLost records a VG that lost its lease and hasn't been recovered yet.
func (o *lvmOutageState) lockspaceLost(vg string) {
	o.mu.Lock()
	defer o.mu.Unlock()

	o.lost[vg] = true
}

// claimLost moves the lost VGs to the quiesced ones.
func (o *lvmOutageState) claimLost() map[string]*lvmQuiescedVG {
	o.mu.Lock()
	defer o.mu.Unlock()

	claimed := map[string]*lvmQuiescedVG{}

	for vg := range o.lost {
		claimed[vg] = &lvmQuiescedVG{settled: make(chan struct{})}
		o.quiesced[vg] = claimed[vg]
		delete(o.lost, vg)
	}

	return claimed
}

// claimVisible hands the VGs now readable to the recovery, so a late declaration can't quiesce them.
// It returns the quiesced VGs and the killed ones left to handle the usual way.
func (o *lvmOutageState) claimVisible(shared []string, killed []string) (map[string]*lvmQuiescedVG, []string) {
	o.mu.Lock()
	defer o.mu.Unlock()

	for _, vg := range shared {
		delete(o.lost, vg)
	}

	remaining := slices.DeleteFunc(killed, func(vg string) bool {
		_, quiesced := o.quiesced[vg]

		return quiesced
	})

	return maps.Clone(o.quiesced), remaining
}

// saveCtrlLoss records the original value of a controller, returning false if it's already held.
func (o *lvmOutageState) saveCtrlLoss(path string, value string) bool {
	o.mu.Lock()
	defer o.mu.Unlock()

	_, held := o.ctrlLoss[path]
	if held {
		return false
	}

	o.ctrlLoss[path] = value

	return true
}

// release forgets a recovered or abandoned VG. The outage is over once none are left, at which point the held controllers are returned.
func (o *lvmOutageState) release(vg string) map[string]string {
	o.mu.Lock()
	defer o.mu.Unlock()

	delete(o.quiesced, vg)

	if len(o.quiesced) > 0 {
		return nil
	}

	o.until = time.Time{}

	held := o.ctrlLoss
	o.ctrlLoss = map[string]string{}

	return held
}

// status returns the API representation of the outage, if there is one.
func (o *lvmOutageState) status() *api.ServiceLVMOutage {
	o.mu.Lock()
	defer o.mu.Unlock()

	if len(o.quiesced) == 0 && !time.Now().Before(o.until) {
		return nil
	}

	return &api.ServiceLVMOutage{
		DeclaredUntil: o.until,
		QuiescedVGs:   slices.Sorted(maps.Keys(o.quiesced)),
	}
}

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

	// The retry loop owns the recovery during an outage.
	if lvmOutage.active() {
		return nil
	}

	return lvm.recoverLockspaces(ctx)
}

// LockspaceLost is called by the lvmlockctl kill command and recovers the lockspaces in the background.
func LockspaceLost(ctx context.Context, s *state.State, vg string) error {
	lvm, err := loadEnabledLVM(ctx, s)
	if err != nil {
		return err
	}

	slog.WarnContext(ctx, "LVM lockspace lost its lease, recovering once storage is back", "vg", vg)

	ctx = context.WithoutCancel(ctx)

	lvmOutage.lockspaceLost(vg)

	if lvmOutage.armed() {
		lvm.quiesceLostVGs(ctx)

		return nil
	}

	go lvm.recoverLockspacesRetry(ctx)

	return nil
}

// declareOutage asserts that no server can write to the shared storage. It applies to any lease loss until the timeout (in seconds).
func (n *LVM) declareOutage(ctx context.Context, config map[string]string) error {
	if !n.state.Services.LVM.Config.Enabled {
		return errors.New("LVM isn't currently enabled")
	}

	duration := lvmOutageDefaultTimeout

	if config["timeout"] != "" {
		timeout, err := strconv.Atoi(config["timeout"])
		if err != nil || timeout <= 0 {
			return errors.New("invalid timeout '" + config["timeout"] + "'")
		}

		duration = time.Duration(timeout) * time.Second
	}

	until := time.Now().Add(duration)

	slog.WarnContext(ctx, "Global storage outage declared, watchdog fencing is disarmed on lease loss", "until", until)

	lvmOutage.declare(until)

	// Leases already lost have little time left before the watchdog fires.
	n.quiesceLostVGs(context.WithoutCancel(ctx))

	return nil
}

// clearOutage withdraws an outage declaration which hasn't taken effect.
func (n *LVM) clearOutage(ctx context.Context) error {
	if !n.state.Services.LVM.Config.Enabled {
		return errors.New("LVM isn't currently enabled")
	}

	err := lvmOutage.clear()
	if err != nil {
		return err
	}

	slog.WarnContext(ctx, "Global storage outage declaration cleared")

	return nil
}

// loadEnabledLVM returns the LVM service if it's enabled.
func loadEnabledLVM(ctx context.Context, s *state.State) (*LVM, error) {
	srv, err := Load(ctx, s, "lvm")
	if err != nil {
		return nil, err
	}

	lvm, ok := srv.(*LVM)
	if !ok || !lvm.state.Services.LVM.Config.Enabled {
		return nil, errors.New("LVM isn't currently enabled")
	}

	return lvm, nil
}

// quiesceLostVGs gives up the lockspace of every VG that lost its lease.
func (n *LVM) quiesceLostVGs(ctx context.Context) {
	claimed := lvmOutage.claimLost()

	for vg, quiesced := range claimed {
		go n.quiesceVG(ctx, vg, quiesced)
	}

	if len(claimed) > 0 {
		go n.recoverLockspacesRetry(ctx)
	}
}

// quiesceVG holds back the I/O of a VG and drops its lockspace, which is what disarms the watchdog.
// Those commands block until storage returns, so a suspended LV is only resumed by re-acquiring its lock.
func (n *LVM) quiesceVG(ctx context.Context, vg string, quiesced *lvmQuiescedVG) {
	defer close(quiesced.settled)

	infoOutput, err := subprocess.RunCommandContext(ctx, "lvmlockctl", "--info")
	if err != nil {
		slog.ErrorContext(ctx, "Failed reading LVM locks, leaving the lockspace to the watchdog", "vg", vg, "err", err)
		n.releaseVG(ctx, vg)

		return
	}

	dmOutput, err := subprocess.RunCommandContext(ctx, "dmsetup", "info", "-c", "--noheadings", "--separator", ":", "-o", "name,subsystem,vg_name,lv_name,lv_layer")
	if err != nil {
		slog.ErrorContext(ctx, "Failed listing active LVs, leaving the lockspace to the watchdog", "vg", vg, "err", err)
		n.releaseVG(ctx, vg)

		return
	}

	quiesced.modes = lvmLockspaces(infoOutput)[vg]
	quiesced.devices = lvmDevices(dmOutput, vg)

	slog.WarnContext(ctx, "Suspending LVs and dropping the LVM lockspace for the declared outage", "vg", vg, "devices", quiesced.devices)

	n.holdNVMeControllers(ctx)

	var wg sync.WaitGroup

	for _, device := range quiesced.devices {
		wg.Go(func() {
			_, err := subprocess.RunCommandContext(ctx, "dmsetup", "suspend", "--noflush", "--nolockfs", device)
			if err != nil {
				slog.ErrorContext(ctx, "Failed suspending LV", "device", device, "err", err)
			}
		})
	}

	wg.Go(func() {
		_, err := subprocess.RunCommandContext(ctx, "lvmlockctl", "--drop", vg)
		if err != nil {
			slog.ErrorContext(ctx, "Failed dropping LVM lockspace", "vg", vg, "err", err)
		}
	})

	wg.Wait()
}

// recoverQuiescedVG takes back the locks of a quiesced VG and resumes its LVs.
func (n *LVM) recoverQuiescedVG(ctx context.Context, vg string, quiesced *lvmQuiescedVG, started bool) error {
	select {
	case <-quiesced.settled:
	default:
		return errors.New("still waiting on the suspends and the lockspace drop")
	}

	if !started {
		slog.WarnContext(ctx, "Starting LVM lockspace", "vg", vg)

		err := n.lockStart(ctx, vg)
		if err != nil {
			return err
		}
	}

	err := n.relockActiveLVs(ctx, vg, quiesced.modes)
	if err != nil {
		return err
	}

	// The relock usually resumes the LV by itself and the device may have been removed since.
	for _, device := range quiesced.devices {
		_, err = os.Stat("/dev/mapper/" + device)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}

		_, err = subprocess.RunCommandContext(ctx, "dmsetup", "resume", device)
		if err != nil {
			return err
		}
	}

	n.releaseVG(ctx, vg)

	slog.WarnContext(ctx, "LVM lockspace recovered from the declared outage", "vg", vg)

	return nil
}

// holdNVMeControllers stops NVMe-oF controllers from being removed by a long outage, their I/O would then fail and they wouldn't reconnect.
func (*LVM) holdNVMeControllers(ctx context.Context) {
	paths, err := filepath.Glob("/sys/class/nvme/*/ctrl_loss_tmo")
	if err != nil {
		slog.WarnContext(ctx, "Failed listing NVMe controllers", "err", err)

		return
	}

	for _, path := range paths {
		value, err := os.ReadFile(path) //nolint:gosec
		if err != nil {
			slog.WarnContext(ctx, "Failed reading NVMe controller loss timeout", "path", path, "err", err)

			continue
		}

		if !lvmOutage.saveCtrlLoss(path, strings.TrimSpace(string(value))) {
			continue
		}

		err = os.WriteFile(path, []byte("-1"), 0o600)
		if err != nil {
			slog.WarnContext(ctx, "Failed disabling NVMe controller loss timeout", "path", path, "err", err)
		}
	}
}

// releaseVG ends the outage for a VG and restores the NVMe-oF controllers once it's over.
func (*LVM) releaseVG(ctx context.Context, vg string) {
	for path, value := range lvmOutage.release(vg) {
		// The kernel reports no timeout as "off" but only accepts a negative value.
		if value == "off" {
			value = "-1"
		}

		err := os.WriteFile(path, []byte(value), 0o600)
		if err != nil {
			slog.WarnContext(ctx, "Failed restoring NVMe controller loss timeout", "path", path, "err", err)
		}
	}
}

// recoverLockspacesRetry runs the lockspace recovery until it succeeds, the VG commands block or fail while storage is down.
func (n *LVM) recoverLockspacesRetry(ctx context.Context) {
	var err error

	// A declared outage may last a long time.
	for attempt := 0; attempt < 60 || lvmOutage.active(); attempt++ {
		err = n.recoverLockspaces(ctx)
		if err == nil && !lvmOutage.active() {
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

	quiesced, killed := lvmOutage.claimVisible(shared, lvmKilledVGs(vgs.Log))

	infoOutput, err := subprocess.RunCommandContext(ctx, "lvmlockctl", "--info")
	if err != nil {
		return err
	}

	lockspaces := lvmLockspaces(infoOutput)

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

		quiescedVG, isQuiesced := quiesced[vg]
		if isQuiesced {
			err = n.recoverQuiescedVG(ctx, vg, quiescedVG, started)
			if err != nil && strings.Contains(err.Error(), "LV locked by other host") {
				slog.ErrorContext(ctx, "Another server took an LV during the declared outage, resetting to protect shared storage", "vg", vg, "err", err)

				n.fence()

				return nil
			}

			// Anything else can be retried as the suspended LVs can't write.
			if err != nil {
				slog.WarnContext(ctx, "Failed recovering from the declared outage, retrying", "vg", vg, "err", err)
			}

			continue
		}

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
				LVName string   `json:"lv_name"`
				LVUUID string   `json:"lv_uuid"`
				LVRole []string `json:"lv_role"`
			} `json:"lv"`
		} `json:"report"`
	}

	lvsOutput, err := subprocess.RunCommandContext(ctx, "lvs", "--reportformat", "json_std", "-o", "lv_name,lv_uuid,lv_role", "-S", "lv_active=active", vg)
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
			// A snapshot has no lock of its own, it's covered by its origin.
			if slices.Contains(lv.LVRole, "thicksnapshot") {
				continue
			}

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

// lvmDevices parses "dmsetup info" into the top-level devices of the VG's LVs, leaving out the one holding the sanlock leases.
func lvmDevices(output string, vg string) []string {
	devices := []string{}

	for line := range strings.SplitSeq(output, "\n") {
		fields := strings.Split(strings.TrimSpace(line), ":")
		if len(fields) != 5 || fields[1] != "LVM" || fields[2] != vg || fields[3] == "lvmlock" || fields[4] != "" {
			continue
		}

		devices = append(devices, fields[0])
	}

	return devices
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
