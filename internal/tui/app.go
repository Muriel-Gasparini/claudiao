package tui

import (
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
)

type screen int

const (
	screenWelcome screen = iota
	screenModules
	screenMode
	screenDone
)

type InstallMode int

const (
	ModeCopy InstallMode = iota
	ModeSymlink
)

type Module struct {
	ID      string
	Name    string
	Desc    string
	Count   int
	Enabled bool
}

type Model struct {
	screen screen
	width  int
	height int

	claudePath  string
	claudeThere bool

	welcomeCursor int

	modules       []Module
	modulesCursor int

	mode       InstallMode
	modeCursor int
}

func defaultModules() []Module {
	return []Module{
		{ID: "rules", Name: "Rules", Desc: "Global conventions and behavior rules", Count: 6, Enabled: true},
		{ID: "commands", Name: "Commands", Desc: "Slash commands for SDD flow", Count: 7, Enabled: true},
		{ID: "agents", Name: "Agents", Desc: "SDD sub-agent definitions", Count: 7, Enabled: true},
		{ID: "output-styles", Name: "Output Styles", Desc: "Orchestrator persona", Count: 1, Enabled: true},
		{ID: "templates", Name: "Templates", Desc: "Spec templates (discover/design/tasks)", Count: 4, Enabled: true},
	}
}

func New() Model {
	home, _ := os.UserHomeDir()
	path := filepath.Join(home, ".claude")
	_, err := os.Stat(path)
	return Model{
		screen:      screenWelcome,
		claudePath:  path,
		claudeThere: err == nil,
		modules:     defaultModules(),
	}
}

func (m Model) Init() tea.Cmd { return nil }

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if sm, ok := msg.(tea.WindowSizeMsg); ok {
		m.width = sm.Width
		m.height = sm.Height
		return m, nil
	}

	switch m.screen {
	case screenWelcome:
		return updateWelcome(m, msg)
	case screenModules:
		return updateModules(m, msg)
	case screenMode:
		return updateMode(m, msg)
	case screenDone:
		return updateDone(m, msg)
	}
	return m, nil
}

func (m Model) View() string {
	switch m.screen {
	case screenWelcome:
		return viewWelcome(m)
	case screenModules:
		return viewModules(m)
	case screenMode:
		return viewMode(m)
	case screenDone:
		return viewDone(m)
	}
	return ""
}
