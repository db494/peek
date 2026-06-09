package tui

import "github.com/charmbracelet/lipgloss"

// Palette: one teal accent over neutral grays, in the spirit of OpenAI
// Codex — minimal chrome, color only where it carries meaning.
var (
	colorAccent = lipgloss.Color("#10A37F") // OpenAI teal
	colorText   = lipgloss.Color("#ECECEC")
	colorDim    = lipgloss.Color("#9A9A9A")
	colorFaint  = lipgloss.Color("#6B6B6B")
	colorBorder = lipgloss.Color("#3F3F3F")
	colorAmber  = lipgloss.Color("#E5C07B")
	colorRed    = lipgloss.Color("#E06C75")
)

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorAccent)

	contextStyle = lipgloss.NewStyle().
			Foreground(colorDim)

	helpStyle = lipgloss.NewStyle().
			Foreground(colorFaint)

	noticeStyle = lipgloss.NewStyle().
			Foreground(colorAmber)

	errorTitleStyle = lipgloss.NewStyle().
			Foreground(colorRed).
			Bold(true)

	errorBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorRed).
			Padding(1, 2)

	spinnerStyle = lipgloss.NewStyle().
			Foreground(colorAccent)

	filterPromptStyle = lipgloss.NewStyle().
				Foreground(colorAccent).
				Bold(true)

	tableBorderStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(colorBorder)

	// Instance state colors (256-color indexes), keyed by EC2 state name.
	// Soft variants rather than pure terminal colors.
	stateColors = map[string]string{
		"running":  "114", // soft green
		"pending":  "221", // soft yellow
		"stopping": "209", // soft orange
		"stopped":  "245", // mid gray
	}
)

// styleState colorizes an EC2 state name for use inside a table cell.
//
// The escape sequence is built by hand rather than with lipgloss because
// bubbles/table truncates cells with go-runewidth, which counts the
// printable bytes of an escape sequence as visible width. Hand-building
// keeps that phantom width fixed (at most 14 cells: "[38;5;NNNm" + "[39m")
// so the State column width can simply absorb it. The trailing sequence
// resets only the foreground, so the selected-row background survives
// across the rest of the row.
func styleState(state string) string {
	c, ok := stateColors[state]
	if !ok {
		return state
	}
	return "\x1b[38;5;" + c + "m" + state + "\x1b[39m"
}
