// Package tui implements the interactive terminal UI: profile selection,
// instance browsing, region switching, and the handoff selection result.
package tui

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"peek/aws"
)

// state identifies which screen the model is showing. Transitions:
//
//	statePickProfile -> stateLoading -> stateInstances <-> statePickRegion
//	any state -> stateError (on a failed AWS call)
type state int

const (
	statePickProfile state = iota
	stateLoading
	stateInstances
	statePickRegion
	stateError
)

// Messages produced by asynchronous AWS commands.
type (
	clientReadyMsg struct{ client *aws.Client }
	instancesMsg   struct{ instances []aws.Instance }
	regionsMsg     struct{ regions []string }
	errMsg         struct{ err error }
)

// Config controls how the model starts up.
type Config struct {
	Profiles []string // all detected profiles (for the picker)
	Profile  string   // preselected profile ("" -> show picker)
	Region   string   // region override from --region ("" -> profile default)
	Demo     bool     // use fake data instead of AWS calls
}

// Model is the root Bubble Tea model.
type Model struct {
	state state
	cfg   Config

	client  *aws.Client
	profile string
	region  string

	// Selected is the instance the user chose to connect to. It is nil if
	// the user quit without selecting; main reads it after Run returns.
	Selected *aws.Instance

	spinner     spinner.Model
	loadingText string

	table     table.Model
	instances []aws.Instance // all fetched instances
	visible   []aws.Instance // instances currently shown (after filtering)

	filterInput textinput.Model
	filtering   bool // filter input has focus

	profileList list.Model
	regionList  list.Model

	notice string
	err    error

	waveFrame int // animation frame for the title gradient

	width  int
	height int
}

// New builds the root model. The initial state depends on whether a profile
// is already known (flag, env var, or only one configured).
func New(cfg Config) Model {
	sp := spinner.New(spinner.WithSpinner(spinner.Dot), spinner.WithStyle(spinnerStyle))

	fi := textinput.New()
	fi.Prompt = filterPromptStyle.Render("/ ")
	fi.Placeholder = "filter by id, name, ip…"
	fi.CharLimit = 64

	m := Model{
		cfg:         cfg,
		profile:     cfg.Profile,
		region:      cfg.Region,
		spinner:     sp,
		filterInput: fi,
		table:       newInstanceTable(),
		width:       80,
		height:      24,
	}

	switch {
	case cfg.Demo:
		m.state = stateInstances
		if m.region == "" {
			m.region = "us-east-1"
		}
		m.profile = "demo"
		m.setInstances(demoInstances())
	case cfg.Profile != "":
		m.state = stateLoading
		m.loadingText = fmt.Sprintf("Authenticating profile %q…", cfg.Profile)
	default:
		m.state = statePickProfile
		m.profileList = newPickList("Select an AWS profile", cfg.Profiles)
	}
	return m
}

func (m Model) Init() tea.Cmd {
	switch m.state {
	case stateLoading:
		return tea.Batch(waveTick(), m.spinner.Tick, initClientCmd(m.profile, m.cfg.Region))
	default:
		return waveTick()
	}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.resize()
		return m, nil

	case tea.KeyMsg:
		if msg.Type == tea.KeyCtrlC {
			return m, tea.Quit
		}

	case waveTickMsg:
		m.waveFrame++
		return m, waveTick()

	case spinner.TickMsg:
		if m.state == stateLoading {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}
		return m, nil

	case clientReadyMsg:
		m.client = msg.client
		m.region = msg.client.Region
		m.loadingText = fmt.Sprintf("Fetching instances in %s…", m.region)
		return m, fetchInstancesCmd(m.client)

	case instancesMsg:
		m.setInstances(msg.instances)
		m.state = stateInstances
		return m, nil

	case regionsMsg:
		m.regionList = newPickList("Select a region", msg.regions)
		m.state = statePickRegion
		m.resize()
		return m, nil

	case errMsg:
		m.err = msg.err
		m.state = stateError
		return m, nil
	}

	switch m.state {
	case statePickProfile:
		return m.updatePickProfile(msg)
	case stateInstances:
		return m.updateInstances(msg)
	case statePickRegion:
		return m.updatePickRegion(msg)
	case stateError:
		return m.updateError(msg)
	}
	return m, nil
}

func (m Model) updatePickProfile(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok && m.profileList.FilterState() != list.Filtering {
		switch key.String() {
		case "q":
			return m, tea.Quit
		case "enter":
			if item, ok := m.profileList.SelectedItem().(strItem); ok {
				m.profile = string(item)
				m.state = stateLoading
				m.loadingText = fmt.Sprintf("Authenticating profile %q…", m.profile)
				return m, tea.Batch(m.spinner.Tick, initClientCmd(m.profile, m.cfg.Region))
			}
			return m, nil
		}
	}
	var cmd tea.Cmd
	m.profileList, cmd = m.profileList.Update(msg)
	return m, cmd
}

