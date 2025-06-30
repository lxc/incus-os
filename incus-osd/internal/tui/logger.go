package tui

import (
	"context"
	"io"
	"log/slog"
	"sort"
	"strings"
	"time"
)

// CustomTextHandler extends the slog.Handler struct to provide more compact text logging.
type CustomTextHandler struct {
	slog.Handler

	w io.Writer
}

// NewCustomTextHandler returns an instance of the CustomTextHandler ready for use.
func NewCustomTextHandler(out io.Writer) *CustomTextHandler {
	cth := &CustomTextHandler{
		w: out,
	}

	return cth
}

// Enabled reports whether the handler handles records at the given level.
func (*CustomTextHandler) Enabled(_ context.Context, level slog.Level) bool {
	// We hide debug messages from the UI.
	if level == slog.LevelDebug {
		return false
	}

	return true
}

// Handle handles the Record.
func (cth *CustomTextHandler) Handle(_ context.Context, r slog.Record) error {
	var buf strings.Builder

	var err error

	// Build up the base line with timestamp, log level, and message.
	_, err = buf.WriteString(r.Time.Format(time.DateTime) + " ")
	if err != nil {
		return err
	}

	levelColor := ""

	switch r.Level {
	case slog.LevelDebug:
		levelColor = "[blue]"
	case slog.LevelInfo:
		levelColor = "[green]"
	case slog.LevelWarn:
		levelColor = "[yellow]"
	case slog.LevelError:
		levelColor = "[red]"
	}

	_, err = buf.WriteString(levelColor + r.Level.String() + "[white] ")
	if err != nil {
		return err
	}

	_, err = buf.WriteString(r.Message)
	if err != nil {
		return err
	}

	// Append any attributes.
	if r.NumAttrs() > 0 {
		// Get the attributes for this record.
		attrs := make(map[string]string, r.NumAttrs())
		r.Attrs(func(a slog.Attr) bool {
			attrs[a.Key] = a.Value.String()

			return true
		})

		// Sort the keys so we have a consistent output.
		keys := make([]string, 0, r.NumAttrs())
		for k := range attrs {
			keys = append(keys, k)
		}

		sort.Strings(keys)

		_, err = buf.WriteString("[purple]")
		if err != nil {
			return err
		}

		// Append "k=v" to the log message.
		for _, k := range keys {
			_, err = buf.WriteString(" " + k + "=" + attrs[k])
			if err != nil {
				return err
			}
		}

		_, err = buf.WriteString("[white]")
		if err != nil {
			return err
		}
	}

	// Add a trailing newline.
	_, err = buf.WriteString("\n")
	if err != nil {
		return err
	}

	// Write the line.
	_, err = cth.w.Write([]byte(buf.String()))

	return err
}
