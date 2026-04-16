package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

type modeOption struct {
	mode InstallMode
	name string
	desc string
}

var modeOptions = []modeOption{
	{ModeCopy, "Copy", "Files are copied into ~/.claude. Safe — easy to customize locally without touching the repo."},
	{ModeSymlink, "Symlink", "Files are symlinked from this repo. Pulls from upstream apply immediately. Best for contributors."},
}

func updateMode(m Model, msg tea.Msg) (tea.Model, tea.Cmd) {
	km, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch km.String() {
	case "ctrl+c", "q":
		return m, tea.Quit
	case "up", "k":
		if m.modeCursor > 0 {
			m.modeCursor--
		}
	case "down", "j":
		if m.modeCursor < len(modeOptions)-1 {
			m.modeCursor++
		}
	case "b":
		m.screen = screenModules
	case "enter":
		m.mode = modeOptions[m.modeCursor].mode
		m.screen = screenDone
	}
	return m, nil
}

func viewMode(m Model) string {
	title := titleStyle.Render("Install mode")
	subtitle := mutedStyle.Render("How should selected files land in ~/.claude?")

	var lines []string
	for i, opt := range modeOptions {
		cursor := "  "
		nameStyle := itemStyle
		radio := mutedStyle.Render("( )")
		if m.modeCursor == i {
			cursor = cursorStyle.Render("▸ ")
			nameStyle = selectedItemStyle
			radio = okStyle.Render("(•)")
		}
		head := fmt.Sprintf("%s%s %s", cursor, radio, nameStyle.Render(opt.name))
		desc := "     " + mutedStyle.Render(opt.desc)
		lines = append(lines, head+"\n"+desc)
	}

	body := strings.Join([]string{
		title,
		subtitle,
		"",
		strings.Join(lines, "\n\n"),
	}, "\n")

	help := helpStyle.Render("↑/↓ navigate · enter select · b back · q quit")

	return fmt.Sprintf("%s\n\n%s", boxStyle.Render(body), help)
}
