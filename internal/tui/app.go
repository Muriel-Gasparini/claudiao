package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

type screen int

const (
	screenWelcome screen = iota
	screenQuit
)

type Model struct {
	screen      screen
	cursor      int
	claudePath  string
	claudeThere bool
	width       int
	height      int
}

func New() Model {
	home, _ := os.UserHomeDir()
	path := filepath.Join(home, ".claude")
	_, err := os.Stat(path)
	return Model{
		screen:      screenWelcome,
		claudePath:  path,
		claudeThere: err == nil,
	}
}

func (m Model) Init() tea.Cmd { return nil }

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < 1 {
				m.cursor++
			}
		case "enter":
			if m.cursor == 1 {
				return m, tea.Quit
			}
		}
	}
	return m, nil
}

func (m Model) View() string {
	switch m.screen {
	case screenWelcome:
		return m.viewWelcome()
	default:
		return ""
	}
}

func (m Model) viewWelcome() string {
	title := titleStyle.Render("claudiao")
	subtitle := mutedStyle.Render("SDD framework installer for Claude Code")

	var status string
	if m.claudeThere {
		status = okStyle.Render("✓ detected ") + mutedStyle.Render(m.claudePath)
	} else {
		status = warnStyle.Render("! not found ") + mutedStyle.Render(m.claudePath+" will be created")
	}

	choices := []string{"Continue", "Abort"}
	var lines []string
	for i, c := range choices {
		cursor := "  "
		style := itemStyle
		if m.cursor == i {
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
