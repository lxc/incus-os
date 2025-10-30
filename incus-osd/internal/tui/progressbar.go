package tui

/* Copied from https://code.rocket9labs.com/tslocum/cview/src/branch/master/progressbar.go
 * MIT License, Copyright (c) 2020 Trevor Slocum <trevor@rocketnine.space>
 */

import (
	"math"
	"sync"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// ProgressBar indicates the progress of an operation.
type ProgressBar struct {
	*tview.Box
	sync.RWMutex

	// Rune to use when rendering the empty area of the progress bar.
	emptyRune rune

	// Color of the empty area of the progress bar.
	emptyColor tcell.Color

	// Rune to use when rendering the filled area of the progress bar.
	filledRune rune

	// Color of the filled area of the progress bar.
	filledColor tcell.Color

	// If set to true, instead of filling from left to right, the bar is filled
	// from bottom to top.
	vertical bool

	// Current progress.
	progress int64

	// Progress required to fill the bar.
	max int64
}

// NewProgressBar returns a new progress bar.
func NewProgressBar() *ProgressBar {
	p := &ProgressBar{
		Box:         tview.NewBox(),
		emptyRune:   tcell.RuneBlock,
		emptyColor:  tview.Styles.PrimitiveBackgroundColor,
		filledRune:  tcell.RuneBlock,
		filledColor: tview.Styles.PrimaryTextColor,
		max:         100,
	}
	p.SetBackgroundColor(tview.Styles.PrimitiveBackgroundColor)

	return p
}

// SetEmptyRune sets the rune used for the empty area of the progress bar.
func (p *ProgressBar) SetEmptyRune(empty rune) {
	p.Lock()
	defer p.Unlock()

	p.emptyRune = empty
}

// SetEmptyColor sets the color of the empty area of the progress bar.
func (p *ProgressBar) SetEmptyColor(empty tcell.Color) {
	p.Lock()
	defer p.Unlock()

	p.emptyColor = empty
}

// SetFilledRune sets the rune used for the filled area of the progress bar.
func (p *ProgressBar) SetFilledRune(filled rune) {
	p.Lock()
	defer p.Unlock()

	p.filledRune = filled
}

// SetFilledColor sets the color of the filled area of the progress bar.
func (p *ProgressBar) SetFilledColor(filled tcell.Color) {
	p.Lock()
	defer p.Unlock()

	p.filledColor = filled
}

// SetVertical sets the direction of the progress bar.
func (p *ProgressBar) SetVertical(vertical bool) {
	p.Lock()
	defer p.Unlock()

	p.vertical = vertical
}

// SetMax sets the progress required to fill the bar.
func (p *ProgressBar) SetMax(maxVal int64) {
	p.Lock()
	defer p.Unlock()

	p.max = maxVal
}

// GetMax returns the progress required to fill the bar.
func (p *ProgressBar) GetMax() int64 {
	p.RLock()
	defer p.RUnlock()

	return p.max
}

// AddProgress adds to the current progress.
func (p *ProgressBar) AddProgress(progress int64) {
	p.Lock()
	defer p.Unlock()

	p.progress += progress
	if p.progress < 0 {
		p.progress = 0
	} else if p.progress > p.max {
		p.progress = p.max
	}
}

// SetProgress sets the current progress.
func (p *ProgressBar) SetProgress(progress int64) {
	p.Lock()
	defer p.Unlock()

	p.progress = progress
	if p.progress < 0 {
		p.progress = 0
	} else if p.progress > p.max {
		p.progress = p.max
	}
}

// GetProgress gets the current progress.
func (p *ProgressBar) GetProgress() int64 {
	p.RLock()
	defer p.RUnlock()

	return p.progress
}

// Complete returns whether the progress bar has been filled.
func (p *ProgressBar) Complete() bool {
	p.RLock()
	defer p.RUnlock()

	return p.progress >= p.max
}

// Draw draws this primitive onto the screen.
func (p *ProgressBar) Draw(screen tcell.Screen) {
	p.Box.Draw(screen)

	p.Lock()
	defer p.Unlock()

	x, y, width, height := p.GetInnerRect()

	barSize := height

	maxLength := width
	if p.vertical {
		barSize = width
		maxLength = height
	}

	barLength := min(int(math.RoundToEven(float64(maxLength)*(float64(p.progress)/float64(p.max)))), maxLength)

	for i := range barSize {
		for j := range barLength {
			if p.vertical {
				screen.SetContent(x+i, y+(height-1-j), p.filledRune, nil, tcell.StyleDefault.Foreground(p.filledColor).Background(p.GetBackgroundColor()))
			} else {
				screen.SetContent(x+j, y+i, p.filledRune, nil, tcell.StyleDefault.Foreground(p.filledColor).Background(p.GetBackgroundColor()))
			}
		}

		for j := barLength; j < maxLength; j++ {
			if p.vertical {
				screen.SetContent(x+i, y+(height-1-j), p.emptyRune, nil, tcell.StyleDefault.Foreground(p.emptyColor).Background(p.GetBackgroundColor()))
			} else {
				screen.SetContent(x+j, y+i, p.emptyRune, nil, tcell.StyleDefault.Foreground(p.emptyColor).Background(p.GetBackgroundColor()))
			}
		}
	}
}
