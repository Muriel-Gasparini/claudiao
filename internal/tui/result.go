package tui

import (
	"fmt"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Muriel-Gasparini/claudiao/internal/installer"
)

func filepathDir(p string) string { return filepath.Dir(p) }

func updateResult(m Model, msg tea.Msg) (tea.Model, tea.Cmd) {
	km, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch km.String() {
	case "ctrl+c", "q", "enter", "esc":
		return m, tea.Quit
	}
	return m, nil
}

func viewResult(m Model) string {
	var title, body string
	if m.result.err != nil {
		title = errStyle.Render("✗ Install failed")
		body = errStyle.Render(m.result.err.Error())
		if m.result.backupPath != "" {
			body += "\n\n" + mutedStyle.Render("Backup kept at: ") + m.result.backupPath
		}
	} else {
		title = okStyle.Render("✓ Done")
		lines := []string{
			fmt.Sprintf("%s %d files installed into %s",
				okStyle.Render("•"), m.result.written, mutedStyle.Render(m.claudePath)),
		}
		if m.result.hooksMerged {
			if m.result.bin.Copied {
				lines = append(lines, fmt.Sprintf("%s binary installed at %s",
					okStyle.Render("•"), mutedStyle.Render(m.result.bin.Path)))
			} else if m.result.bin.AlreadyOK {
				lines = append(lines, fmt.Sprintf("%s binary already at %s",
					okStyle.Render("•"), mutedStyle.Render(m.result.bin.Path)))
			}
			lines = append(lines, fmt.Sprintf("%s enforcement hooks merged into settings.json (disable: CLAUDIAO_ENFORCE=off)",
				okStyle.Render("•")))
			if !m.result.bin.OnPath {
				lines = append(lines, fmt.Sprintf("%s %s is not on your PATH — hooks still work (absolute path), but to run `claudiao` directly add:",
					mutedStyle.Render("!"), mutedStyle.Render(filepathDir(m.result.bin.Path))))
				lines = append(lines, "    "+mutedStyle.Render(installer.PathHint(filepathDir(m.result.bin.Path))))
			}
		}
		if m.result.backupPath != "" {
			lines = append(lines, fmt.Sprintf("%s backup: %s",
				okStyle.Render("•"), mutedStyle.Render(m.result.backupPath)))
		}
		body = strings.Join(lines, "\n")
	}

	help := helpStyle.Render("enter quit")
	return fmt.Sprintf("%s\n\n%s\n\n%s", boxStyle.Render(title+"\n\n"+body), "", help)
}