func (m Model) updateInstances(msg tea.Msg) (tea.Model, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		var cmd tea.Cmd
		m.table, cmd = m.table.Update(msg)
		return m, cmd
	}
	m.notice = ""

	// While the filter input has focus, it captures every key except the
	// ones that leave filter mode.
	if m.filtering {
		switch key.String() {
		case "enter":
			m.filtering = false
			m.filterInput.Blur()
		case "esc":
			m.filtering = false
			m.filterInput.Blur()
			m.filterInput.SetValue("")
			m.applyFilter()
		default:
			var cmd tea.Cmd
			m.filterInput, cmd = m.filterInput.Update(msg)
			m.applyFilter()
			return m, cmd
		}
		return m, nil
	}

	switch key.String() {
	case "q":
		return m, tea.Quit
	case "/":
		m.filtering = true
		m.filterInput.Focus()
		return m, textinput.Blink
	case "esc":
		if m.filterInput.Value() != "" {
			m.filterInput.SetValue("")
			m.applyFilter()
		}
		return m, nil
	case "r":
		if m.cfg.Demo {
			return m, func() tea.Msg { return regionsMsg{regions: demoRegions()} }
		}
		m.state = stateLoading
		m.loadingText = "Fetching regions…"
		return m, tea.Batch(m.spinner.Tick, fetchRegionsCmd(m.client))
	case "enter":
		cursor := m.table.Cursor()
		if cursor < 0 || cursor >= len(m.visible) {
			return m, nil
		}
		inst := m.visible[cursor]
		if inst.State != "running" {
			m.notice = fmt.Sprintf("%s is %s — only running instances can be connected to", inst.ID, inst.State)
			return m, nil
		}
		m.Selected = &inst
		return m, tea.Quit
	}

	var cmd tea.Cmd
	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

func (m Model) updatePickRegion(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok && m.regionList.FilterState() != list.Filtering {
		switch key.String() {
		case "q":
			return m, tea.Quit
		case "esc":
			m.state = stateInstances
			return m, nil
		case "enter":
			if item, ok := m.regionList.SelectedItem().(strItem); ok {
				region := string(item)
				if m.cfg.Demo {
					m.region = region
					m.setInstances(demoInstances())
					m.state = stateInstances
					return m, nil
				}
				m.client = m.client.WithRegion(region)
				m.region = region
				m.state = stateLoading
				m.loadingText = fmt.Sprintf("Fetching instances in %s…", region)
				return m, tea.Batch(m.spinner.Tick, fetchInstancesCmd(m.client))
			}
			return m, nil
		}
	}
	var cmd tea.Cmd
	m.regionList, cmd = m.regionList.Update(msg)
	return m, cmd
}

func (m Model) updateError(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "q", "esc", "enter":
			return m, tea.Quit
		}
	}
	return m, nil
}

// setInstances replaces the instance set and rebuilds the table through the
// current filter.
func (m *Model) setInstances(instances []aws.Instance) {
	m.instances = instances
	m.applyFilter()
}

// applyFilter rebuilds the visible rows from the filter input's value,
// matching case-insensitively against ID, name, state, and IPs.
func (m *Model) applyFilter() {
	query := strings.ToLower(strings.TrimSpace(m.filterInput.Value()))
	m.visible = m.visible[:0]
	for _, inst := range m.instances {
		if query == "" || matchesFilter(inst, query) {
			m.visible = append(m.visible, inst)
		}
	}

	rows := make([]table.Row, len(m.visible))
	for i, inst := range m.visible {
		rows[i] = table.Row{inst.ID, inst.Name, styleState(inst.State), orDash(inst.PrivateIP), orDash(inst.PublicIP)}
	}
	m.table.SetRows(rows)
	if m.table.Cursor() >= len(rows) {
		m.table.SetCursor(0)
	}
}

func matchesFilter(inst aws.Instance, query string) bool {
	for _, field := range []string{inst.ID, inst.Name, inst.State, inst.PrivateIP, inst.PublicIP} {
		if strings.Contains(strings.ToLower(field), query) {
			return true
		}
	}
	return false
}

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

// resize propagates the terminal size to whichever components exist.
func (m *Model) resize() {
	listHeight := max(m.height-4, 5)
	if m.profileList.Title != "" {
		m.profileList.SetSize(m.width, listHeight)
	}
	if m.regionList.Title != "" {
		m.regionList.SetSize(m.width, listHeight)
	}
	m.sizeTable()
}

// Asynchronous AWS commands. Each returns a typed message consumed in Update.

func initClientCmd(profile, region string) tea.Cmd {
	return func() tea.Msg {
		client, err := aws.NewClient(context.Background(), profile, region)
		if err != nil {
			return errMsg{err}
		}
		return clientReadyMsg{client: client}
	}
}

func fetchInstancesCmd(client *aws.Client) tea.Cmd {
	return func() tea.Msg {
		instances, err := client.Instances(context.Background())
		if err != nil {
			return errMsg{err}
		}
		return instancesMsg{instances: instances}
	}
}

func fetchRegionsCmd(client *aws.Client) tea.Cmd {
	return func() tea.Msg {
		regions, err := client.Regions(context.Background())
		if err != nil {
			return errMsg{err}
		}
		return regionsMsg{regions: regions}
	}
}

// Profile returns the profile in effect when the program exited.
func (m Model) Profile() string { return m.profile }

// Region returns the region in effect when the program exited.
func (m Model) Region() string { return m.region }

func demoInstances() []aws.Instance {
	return []aws.Instance{
		{ID: "i-0a1b2c3d4e5f60001", Name: "web-server-1", State: "running", PrivateIP: "10.0.1.12", PublicIP: "54.12.3.4"},
		{ID: "i-0a1b2c3d4e5f60002", Name: "web-server-2", State: "running", PrivateIP: "10.0.1.13", PublicIP: "54.12.3.5"},
		{ID: "i-0a1b2c3d4e5f60003", Name: "db-primary", State: "stopped", PrivateIP: "10.0.2.40"},
		{ID: "i-0a1b2c3d4e5f60004", Name: "worker-3", State: "running", PrivateIP: "10.0.3.7"},
		{ID: "i-0a1b2c3d4e5f60005", Name: "batch-runner", State: "pending", PrivateIP: "10.0.3.8"},
		{ID: "i-0a1b2c3d4e5f60006", Name: "old-jenkins", State: "stopping", PrivateIP: "10.0.9.9"},
	}
}

func demoRegions() []string {
	return []string{"us-east-1", "us-west-2", "eu-west-1", "eu-west-2", "ap-southeast-1"}
}
