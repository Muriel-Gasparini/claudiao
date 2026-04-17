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

var runTUI = func() error {
	p := tea.NewProgram(tui.New(), tea.WithAltScreen())
	_, err := p.Run()
	return err
}

func main() {
	os.Exit(app(os.Args, os.Stdout, os.Stderr))
}

func app(args []string, stdout, stderr io.Writer) int {
	if handleFlag(args, stdout) {
		return 0
	}
	if err := runTUI(); err != nil {
		fmt.Fprintf(stderr, "claudiao: %v\n", err)
		return 1
	}
	return 0
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
