package tui

import (
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Muriel-Gasparini/claudiao/internal/assets"
	"github.com/Muriel-Gasparini/claudiao/internal/installer"
)

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

type spinnerTickMsg struct{}
type installDoneMsg struct {
	err        error
	backupPath string
	written    int
}

func spinnerTick() tea.Cmd {
	return tea.Tick(80*time.Millisecond, func(time.Time) tea.Msg { return spinnerTickMsg{} })
}

func runInstallCmd(m Model) tea.Cmd {
	plan := m.plan
	req := installer.Request{
		ClaudePath: m.claudePath,
		Assets:     assets.FS(),
		AssetsDir:  assetsDir(),
		Modules:    m.selectedModuleIDs(),
		Mode:       m.installerMode(),
	}
	return func() tea.Msg {
		backup, err := installer.Backup(req.ClaudePath)
		if err != nil {
			return installDoneMsg{err: err}
		}
		written := 0
		err = installer.Apply(plan, req, func(done, total int, current string) {
			written = done
		})
		return installDoneMsg{err: err, backupPath: backup, written: written}
	}
}

func updateInstalling(m Model, msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case spinnerTickMsg:
		m.spinnerFrame = (m.spinnerFrame + 1) % len(spinnerFrames)
		return m, spinnerTick()
	case installDoneMsg:
		m.result = installResult{err: msg.err, backupPath: msg.backupPath, written: msg.written}
		m.screen = screenResult
		return m, nil
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
	}
	return m, nil
}

func viewInstalling(m Model) string {
	frame := cursorStyle.Render(spinnerFrames[m.spinnerFrame])
	body := strings.Join([]string{
		titleStyle.Render("Installing"),
		"",
		frame + " " + mutedStyle.Render("writing files and backing up existing state…"),
	}, "\n")
	return boxStyle.Render(body)
}
