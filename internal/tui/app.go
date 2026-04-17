package tui

import (
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Muriel-Gasparini/claudiao/internal/installer"
)

type screen int

const (
	screenWelcome screen = iota
	screenModules
	screenMode
	screenPreview
	screenInstalling
	screenResult
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

	plan        *installer.Plan
	planErr     error
	previewCur  int

	spinnerFrame int
	result       installResult
}

type installResult struct {
	err        error
	backupPath string
	written    int
}

func defaultModules() []Module {
	return []Module{
		{ID: "core", Name: "Core", Desc: "CLAUDE.md — global SDD flow and rules index", Count: 1, Enabled: true},
		{ID: "rules", Name: "Rules", Desc: "Global behavior rules (testing, security, performance, code-quality, ui-ux, git, concision, etc)", Count: 9, Enabled: true},
		{ID: "agents", Name: "Agents", Desc: "SDD sub-agent definitions (PO, architect, dev lead, …)", Count: 6, Enabled: true},
		{ID: "output-styles", Name: "Output Styles", Desc: "Orchestrator and per-phase personas", Count: 7, Enabled: true},
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
	case screenPreview:
		return updatePreview(m, msg)
	case screenInstalling:
		return updateInstalling(m, msg)
	case screenResult:
		return updateResult(m, msg)
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
	case screenPreview:
		return viewPreview(m)
	case screenInstalling:
		return viewInstalling(m)
	case screenResult:
		return viewResult(m)
	}
	return ""
}

func (m Model) selectedModuleIDs() []string {
	var ids []string
	for _, mod := range m.modules {
		if mod.Enabled {
			ids = append(ids, mod.ID)
		}
	}
	return ids
}

func (m Model) installerMode() installer.Mode {
	if m.mode == ModeSymlink {
		return installer.ModeSymlink
	}
	return installer.ModeCopy
}

func assetsDir() string {
	if d := os.Getenv("CLAUDIAO_ASSETS_DIR"); d != "" {
		return d
	}
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	candidates := []string{
		filepath.Join(filepath.Dir(exe), "internal/assets/files"),
		filepath.Join(filepath.Dir(exe), "assets"),
	}
	for _, c := range candidates {
		if info, err := os.Stat(c); err == nil && info.IsDir() {
			return c
		}
	}
	return ""
}
