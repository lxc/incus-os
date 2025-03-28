package tui

import (
	"context"
	"fmt"
	"net"
	"os"
	"slices"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/lxc/incus-os/incus-osd/internal/state"
	"github.com/lxc/incus-os/incus-osd/internal/systemd"
)

// TUI represents a terminal user interface.
type TUI struct {
	app      *tview.Application
	frame    *tview.Frame
	textView *tview.TextView
}

// NewTUI returns a new TUI application that will show basic information and recent
// log entries on the system's console.
func NewTUI() (*TUI, error) {
	ret := &TUI{}

	// Attempt to open the system's console.
	tty, err := tcell.NewDevTtyFromDev("/dev/console")
	if err != nil {
		return ret, err
	}

	// Construct a screen bound to the system console.
	screen, err := tcell.NewTerminfoScreenFromTty(tty)
	if err != nil {
		return ret, err
	}

	// Define a text view to show recent log entries.
	ret.textView = tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(false).
		SetWordWrap(true).
		SetChangedFunc(func() {
			ret.app.Draw()
		})
	ret.textView.SetBorder(true)

	// Define a frame to hold the TUI's content.
	ret.frame = tview.NewFrame(nil).SetBorders(0, 0, 1, 1, 0, 0)
	ret.RedrawFrame()

	// Define the TUI application.
	ret.app = tview.NewApplication().SetScreen(screen).SetRoot(ret.frame, true)

	return ret, nil
}

// Write implements the Writer interface, so we can be passed to slog.NewTextHandler()
// to update both the TUI and stdout with log entries.
func (t *TUI) Write(p []byte) (int, error) {
	s := string(p)

	if strings.Contains(s, "level=WARN") {
		s = "[orange]" + s + "[white]"
	} else if strings.Contains(s, "level=ERROR") {
		s = "[red]" + s + "[white]"
	}

	num, err := fmt.Fprint(t.textView, s)
	if err != nil {
		return num, err
	}

	return fmt.Fprint(os.Stdout, string(p))
}

// Run is a wrapper to start the underlying TUI application.
func (t *TUI) Run() error {
	return t.app.Run()
}

// RedrawFrame clears and completely re-draws the TUI frame. This is necessary when updating
// footer values, such as when an IP address changes.
func (t *TUI) RedrawFrame() {
	if t.frame == nil {
		return
	}

	// Always directly fetch the OS version, since it won't be in the state on first startup.
	incusOSVersion, err := systemd.GetCurrentRelease(context.TODO())
	if err != nil {
		incusOSVersion = "[" + err.Error() + "]"
	}

	// Get list of applications from state.
	applications := []string{}
	s, err := state.LoadOrCreate(context.TODO(), "/var/lib/incus-os/state.json")
	if err == nil {
		for app, info := range s.Applications {
			applications = append(applications, app+"("+info.Version+")")
		}
	}
	slices.Sort(applications)

	t.frame.Clear()
	t.frame.AddText("Incus OS "+incusOSVersion, true, tview.AlignCenter, tcell.ColorWhite)
	t.frame.AddText("[green]IP Address(es):[white] "+strings.Join(getIPAddresses(), ", "), false, tview.AlignLeft, tcell.ColorWhite)
	t.frame.AddText("[green]Installed application(s):[white] "+strings.Join(applications, ", "), false, tview.AlignLeft, tcell.ColorWhite)

	if t.textView != nil {
		t.frame.SetPrimitive(t.textView)
	}
}

func getIPAddresses() []string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return []string{err.Error()}
	}

	ret := []string{}

	for _, addr := range addrs {
		ret = append(ret, addr.String())
	}

	return ret
}
