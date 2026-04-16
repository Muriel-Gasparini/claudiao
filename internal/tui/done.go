package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

func updateDone(m Model, msg tea.Msg) (tea.Model, tea.Cmd) {
	km, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch km.String() {
	case "ctrl+c", "q", "enter", "esc":
		return m, tea.Quit
	case "b":
		m.screen = screenModules
	}
	return m, nil
}

func viewDone(m Model) string {
	title := titleStyle.Render("Summary")
	subtitle := mutedStyle.Render("Preview — nothing written yet")

	var lines []string
	for _, mod := range m.modules {
		if !mod.Enabled {
			continue
		}
		lines = append(lines, okStyle.Render("✓ ")+selectedItemStyle.Render(mod.Name)+mutedStyle.Render(fmt.Sprintf(" (%d files)", mod.Count)))
	}
	if len(lines) == 0 {
		lines = append(lines, warnStyle.Render("no modules selected"))
	}

	body := strings.Join([]string{
		title,
		subtitle,
		"",
		strings.Join(lines, "\n"),
	}, "\n")

	help := helpStyle.Render("b back · enter quit")
	return fmt.Sprintf("%s\n\n%s", boxStyle.Render(body), help)
}
