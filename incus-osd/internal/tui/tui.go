package tui

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/lxc/incus/v6/shared/subprocess"
	"github.com/rivo/tview"

	"github.com/lxc/incus-os/incus-osd/internal/state"
)

// TUI represents a terminal user interface.
type TUI struct {
	app      *tview.Application
	frame    *tview.Frame
	pages    *tview.Pages
	screen   tcell.Screen
	textView *tview.TextView

	state *state.State
}

// NewTUI constructs a new TUI application that will show basic information and recent
// log entries on the system's console.
func NewTUI(s *state.State) (*TUI, error) {
	ret := &TUI{
		state: s,
	}

	// Attempt to open the system's consoles.
	ttys, err := newTtyMultiplexer("/dev/console", "/dev/tty1", "/dev/tty2", "/dev/tty3", "/dev/tty4", "/dev/tty5", "/dev/tty6", "/dev/tty7", "/dev/ttyS0")
	if err != nil {
		return ret, err
	}

	// Construct a screen that is bound to the system console.
	ret.screen, err = tcell.NewTerminfoScreenFromTty(ttys)
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

	// Define a frame to hold the TUI's primary content.
	ret.frame = tview.NewFrame(nil).SetBorders(0, 0, 1, 1, 0, 0)

	// Define a set of pages so we can present modal popups.
	ret.pages = tview.NewPages().AddPage("frame", ret.frame, true, true)

	// Define the TUI application.
	ret.app = tview.NewApplication().SetScreen(ret.screen).SetRoot(ret.pages, true)

	return ret, nil
}

// Write implements the Writer interface, so we can be passed to slog.NewTextHandler()
// to update both the TUI and stdout with log entries.
func (t *TUI) Write(p []byte) (int, error) {
	s := string(p)

	// Colorize any warning or error level messages.
	if strings.Contains(s, " WARN:") {
		s = "[orange]" + s + "[white]"
	} else if strings.Contains(s, " ERROR:") {
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

			t.redrawScreen()
			time.Sleep(5 * time.Second)
		}
	}()

	return t.app.Run()
}

// DisplayModal displays a centered popup dialog. Optionally, if maxProgress is greater than zero,
// renders a progress bar at the bottom.
func (t *TUI) DisplayModal(title string, msg string, progress int64, maxProgress int64) {
	// Returns a new primitive which puts the provided primitive in the center and
	// sets its size to the given width and height.
	modal := func(p tview.Primitive, width, height int) tview.Primitive {
		return tview.NewFlex().
			AddItem(nil, 0, 1, false).
			AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
				AddItem(nil, 0, 1, false).
				AddItem(p, height, 1, true).
				AddItem(nil, 0, 1, false), width, 1, true).
			AddItem(nil, 0, 1, false)
	}

	// Calculate width and height for modal dialog.
	consoleWidth, consoleHeight := t.screen.Size()
	modalWidth := consoleWidth * 3 / 4
	modalHeight := consoleHeight / 2

	// Setup a text view to display the message.
	textView := tview.NewTextView().
		SetText(msg).
		SetDynamicColors(true).
		SetScrollable(false).
		SetWordWrap(true)

	// Setup a grid to show the text area and possibly a progress bar.
	grid := tview.NewGrid().
		SetColumns(0).
		SetRows(modalHeight-4).
		SetBorders(true).
		AddItem(textView, 0, 0, 1, 1, 0, 0, false)

	// If a maximum value is provided, display the progress bar.
	if maxProgress > 0 {
		progressBar := NewProgressBar()
		progressBar.SetMax(maxProgress)
		progressBar.SetProgress(progress)
		grid.SetRows(modalHeight-6, 1).AddItem(progressBar, 1, 0, 1, 1, 0, 0, false)
	}

	grid.SetTitle(title).SetBorder(true)

	t.pages.AddPage("modal", modal(grid, modalWidth, modalHeight), true, true)
	t.app.Draw()
}

// RemoveModal hides the modal popup.
func (t *TUI) RemoveModal() {
	t.pages.RemovePage("modal")
}

// redrawScreen clears and completely re-draws the TUI frame. This is necessary when updating
// header or footer values, such as showing the current time.
func (t *TUI) redrawScreen() {
	if t.frame == nil {
		return
	}

	// Get list of applications from state.
	applications := []string{}
	for app, info := range t.state.Applications {
		applications = append(applications, app+"("+info.Version+")")
	}
	slices.Sort(applications)

	t.frame.Clear()

	t.frame.AddText("Incus OS "+t.state.RunningRelease, true, tview.AlignCenter, tcell.ColorWhite)
	t.frame.AddText(time.Now().UTC().Format("2006-01-02 15:04 UTC"), true, tview.AlignRight, tcell.ColorWhite)

	consoleWidth, _ := t.screen.Size()
	for _, line := range wrapFooterText("Network configuration", strings.Join(t.getIPAddresses(), ", "), consoleWidth) {
		t.frame.AddText(line, false, tview.AlignLeft, tcell.ColorWhite)
	}
	for _, line := range wrapFooterText("Installed application(s)", strings.Join(applications, ", "), consoleWidth) {
		t.frame.AddText(line, false, tview.AlignLeft, tcell.ColorWhite)
	}

	if !t.state.System.Encryption.State.RecoveryKeysRetrieved {
		t.frame.AddText("WARNING: Encryption recovery key has not been retrieved yet!", false, tview.AlignLeft, tcell.ColorRed)
	}

	if t.textView != nil {
		t.frame.SetPrimitive(t.textView)
	}

	t.app.Draw()
}

// Return a list of IP addresses for configured interfaces.
func (t *TUI) getIPAddresses() []string {
	if t.state.System.Network.Config == nil {
		return []string{}
	}

	ipAddressRegex := regexp.MustCompile(`inet6? (.+)/\d+ `)
	ret := []string{}

	appendIPs := func(name string) {
		output, err := subprocess.RunCommandContext(context.Background(), "ip", "address", "show", name)
		if err != nil {
			ret = append(ret, name+"("+err.Error()+")")

			return
		}

		addrs := []string{}
		matches := ipAddressRegex.FindAllStringSubmatch(output, -1)

		for _, addr := range matches {
			// Don't show link-local address.
			if strings.HasPrefix(addr[1], "fe80:") {
				continue
			}

			addrs = append(addrs, addr[1])
		}

		ret = append(ret, name+"("+strings.Join(addrs, ", ")+")")
	}

	for _, i := range t.state.System.Network.Config.Interfaces {
		if len(i.Addresses) > 0 {
			appendIPs(i.Name)
		}
	}

	for _, b := range t.state.System.Network.Config.Bonds {
		if len(b.Addresses) > 0 {
			appendIPs(b.Name)
		}
	}

	for _, v := range t.state.System.Network.Config.VLANs {
		if len(v.Addresses) > 0 {
			appendIPs(v.Name)
		}
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
