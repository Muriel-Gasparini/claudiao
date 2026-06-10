package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Muriel-Gasparini/claudiao/internal/checks"
	"github.com/Muriel-Gasparini/claudiao/internal/hook"
	"github.com/Muriel-Gasparini/claudiao/internal/immune"
	"github.com/Muriel-Gasparini/claudiao/internal/learn"
	"github.com/Muriel-Gasparini/claudiao/internal/mutate"
	"github.com/Muriel-Gasparini/claudiao/internal/projinit"
	"github.com/Muriel-Gasparini/claudiao/internal/receipt"
	"github.com/Muriel-Gasparini/claudiao/internal/stats"
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
	os.Exit(app(os.Args, os.Stdin, os.Stdout, os.Stderr))
}

func app(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) >= 2 {
		switch args[1] {
		case "-v", "--version", "version":
			fmt.Fprintf(stdout, "claudiao %s (commit %s, built %s)\n", version, commit, date)
			return 0
		case "-h", "--help", "help":
			printHelp(stdout)
			return 0
		case "hook":
			if len(args) < 3 {
				fmt.Fprintln(stderr, "usage: claudiao hook <pre-bash|post-edit|stop>")
				return 1
			}
			return hook.Run(args[2], stdin, stdout, stderr)
		case "receipt":
			return receiptCmd(args[2:], stdout, stderr)
		case "stats":
			if err := stats.SummarizeFile(stdout); err != nil {
				fmt.Fprintf(stderr, "claudiao stats: %v\n", err)
				return 1
			}
			return 0
		case "check":
			return checkCmd(args[2:], stdout, stderr)
		case "mutate":
			return mutateCmd(args[2:], stdout, stderr)
		case "init":
			return initCmd(args[2:], stdout, stderr)
		case "learn":
			return learnCmd(args[2:], stdout, stderr)
		case "immune":
			return immuneCmd(args[2:], stdout, stderr)
		}
	}
	if err := runTUI(); err != nil {
		fmt.Fprintf(stderr, "claudiao: %v\n", err)
		return 1
	}
	return 0
}

func printHelp(out io.Writer) {
	fmt.Fprintln(out, "claudiao — working-agreement installer and enforcement runtime for Claude Code")
	fmt.Fprintln(out)
	fmt.Fprintln(out, "Usage:")
	fmt.Fprintln(out, "  claudiao                      run the interactive TUI installer")
	fmt.Fprintln(out, "  claudiao hook <event>         hook executor (pre-bash | post-edit | stop)")
	fmt.Fprintln(out, "  claudiao receipt create       record a review receipt for the current diff")
	fmt.Fprintln(out, "  claudiao receipt verify       check the receipt against the working tree")
	fmt.Fprintln(out, "  claudiao check                run rule-embedded antipattern checks on the diff")
	fmt.Fprintln(out, "  claudiao mutate               mutation-test the changed lines (test-theater detector)")
	fmt.Fprintln(out, "  claudiao init                 write project-local stack-specific checks (.claude/)")
	fmt.Fprintln(out, "  claudiao learn                mine stats + fix history for risk hotspots")
	fmt.Fprintln(out, "  claudiao immune               learn antibodies from the repo's fix history")
	fmt.Fprintln(out, "  claudiao stats                agreement-adherence report")
	fmt.Fprintln(out, "  claudiao version              print version and exit")
	fmt.Fprintln(out)
	fmt.Fprintln(out, "Env:")
	fmt.Fprintln(out, "  CLAUDIAO_ASSETS_DIR   physical assets dir (required for symlink mode)")
	fmt.Fprintln(out, "  CLAUDIAO_ENFORCE=off  disable all hook enforcement")
	fmt.Fprintln(out, "  CLAUDIAO_STATE_DIR    override state dir (default ~/.claude/claudiao)")
}

