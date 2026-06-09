package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func (m Model) View() string {
	switch m.state {
	case statePickProfile:
		return m.profileList.View()
	case stateLoading:
		return m.viewLoading()
	case stateInstances:
		return m.viewInstances()
	case statePickRegion:
		return m.regionList.View()
	case stateError:
		return m.viewError()
	}
	return ""
}

func (m Model) viewLoading() string {
	return fmt.Sprintf("\n %s %s\n", m.spinner.View(), contextStyle.Render(m.loadingText))
}

func (m Model) viewInstances() string {
	var b strings.Builder

	header := waveTitle("› peek", m.waveFrame) + "  " +
		contextStyle.Render(fmt.Sprintf("profile: %s · region: %s · %d/%d instances",
			m.profile, m.region, len(m.visible), len(m.instances)))
	b.WriteString(header + "\n")

	if m.filtering || m.filterInput.Value() != "" {
		b.WriteString(m.filterInput.View() + "\n")
	} else {
		b.WriteString("\n")
	}

	if len(m.instances) == 0 {
		empty := contextStyle.Render(fmt.Sprintf("\n  No instances found in %s.\n  Press r to try another region, or q to quit.\n", m.region))
		b.WriteString(empty)
	} else {
		b.WriteString(tableBorderStyle.Render(m.table.View()) + "\n")
	}

	if m.notice != "" {
		b.WriteString(noticeStyle.Render("⚠ "+m.notice) + "\n")
	}

	help := "enter connect · / filter · r region · q quit"
	if m.filtering {
		help = "enter apply filter · esc clear filter"
	}
	b.WriteString(helpStyle.Render(help))
	return b.String()
}

func (m Model) viewError() string {
	body := errorTitleStyle.Render("✗ Something went wrong") + "\n\n" +
		wordwrap(m.err.Error(), max(m.width-10, 30)) + "\n\n" +
		helpStyle.Render("press q to exit")
	box := errorBoxStyle.Render(body)
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}

// wordwrap breaks a string at word boundaries so error messages stay inside
// the error box.
func wordwrap(s string, width int) string {
	words := strings.Fields(s)
	if len(words) == 0 {
		return s
	}
	var b strings.Builder
	lineLen := 0
	for i, w := range words {
		if i > 0 {
			if lineLen+1+len(w) > width {
				b.WriteString("\n")
				lineLen = 0
			} else {
				b.WriteString(" ")
				lineLen++
			}
		}
		b.WriteString(w)
		lineLen += len(w)
	}
	return b.String()
}
