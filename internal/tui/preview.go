package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Muriel-Gasparini/claudiao/internal/assets"
	"github.com/Muriel-Gasparini/claudiao/internal/installer"
)

type planReadyMsg struct {
	plan *installer.Plan
	err  error
}

func buildPlanCmd(m Model) tea.Cmd {
	return func() tea.Msg {
		req := installer.Request{
			ClaudePath: m.claudePath,
			Assets:     assets.FS(),
			AssetsDir:  assetsDir(),
			Modules:    m.selectedModuleIDs(),
			Mode:       m.installerMode(),
		}
		plan, err := installer.Preview(req)
		return planReadyMsg{plan: plan, err: err}
	}
}

func updatePreview(m Model, msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case planReadyMsg:
		m.plan = msg.plan
		m.planErr = msg.err
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "b":
			m.screen = screenMode
			m.plan = nil
			m.planErr = nil
		case "up", "k":
			if m.previewCur > 0 {
				m.previewCur--
			}
		case "down", "j":
			if m.plan != nil && m.previewCur < len(m.plan.Actions)-1 {
				m.previewCur++
			}
		case "i", "enter":
			if m.plan != nil && m.planErr == nil && len(m.plan.Actions) > 0 {
				m.screen = screenInstalling
				return m, tea.Batch(spinnerTick(), runInstallCmd(m))
			}
		}
	}
	return m, nil
}

func viewPreview(m Model) string {
	title := titleStyle.Render("Preview")

	if m.planErr != nil {
		body := strings.Join([]string{
			title,
			"",
			errStyle.Render("! " + m.planErr.Error()),
			"",
			mutedStyle.Render("b back · q quit"),
		}, "\n")
		return boxStyle.Render(body)
	}

	if m.plan == nil {
		return boxStyle.Render(title + "\n\n" + mutedStyle.Render("Computing plan…"))
	}

	var create, same, differ int
	for _, a := range m.plan.Actions {
		switch a.Kind {
		case installer.KindCreate:
			create++
		case installer.KindSame:
			same++
		case installer.KindDiffer:
			differ++
		}
	}
	summary := fmt.Sprintf("%s  %s  %s",
		okStyle.Render(fmt.Sprintf("+%d create", create)),
		mutedStyle.Render(fmt.Sprintf("=%d same", same)),
		warnStyle.Render(fmt.Sprintf("~%d differ", differ)),
	)

	start := 0
	end := len(m.plan.Actions)
	if end > 15 {
		start = m.previewCur - 7
		if start < 0 {
			start = 0
		}
		end = start + 15
		if end > len(m.plan.Actions) {
			end = len(m.plan.Actions)
			start = end - 15
		}
	}

	var lines []string
	for i := start; i < end; i++ {
		a := m.plan.Actions[i]
		cursor := "  "
		if i == m.previewCur {
			cursor = cursorStyle.Render("▸ ")
		}
		sigil, st := "+", okStyle
		switch a.Kind {
		case installer.KindSame:
			sigil, st = "=", mutedStyle
		case installer.KindDiffer:
			sigil, st = "~", warnStyle
		}
		lines = append(lines, fmt.Sprintf("%s%s %s", cursor, st.Render(sigil), mutedStyle.Render(a.Source)))
	}
	if len(m.plan.Actions) == 0 {
		lines = []string{warnStyle.Render("no files to install")}
	}

	modeName := "copy"
	if m.mode == ModeSymlink {
		modeName = "symlink"
	}
	meta := mutedStyle.Render(fmt.Sprintf("mode: %s · target: %s", modeName, m.claudePath))

	body := strings.Join([]string{
		title,
		"",
		summary,
		"",
		strings.Join(lines, "\n"),
		"",
		meta,
	}, "\n")

	help := helpStyle.Render("↑/↓ scroll · i/enter install · b back · q quit")
	return fmt.Sprintf("%s\n\n%s", boxStyle.Render(body), help)
}