func receiptCmd(args []string, stdout, stderr io.Writer) int {
	if len(args) < 1 {
		fmt.Fprintln(stderr, "usage: claudiao receipt <create|verify|show>")
		return 1
	}
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(stderr, "claudiao receipt: %v\n", err)
		return 1
	}
	switch args[0] {
	case "create":
		fs := flag.NewFlagSet("receipt create", flag.ContinueOnError)
		fs.SetOutput(stderr)
		reviewer := fs.String("reviewer", "sdd-reviewer", "reviewer that produced this receipt")
		summary := fs.String("summary", "", "severity summary (e.g. \"blocker:0 major:0 minor:2\")")
		if fs.Parse(args[1:]) != nil {
			return 1
		}
		r, err := receipt.Create(cwd, *reviewer, *summary)
		if err != nil {
			fmt.Fprintf(stderr, "claudiao receipt create: %v\n", err)
			return 1
		}
		_ = stats.Log("receipt.created", "", r.Repo, map[string]string{"reviewer": r.Reviewer})
		fmt.Fprintf(stdout, "receipt recorded for %s (hash %s…)\n", r.Repo, r.Hash[:12])
		return 0
	case "verify":
		r, err := receipt.Verify(cwd)
		switch {
		case err == nil:
			fmt.Fprintf(stdout, "receipt valid — reviewed by %s at %s\n", r.Reviewer, r.CreatedAt.Format(time.RFC3339))
			return 0
		case errors.Is(err, receipt.ErrNoReceipt), errors.Is(err, receipt.ErrStale):
			fmt.Fprintf(stdout, "receipt invalid: %v\n", err)
			return 1
		default:
			fmt.Fprintf(stderr, "claudiao receipt verify: %v\n", err)
			return 1
		}
	case "show":
		r, err := receipt.Load(cwd)
		if err != nil {
			fmt.Fprintf(stderr, "claudiao receipt show: %v\n", err)
			return 1
		}
		fmt.Fprintf(stdout, "repo:     %s\nreviewer: %s\ncreated:  %s\nhash:     %s\nsummary:  %s\n",
			r.Repo, r.Reviewer, r.CreatedAt.Format(time.RFC3339), r.Hash, r.Summary)
		return 0
	default:
		fmt.Fprintf(stderr, "unknown receipt subcommand %q\n", args[0])
		return 1
	}
}

func checkCmd(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("check", flag.ContinueOnError)
	fs.SetOutput(stderr)
	rulesDir := fs.String("rules", defaultRulesDir(), "global directory of rule .md files with claudiao-check blocks")
	if fs.Parse(args) != nil {
		return 1
	}

	// Load the global rules plus the project-local .claude/rules (written by
	// `claudiao init`), so stack-specific checks apply without extra flags.
	var cs []checks.Check
	for _, dir := range candidateRuleDirs(*rulesDir) {
		loaded, err := checks.LoadDir(dir)
		if err != nil {
			fmt.Fprintf(stderr, "claudiao check: loading rules from %s: %v\n", dir, err)
			return 1
		}
		cs = append(cs, loaded...)
	}
	if len(cs) == 0 {
		fmt.Fprintf(stderr, "claudiao check: no claudiao-check blocks found (looked in %s and ./.claude/rules)\n", *rulesDir)
		return 0
	}
	diff, err := gitDiffHead()
	if err != nil {
		fmt.Fprintf(stderr, "claudiao check: git diff: %v\n", err)
		return 1
	}
	return checks.Report(checks.RunOnDiff(cs, diff), stdout)
}

// candidateRuleDirs returns the global rules dir plus the project-local one
// when it exists and is not the same directory.
func candidateRuleDirs(global string) []string {
	dirs := []string{global}
	if cwd, err := os.Getwd(); err == nil {
		local := filepath.Join(cwd, ".claude", "rules")
		if info, err := os.Stat(local); err == nil && info.IsDir() && local != global {
			dirs = append(dirs, local)
		}
	}
	return dirs
}

