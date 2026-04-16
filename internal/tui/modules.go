package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

func updateModules(m Model, msg tea.Msg) (tea.Model, tea.Cmd) {
	km, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch km.String() {
	case "ctrl+c", "q":
		return m, tea.Quit
	case "up", "k":
		if m.modulesCursor > 0 {
			m.modulesCursor--
		}
	case "down", "j":
		if m.modulesCursor < len(m.modules)-1 {
			m.modulesCursor++
		}
	case " ", "x":
		m.modules[m.modulesCursor].Enabled = !m.modules[m.modulesCursor].Enabled
	case "a":
		all := true
		for _, mod := range m.modules {
			if !mod.Enabled {
				all = false
				break
			}
		}
		for i := range m.modules {
			m.modules[i].Enabled = !all
		}
	case "b":
		m.screen = screenWelcome
	case "enter":
		if anyEnabled(m.modules) {
			m.screen = screenDone
		}
	}
	return m, nil
}

func anyEnabled(mods []Module) bool {
	for _, mod := range mods {
		if mod.Enabled {
			return true
		}
	}
	return false
}

func viewModules(m Model) string {
	title := titleStyle.Render("Select modules")
	subtitle := mutedStyle.Render("Choose what to install into ~/.claude")

	var lines []string
	for i, mod := range m.modules {
		cursor := "  "
		if m.modulesCursor == i {
			cursor = cursorStyle.Render("▸ ")
		}

		mark := mutedStyle.Render("[ ]")
		if mod.Enabled {
			mark = okStyle.Render("[x]")
		}

		nameStyle := itemStyle
		if m.modulesCursor == i {
			nameStyle = selectedItemStyle
		}

		count := mutedStyle.Render(fmt.Sprintf("(%d)", mod.Count))
		name := nameStyle.Render(mod.Name)
		desc := mutedStyle.Render("— " + mod.Desc)

		lines = append(lines, fmt.Sprintf("%s%s %s %s %s", cursor, mark, name, count, desc))
	}

	footer := mutedStyle.Render(fmt.Sprintf("%d of %d selected", countEnabled(m.modules), len(m.modules)))

	body := strings.Join([]string{
		title,
		subtitle,
		"",
		strings.Join(lines, "\n"),
		"",
		footer,
	}, "\n")

	help := helpStyle.Render("↑/↓ navigate · space toggle · a toggle all · b back · enter next · q quit")

	return fmt.Sprintf("%s\n\n%s", boxStyle.Render(body), help)
}

func countEnabled(mods []Module) int {
	n := 0
	for _, mod := range mods {
		if mod.Enabled {
			n++
		}
	}
	return n
}
