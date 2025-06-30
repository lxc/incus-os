package tui

import (
	"errors"

	"github.com/gdamore/tcell/v2"
)

type ttyMultiplexer struct {
	ttys []tcell.Tty
}

// newTtyMultiplexer takes a list of terminal devices that should be multiplexed to display
// output. The first tty will be considered the primary tty and used for determining properties
// such as screen size and any attempt to read bytes from a tty.
func newTtyMultiplexer(ttys ...string) (ttyMultiplexer, error) {
	ret := ttyMultiplexer{}

	if len(ttys) == 0 {
		return ret, errors.New("at least one tty must be provided")
	}

	for _, tty := range ttys {
		ttyDev, err := tcell.NewDevTtyFromDev(tty)
		if err != nil {
			continue
		}

		ret.ttys = append(ret.ttys, ttyDev)
	}

	return ret, nil
}

// Start activates all ttys. Part of the Tty interface.
func (t ttyMultiplexer) Start() error {
	for _, tty := range t.ttys {
		err := tty.Start()
		if err != nil {
			return err
		}
	}

	return nil
}

// Stop stops all ttys. Part of the Tty interface.
func (t ttyMultiplexer) Stop() error {
	for _, tty := range t.ttys {
		err := tty.Stop()
		if err != nil {
			return err
		}
	}

	return nil
}

// Drain drains all ttys. Part of the Tty interface.
func (t ttyMultiplexer) Drain() error {
	for _, tty := range t.ttys {
		err := tty.Drain()
		if err != nil {
			return err
		}
	}

	return nil
}

// NotifyResize registers a callback when the tty is resized; only applied
// to the primary tty. Part of the Tty interface.
func (t ttyMultiplexer) NotifyResize(cb func()) {
	t.ttys[0].NotifyResize(cb)
}

// WindowSize gets the terminal dimensions of the primary tty. Part of the Tty interface.
func (t ttyMultiplexer) WindowSize() (tcell.WindowSize, error) {
	return t.ttys[0].WindowSize()
}

// Read will read bytes from the primary terminal. Part of the ReadWriteCloser interface.
func (t ttyMultiplexer) Read(p []byte) (int, error) {
	return t.ttys[0].Read(p)
}

// Write will write bytes to all terminals. Part of the ReadWriteCloser interface.
func (t ttyMultiplexer) Write(p []byte) (int, error) {
	var n int

	var err error

	for _, tty := range t.ttys {
		n, err = tty.Write(p)
		if err != nil {
			return n, err
		}
	}

	return n, err
}

// Close will close all terminals. Part of the ReadWriteCloser interface.
func (t ttyMultiplexer) Close() error {
	for _, tty := range t.ttys {
		err := tty.Close()
		if err != nil {
			return err
		}
	}

	return nil
}
