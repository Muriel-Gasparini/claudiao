package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

var welcomeChoices = []string{"Continue", "Abort"}

func updateWelcome(m Model, msg tea.Msg) (tea.Model, tea.Cmd) {
	km, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch km.String() {
	case "ctrl+c", "q":
		return m, tea.Quit
	case "up", "k":
		if m.welcomeCursor > 0 {
			m.welcomeCursor--
		}
	case "down", "j":
		if m.welcomeCursor < len(welcomeChoices)-1 {
			m.welcomeCursor++
		}
	case "enter":
		if m.welcomeCursor == 0 {
			m.screen = screenModules
			return m, nil
		}
		return m, tea.Quit
	}
	return m, nil
}

func viewWelcome(m Model) string {
	title := titleStyle.Render("claudiao")
	subtitle := mutedStyle.Render("SDD framework installer for Claude Code")

	var status string
	if m.claudeThere {
		status = okStyle.Render("✓ detected ") + mutedStyle.Render(m.claudePath)
	} else {
		status = warnStyle.Render("! not found ") + mutedStyle.Render(m.claudePath+" will be created")
	}

	var lines []string
	for i, c := range welcomeChoices {
		cursor := "  "
		style := itemStyle
		if m.welcomeCursor == i {
			cursor = cursorStyle.Render("▸ ")
			style = selectedItemStyle
		}
		lines = append(lines, cursor+style.Render(c))
	}

	body := strings.Join([]string{
		title,
		subtitle,
		"",
		status,
		"",
		strings.Join(lines, "\n"),
	}, "\n")

	help := helpStyle.Render("↑/↓ navigate · enter select · q quit")

	return fmt.Sprintf("%s\n\n%s", boxStyle.Render(body), help)
}
