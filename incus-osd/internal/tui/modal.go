package tui

// Modal holds the information for a given modal dialog.
type Modal struct {
	title    string
	message  string
	progress float64
	isDone   bool

	t *TUI
}

// Update sets the current message of the modal dialog.
func (m *Modal) Update(message string) {
	m.message = message

	m.t.quickDraw()
}

// UpdateProgress sets the current modal's progress, expressed as a float between 0 and 1.
func (m *Modal) UpdateProgress(progress float64) {
	m.progress = progress

	m.t.quickDraw()
}

// Done indicates that the modal is no longer needed and should be removed.
func (m *Modal) Done() {
	m.isDone = true

	m.t.quickDraw()
}
