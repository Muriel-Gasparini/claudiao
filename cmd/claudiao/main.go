package main

import (
	"fmt"
	"io"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Muriel-Gasparini/claudiao/internal/tui"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	if handled := handleFlag(os.Args, os.Stdout); handled {
		return
	}
	p := tea.NewProgram(tui.New(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "claudiao: %v\n", err)
		os.Exit(1)
	}
}

func handleFlag(args []string, out io.Writer) bool {
	if len(args) < 2 {
		return false
	}
	switch args[1] {
	case "-v", "--version", "version":
		fmt.Fprintf(out, "claudiao %s (commit %s, built %s)\n", version, commit, date)
		return true
	case "-h", "--help", "help":
		fmt.Fprintln(out, "claudiao — SDD framework installer for Claude Code")
		fmt.Fprintln(out)
		fmt.Fprintln(out, "Usage:")
		fmt.Fprintln(out, "  claudiao              run the interactive TUI installer")
		fmt.Fprintln(out, "  claudiao version      print version and exit")
		fmt.Fprintln(out, "  claudiao help         print this help")
		fmt.Fprintln(out)
		fmt.Fprintln(out, "Env:")
		fmt.Fprintln(out, "  CLAUDIAO_ASSETS_DIR   physical assets dir (required for symlink mode)")
		return true
	}
	return false
}
