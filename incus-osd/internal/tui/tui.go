package tui

import (
	"context"
	"fmt"
	"net"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/lxc/incus-os/incus-osd/internal/state"
	"github.com/lxc/incus-os/incus-osd/internal/systemd"
)

// TUI represents a terminal user interface.
type TUI struct {
	app      *tview.Application
	frame    *tview.Frame
	screen   tcell.Screen
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
	ret.screen, err = tcell.NewTerminfoScreenFromTty(tty)
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

	// Define the TUI application.
	ret.app = tview.NewApplication().SetScreen(ret.screen).SetRoot(ret.frame, true)

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
	// Setup a gofunc to periodically re-draw the screen.
	go func() {
		for i := 0; ; i++ {
			// When the daemon starts up, several log messages from systemd are
			// also written to the console. Once a minute forcefully clear the
			// entire console prior to drawing the TUI.
			if i%12 == 1 {
				// Send "ESC c" sequence to console.
				_ = os.WriteFile("/dev/console", []byte{0x1B, 0x63}, 0o600)
			}
			t.RedrawScreen()
			time.Sleep(5 * time.Second)
		}
	}()

	return t.app.Run()
}

// RedrawScreen clears and completely re-draws the TUI frame. This is necessary when updating
// header or footer values, such as showing the current time.
func (t *TUI) RedrawScreen() {
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
	t.frame.AddText(time.Now().UTC().Format("2006-01-02 15:04 UTC"), true, tview.AlignRight, tcell.ColorWhite)

	consoleWidth, _ := t.screen.Size()
	for _, line := range wrapFooterText("IP Address(es)", strings.Join(getIPAddresses(), ", "), consoleWidth) {
		t.frame.AddText(line, false, tview.AlignLeft, tcell.ColorWhite)
	}
	for _, line := range wrapFooterText("Installed application(s)", strings.Join(applications, ", "), consoleWidth) {
		t.frame.AddText(line, false, tview.AlignLeft, tcell.ColorWhite)
	}

	if t.textView != nil {
		t.frame.SetPrimitive(t.textView)
	}

	t.app.Draw()
}

func getIPAddresses() []string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return []string{err.Error()}
	}

	ret := []string{}

	for _, addr := range addrs {
		// Skip empty, local, and link-local addresses.
		if addr.String() == "" || addr.String() == "127.0.0.1/8" || addr.String() == "::1/128" || strings.HasPrefix(addr.String(), "fe80:") {
			continue
		}

		ret = append(ret, addr.String())
	}

	return ret
}

// Performs a very basic text wrapping at a given maximum length, only on spaces. Returns a
// reversed array, since that is how the frame's footer logic expects things.
func wrapFooterText(label string, text string, maxLineLength int) []string {
	ret := []string{}

	currentLine := "[green]" + label + ":[white] "
	currentLen := len(label) + 2

	for _, word := range strings.Split(text, " ") {
		if currentLen+len(word) > maxLineLength {
			ret = append(ret, currentLine)
			currentLine = ""
			currentLen = 0
		}

		currentLine += word + " "
		currentLen += len(word) + 1
	}

	if len(currentLine) > 0 {
		ret = append(ret, currentLine)
	}
	slices.Reverse(ret)

	return ret
}
