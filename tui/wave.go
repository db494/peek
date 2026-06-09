package tui

import (
	"math"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/lucasb-eyer/go-colorful"
)

// The title animation: each character gets a hue offset by its position
// while the whole gradient drifts over time, so a rainbow appears to flow
// through the text.
const (
	waveFPS        = 12
	waveHueStep    = 18.0 // degrees of hue between adjacent characters
	waveHueSpeed   = 14.0 // degrees of hue drift per frame
	waveSaturation = 0.75
	waveLightness  = 0.65
)

type waveTickMsg struct{}

func waveTick() tea.Cmd {
	return tea.Tick(time.Second/waveFPS, func(time.Time) tea.Msg {
		return waveTickMsg{}
	})
}

// waveTitle renders text with a flowing rainbow gradient for the given
// animation frame.
func waveTitle(text string, frame int) string {
	var b strings.Builder
	for i, r := range []rune(text) {
		hue := math.Mod(float64(frame)*waveHueSpeed+float64(i)*waveHueStep, 360)
		c := colorful.Hsl(hue, waveSaturation, waveLightness)
		b.WriteString(lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(c.Hex())).
			Render(string(r)))
	}
	return b.String()
}
