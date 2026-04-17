package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

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
		if m.result.backupPath != "" {
			lines = append(lines, fmt.Sprintf("%s backup: %s",
				okStyle.Render("•"), mutedStyle.Render(m.result.backupPath)))
		}
		body = strings.Join(lines, "\n")
	}

	help := helpStyle.Render("enter quit")
	return fmt.Sprintf("%s\n\n%s\n\n%s", boxStyle.Render(title+"\n\n"+body), "", help)
}
