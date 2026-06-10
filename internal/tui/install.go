package tui

import (
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Muriel-Gasparini/claudiao/internal/assets"
	"github.com/Muriel-Gasparini/claudiao/internal/installer"
	"github.com/Muriel-Gasparini/claudiao/internal/settings"
)

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

type spinnerTickMsg struct{}
type installDoneMsg struct {
	err         error
	backupPath  string
	written     int
	hooksMerged bool
	bin         installer.BinResult
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
	wantHooks := m.hooksSelected()
	return func() tea.Msg {
		backup, err := installer.Backup(req.ClaudePath)
		if err != nil {
			return installDoneMsg{err: err}
		}
		written := 0
		err = installer.Apply(plan, req, func(done, total int, current string) {
			written = done
		})
		hooksMerged := false
		var bin installer.BinResult
		if err == nil && wantHooks {
			// Install the binary to a stable path first, then point the hooks
			// at that path — not at wherever the installer happened to run
			// from, which may be a temp dir that disappears.
			destDir, dirErr := installer.DefaultBinDir()
			if dirErr != nil {
				err = dirErr
			} else if bin, err = installer.InstallBinary(destDir); err == nil {
				if mergeErr := settings.MergeHooks(req.ClaudePath, bin.Path); mergeErr != nil {
					err = mergeErr
				} else {
					hooksMerged = true
				}
			}
		}
		return installDoneMsg{err: err, backupPath: backup, written: written, hooksMerged: hooksMerged, bin: bin}
	}
}

func updateInstalling(m Model, msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case spinnerTickMsg:
		m.spinnerFrame = (m.spinnerFrame + 1) % len(spinnerFrames)
		return m, spinnerTick()
	case installDoneMsg:
		m.result = installResult{err: msg.err, backupPath: msg.backupPath, written: msg.written, hooksMerged: msg.hooksMerged, bin: msg.bin}
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
