package tui

import (
	"fmt"
	"io"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// strItem adapts a plain string to the list.Item interface.
type strItem string

func (s strItem) FilterValue() string { return string(s) }

var (
	itemStyle         = lipgloss.NewStyle().PaddingLeft(2).Foreground(colorDim)
	selectedItemStyle = lipgloss.NewStyle().Foreground(colorAccent).Bold(true)
)

// strDelegate renders one-line string items with a selection marker.
type strDelegate struct{}

func (d strDelegate) Height() int                         { return 1 }
func (d strDelegate) Spacing() int                        { return 0 }
func (d strDelegate) Update(tea.Msg, *list.Model) tea.Cmd { return nil }
func (d strDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	s, ok := item.(strItem)
	if !ok {
		return
	}
	if index == m.Index() {
		fmt.Fprint(w, selectedItemStyle.Render("❯ "+string(s)))
	} else {
		fmt.Fprint(w, itemStyle.Render(string(s)))
	}
}

// newPickList builds a simple one-line-per-item selection list with the
// built-in fuzzy filter enabled.
func newPickList(title string, values []string) list.Model {
	items := make([]list.Item, len(values))
	for i, v := range values {
		items[i] = strItem(v)
	}
	l := list.New(items, strDelegate{}, 80, 20)
	l.Title = "› " + title
	l.Styles.Title = titleStyle
	l.SetShowStatusBar(false)
	l.SetShowHelp(true)
	l.DisableQuitKeybindings() // we handle q/ctrl+c ourselves
	return l
}

// Fixed column widths; the Name column absorbs whatever space remains.
const (
	colWidthID = 20
	// Wide enough for the longest state name (8) plus the fixed phantom
	// width of its color escapes (14) — see styleState.
	colWidthState   = 22
	colWidthIP      = 16
	tableChromeCols = 12 // borders + per-column padding
)

func newInstanceTable() table.Model {
	t := table.New(
		table.WithColumns(instanceColumns(80)),
		table.WithFocused(true),
		table.WithHeight(15),
	)
	styles := table.DefaultStyles()
	styles.Header = styles.Header.
		Bold(true).
		Foreground(colorDim).
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(colorBorder).
		BorderBottom(true)
	// A quiet teal-tinted bar rather than a loud highlight; the state cell
	// keeps its own color on top of it (see styleState).
	styles.Selected = styles.Selected.
		Foreground(colorText).
		Background(lipgloss.Color("#1E3A32")).
		Bold(true)
	t.SetStyles(styles)
	return t
}

func instanceColumns(totalWidth int) []table.Column {
	nameWidth := max(totalWidth-colWidthID-colWidthState-2*colWidthIP-tableChromeCols, 12)
	return []table.Column{
		{Title: "ID", Width: colWidthID},
		{Title: "Name", Width: nameWidth},
		{Title: "State", Width: colWidthState},
		{Title: "Private IP", Width: colWidthIP},
		{Title: "Public IP", Width: colWidthIP},
	}
}

// sizeTable fits the table to the current terminal, leaving room for the
// header, filter line, and help bar.
func (m *Model) sizeTable() {
	m.table.SetColumns(instanceColumns(m.width))
	m.table.SetWidth(max(m.width-2, 40))
	m.table.SetHeight(max(m.height-8, 3))
}
