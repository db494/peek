package tui

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	ec2inst "github.com/db494/peek/internal/ec2"
)

// Palette — all 256-color ANSI for broad terminal support.
var (
	colorBlue     = lipgloss.Color("69")  // cornflower blue  — primary accent
	colorSkyBlue  = lipgloss.Color("111") // light steel blue — header text
	colorTeal     = lipgloss.Color("38")  // dark cyan        — border
	colorLavender = lipgloss.Color("140") // lavender         — profile badge
	colorGray     = lipgloss.Color("246") // medium gray      — help text
	colorDim      = lipgloss.Color("240") // dark gray        — separators
	colorDimmer   = lipgloss.Color("236") // very dark gray   — table border
	colorSelBg    = lipgloss.Color("24")  // dark blue        — selected row bg
	colorSelFg    = lipgloss.Color("255") // white            — selected row fg
)

// Chrome styles.
var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorBlue)

	countStyle = lipgloss.NewStyle().
			Foreground(colorDim)

	tableWrapStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(colorTeal).
			PaddingLeft(1).
			PaddingRight(1).
			MarginTop(1)

	footerStyle = lipgloss.NewStyle().
			Foreground(colorGray).
			MarginTop(1).
			PaddingLeft(2)

	keyStyle = lipgloss.NewStyle().
			Foreground(colorBlue).
			Bold(true)

	sepStyle = lipgloss.NewStyle().
			Foreground(colorDimmer)

	profileBadgeStyle = lipgloss.NewStyle().
				Foreground(colorLavender).
				Bold(true)
)

type model struct {
	table     table.Model
	instances []ec2inst.Instance
	selected  *ec2inst.Instance
	quitting  bool
	profile   string
}

func (m model) Init() tea.Cmd { return nil }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			row := m.table.SelectedRow()
			if row != nil {
				for i, inst := range m.instances {
					if inst.ID == row[1] {
						m.selected = &m.instances[i]
						break
					}
				}
			}
			return m, tea.Quit
		case "ctrl+c", "q":
			m.quitting = true
			return m, tea.Quit
		}

	case tea.WindowSizeMsg:
		m.table.SetHeight(msg.Height - 9)
	}

	var cmd tea.Cmd
	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

func (m model) View() string {
	if m.quitting {
		return ""
	}

	sep := sepStyle.Render("  ·  ")

	// Title row: "SSM Connect  ·  12 instances"
	title := titleStyle.Render("peek 👀") +
		sep +
		countStyle.Render(fmt.Sprintf("%d instance(s)", len(m.instances)))

	// Table wrapped in rounded border with padding.
	tableView := tableWrapStyle.Render(m.table.View())

	// Footer: key hints on left, profile badge on right.
	p := m.profile
	if p == "" {
		p = "default"
	}
	keys := keyStyle.Render("↑↓") + " navigate" +
		sep +
		keyStyle.Render("enter") + " connect" +
		sep +
		keyStyle.Render("q") + " quit"
	badge := "profile: " + profileBadgeStyle.Render(p)
	footer := footerStyle.Render(keys + sep + badge)

	return title + "\n" + tableView + "\n" + footer + "\n"
}

func buildTable(instances []ec2inst.Instance) table.Model {
	cols := []table.Column{
		{Title: "NAME", Width: 30},
		{Title: "ID", Width: 20},
		{Title: "IP", Width: 16},
		{Title: "TYPE", Width: 14},
		{Title: "OS", Width: 20},
		{Title: "AMI", Width: 22},
		{Title: "STATE", Width: 14},
	}

	rows := make([]table.Row, len(instances))
	for i, inst := range instances {
		rows[i] = table.Row{inst.Name, inst.ID, inst.PrivateIP, inst.Type, inst.Platform, inst.AMIID, inst.State}
	}

	t := table.New(
		table.WithColumns(cols),
		table.WithRows(rows),
		table.WithFocused(true),
		table.WithHeight(20),
	)

	s := table.DefaultStyles()

	s.Header = s.Header.
		BorderStyle(lipgloss.ThickBorder()).
		BorderForeground(colorDimmer).
		BorderBottom(true).
		Foreground(colorSkyBlue).
		Bold(true)

	selected := s.Selected.
		Foreground(colorSelFg).
		Background(colorSelBg).
		Bold(true)
	s.Selected = selected

	t.SetStyles(s)
	return t
}

var ErrNoInstances = fmt.Errorf("no instances found")

func Run(ctx context.Context, cfg aws.Config, profile string) (*ec2inst.Instance, error) {
	instances, err := ec2inst.List(ctx, cfg)
	if err != nil {
		return nil, err
	}
	if len(instances) == 0 {
		return nil, ErrNoInstances
	}

	m := model{
		instances: instances,
		table:     buildTable(instances),
		profile:   profile,
	}

	p := tea.NewProgram(m, tea.WithAltScreen())
	result, err := p.Run()
	if err != nil {
		return nil, err
	}

	return result.(model).selected, nil
}