func initCmd(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	fs.SetOutput(stderr)
	force := fs.Bool("force", false, "overwrite existing project-local check files")
	if fs.Parse(args) != nil {
		return 1
	}
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(stderr, "claudiao init: %v\n", err)
		return 1
	}
	res, err := projinit.Generate(cwd, *force)
	if err != nil {
		fmt.Fprintf(stderr, "claudiao init: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "detected stack: %s\n", strings.Join(res.Stacks, ", "))
	for _, w := range res.Written {
		fmt.Fprintf(stdout, "  wrote   %s\n", w)
	}
	for _, s := range res.Skipped {
		fmt.Fprintf(stdout, "  skipped %s (exists; --force to overwrite)\n", s)
	}
	fmt.Fprintln(stdout, "run `claudiao check` to apply these against your diff")
	return 0
}

func mutateCmd(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("mutate", flag.ContinueOnError)
	fs.SetOutput(stderr)
	cmd := fs.String("cmd", "", "test command (default: autodetected)")
	max := fs.Int("max", 8, "maximum number of mutants")
	timeout := fs.Duration("timeout", 2*time.Minute, "per-mutant test timeout")
	asJSON := fs.Bool("json", false, "emit surviving mutants as JSON (for the test-fortifier agent)")
	if fs.Parse(args) != nil {
		return 1
	}
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(stderr, "claudiao mutate: %v\n", err)
		return 1
	}
	diff, err := gitDiffHead()
	if err != nil {
		fmt.Fprintf(stderr, "claudiao mutate: git diff: %v\n", err)
		return 1
	}
	mutants := mutate.FromDiff(diff, *max)
	if len(mutants) == 0 {
		if *asJSON {
			fmt.Fprintln(stdout, `{"survivors":[],"total":0,"note":"no mutable changed lines"}`)
		} else {
			fmt.Fprintln(stdout, "no mutable changed lines in the diff — nothing to test")
		}
		return 0
	}
	testCmd := *cmd
	if testCmd == "" {
		testCmd = mutate.DetectTestCmd(cwd)
	}
	if testCmd == "" {
		fmt.Fprintln(stderr, "claudiao mutate: could not detect a test command; pass --cmd")
		return 1
	}

	progress := func(i int, m mutate.Mutant) {
		if !*asJSON {
			fmt.Fprintf(stdout, "  [%d/%d] %s:%d (%s)\n", i+1, len(mutants), m.File, m.LineNo, m.Op)
		}
	}
	if !*asJSON {
		fmt.Fprintf(stdout, "running %d mutants against %q (tracked changes only — untracked files are not mutated)\n",
			len(mutants), testCmd)
	}

	outcomes, err := mutate.Run(cwd, testCmd, mutants, *timeout, progress)
	if err != nil {
		fmt.Fprintf(stderr, "claudiao mutate: %v\n", err)
		return 1
	}

	type survivor struct {
		File     string `json:"file"`
		Line     int    `json:"line"`
		Op       string `json:"op"`
		Original string `json:"original"`
		Mutated  string `json:"mutated"`
	}
	var survivors []survivor
	for _, o := range outcomes {
		if o.Err == nil && o.Survived {
			survivors = append(survivors, survivor{o.Mutant.File, o.Mutant.LineNo, o.Mutant.Op, o.Mutant.Original, o.Mutant.Mutated})
		}
	}
	_ = stats.Log("mutate.run", "", cwd, map[string]string{
		"mutants": fmt.Sprint(len(mutants)), "survived": fmt.Sprint(len(survivors))})

	if *asJSON {
		out, _ := json.Marshal(map[string]any{
			"survivors": survivors, "total": len(outcomes), "test_cmd": testCmd})
		fmt.Fprintln(stdout, string(out))
		if len(survivors) > 0 {
			return 1
		}
		return 0
	}

	for _, o := range outcomes {
		switch {
		case o.Err != nil:
			fmt.Fprintf(stdout, "  skip %s:%d — %v\n", o.Mutant.File, o.Mutant.LineNo, o.Err)
		case o.Survived:
			fmt.Fprintf(stdout, "  SURVIVED %s:%d — %s\n    was: %s\n    now: %s\n",
				o.Mutant.File, o.Mutant.LineNo, o.Mutant.Op, o.Mutant.Original, o.Mutant.Mutated)
		}
	}
	if len(survivors) > 0 {
		fmt.Fprintf(stdout, "\n%d/%d mutants survived — the tests covering this diff cannot fail. Strengthen them.\n",
			len(survivors), len(outcomes))
		fmt.Fprintln(stdout, "Tip: claudiao mutate --json | feed the survivors to the test-fortifier agent.")
		return 1
	}
	fmt.Fprintf(stdout, "\nall %d mutants killed — the tests around this diff actually bite.\n", len(outcomes))
	return 0
}

func learnCmd(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("learn", flag.ContinueOnError)
	fs.SetOutput(stderr)
	limit := fs.Int("commits", 500, "how many recent commits to scan for fixes")
	if fs.Parse(args) != nil {
		return 1
	}

	var summary learn.StatsSummary
	if dir, err := stats.Dir(); err == nil {
		if f, err := os.Open(filepath.Join(dir, "stats.jsonl")); err == nil {
			summary = learn.AggregateStats(f)
			f.Close()
		} else {
			summary = learn.StatsSummary{BlocksByType: map[string]int{}, AreaBlocks: map[string]int{}}
		}
	}

	var fixes []learn.FixCommit
	out, err := exec.Command("git", "-c", "core.quotePath=false", "log", "--no-merges",
		fmt.Sprintf("-n%d", *limit), "--format=%x00%s", "--name-only").Output()
	if err == nil {
		fixes = learn.ParseGitLog(string(out))
	}

	learn.Report(summary, fixes, stdout)
	return 0
}

func immuneCmd(args []string, stdout, stderr io.Writer) int {
	sub := "status"
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		sub, args = args[0], args[1:]
	}
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(stderr, "claudiao immune: %v\n", err)
		return 1
	}

	switch sub {
	case "scan":
		fs := flag.NewFlagSet("immune scan", flag.ContinueOnError)
		fs.SetOutput(stderr)
		commits := fs.Int("commits", 500, "how many recent commits to scan for fixes")
		minHits := fs.Int("min-hits", 2, "minimum distinct fix commits before an antipattern becomes an antibody")
		apply := fs.Bool("apply", false, "materialize antibodies (default is a dry run)")
		if fs.Parse(args) != nil {
			return 1
		}
		if err := immune.Scan(cwd, *commits, *minHits, *apply, stdout); err != nil {
			fmt.Fprintf(stderr, "claudiao immune scan: %v\n", err)
			return 1
		}
		return 0
	case "verify":
		diff, err := gitDiffHead()
		if err != nil {
			fmt.Fprintf(stderr, "claudiao immune verify: git diff: %v\n", err)
			return 1
		}
		code, err := immune.Verify(cwd, diff, stdout)
		if err != nil {
			fmt.Fprintf(stderr, "claudiao immune verify: %v\n", err)
			return 1
		}
		return code
	case "status":
		if err := immune.Status(cwd, stdout); err != nil {
			fmt.Fprintf(stderr, "claudiao immune status: %v\n", err)
			return 1
		}
		return 0
	default:
		fmt.Fprintf(stderr, "usage: claudiao immune <scan|verify|status>\n")
		return 1
	}
}

func defaultRulesDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "rules"
	}
	return filepath.Join(home, ".claude", "rules")
}

// gitDiffHead returns the working-tree diff against HEAD, falling back to the
// empty tree in a repo with no commits yet so `check`/`mutate` work on an
// initial commit instead of failing with exit 128.
func gitDiffHead() (string, error) {
	// The trailing `--` forces HEAD to be parsed as a revision; without it a
	// worktree file literally named HEAD makes git print an ambiguity warning
	// and exit 0, masking a real diff as empty.
	if out, err := exec.Command("git", "diff", "HEAD", "--").Output(); err == nil {
		return string(out), nil
	}
	if err := exec.Command("git", "rev-parse", "--git-dir").Run(); err != nil {
		return "", fmt.Errorf("not a git repository")
	}
	const emptyTree = "4b825dc642cb6eb9a060e54bf8d69288fbee4904"
	out, err := exec.Command("git", "diff", emptyTree, "--").Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}
