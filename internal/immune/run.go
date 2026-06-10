package immune

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/Muriel-Gasparini/claudiao/internal/checks"
)

// RepoRoot resolves the work tree root from any directory inside it.
func RepoRoot(dir string) (string, error) {
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("not a git repository")
	}
	return strings.TrimSpace(string(out)), nil
}

// gitFixLog returns `git log -p` over the last n fix/revert commits, NUL-prefixed
// per commit so the diffs parse unambiguously and non-ASCII paths are literal.
// Bounded by a timeout so a huge-history repo cannot stall a SessionStart hook.
func gitFixLog(repoRoot string, n int) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", "-c", "core.quotePath=false", "log", "-p", "--no-merges",
		"-i", "--grep=^fix", "--grep=^revert", "--grep=^hotfix",
		fmt.Sprintf("-n%d", n), "--format=%x00%H")
	cmd.Dir = repoRoot
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// Scan mines the fix history, synthesizes antibodies, persists them, and
// reports what was learned. apply=false is a dry run (no files written).
func Scan(dir string, commits, minHits int, apply bool, w io.Writer) error {
	root, err := RepoRoot(dir)
	if err != nil {
		return err
	}
	logp, err := gitFixLog(root, commits)
	if err != nil {
		return fmt.Errorf("git log: %w", err)
	}

	scars := DetectScars(logp, minHits)
	if len(scars) == 0 {
		fmt.Fprintf(w, "no recurring antipatterns found in the last %d fix/revert commits "+
			"(min %d hits). The repo has no scars to immunize against yet.\n", commits, minHits)
		return nil
	}

	var abs []Antibody
	for _, s := range scars {
		abs = append(abs, Synthesize(s))
	}

	fmt.Fprintf(w, "found %d recurring antipattern(s) the repo already fixed:\n\n", len(scars))
	for _, a := range abs {
		fmt.Fprintf(w, "  [%s] %s  (bitten %dx)\n    sample: %s\n    antibody pattern: %s\n",
			a.Severity, a.ID, a.Scar.Hits, oneLine(a.Scar.Sample), a.Pattern)
	}

	if !apply {
		fmt.Fprintf(w, "\ndry run — re-run with --apply to materialize these antibodies into "+
			".claude/rules/immune-checks.md and the ledger.\n")
		return nil
	}

	ledger, err := Load(root)
	if err != nil {
		return err
	}
	added := ledger.Add(abs)
	if err := ledger.Save(root); err != nil {
		return err
	}
	fmt.Fprintf(w, "\nimmunized: %d antibody(ies) recorded (%d new). `claudiao check` now blocks "+
		"these antipatterns from returning.\n", len(abs), len(added))
	return nil
}

// Verify runs the recorded antibodies against the current diff and reports any
// scar that is trying to return. Returns the exit code (1 if any fires).
func Verify(dir, diff string, w io.Writer) (int, error) {
	root, err := RepoRoot(dir)
	if err != nil {
		return 0, err
	}
	ledger, err := Load(root)
	if err != nil {
		return 0, err
	}
	if len(ledger.Antibodies) == 0 {
		fmt.Fprintln(w, "no antibodies recorded yet — run `claudiao immune scan --apply`")
		return 0, nil
	}

	var cs []checks.Check
	for _, a := range ledger.sorted() {
		re, err := regexp.Compile(a.Pattern)
		if err != nil {
			continue // a corrupt stored pattern must not break verification
		}
		cs = append(cs, checks.Check{ID: a.ID, Severity: checks.Severity(a.Severity), Pattern: re, Source: "immune"})
	}
	findings := checks.RunOnDiff(cs, diff)
	if len(findings) == 0 {
		fmt.Fprintf(w, "immune: clear — none of the %d known scars are returning in this diff\n", len(cs))
		return 0, nil
	}
	return checks.Report(findings, w), nil
}

// DetectNew returns recurring antipatterns NOT yet recorded as antibodies —
// the unimmunized scars. It writes nothing; it powers the non-invasive
// SessionStart nudge ("you could immunize against X").
func DetectNew(dir string, commits, minHits int) ([]Scar, error) {
	root, err := RepoRoot(dir)
	if err != nil {
		return nil, err
	}
	logp, err := gitFixLog(root, commits)
	if err != nil {
		return nil, err
	}
	scars := DetectScars(logp, minHits)
	if len(scars) == 0 {
		return nil, nil
	}
	ledger, err := Load(root)
	if err != nil {
		return nil, err
	}
	var fresh []Scar
	for _, s := range scars {
		if _, ok := ledger.Antibodies[Synthesize(s).ID]; !ok {
			fresh = append(fresh, s)
		}
	}
	return fresh, nil
}

// Status summarizes the recorded immunity.
func Status(dir string, w io.Writer) error {
	root, err := RepoRoot(dir)
	if err != nil {
		return err
	}
	ledger, err := Load(root)
	if err != nil {
		return err
	}
	if len(ledger.Antibodies) == 0 {
		fmt.Fprintln(w, "no immunity yet — run `claudiao immune scan --apply`")
		return nil
	}
	fmt.Fprintf(w, "%d antibody(ies) protecting this repo:\n\n", len(ledger.Antibodies))
	for _, a := range ledger.sorted() {
		fmt.Fprintf(w, "  [%s] %s — bitten %dx\n", a.Severity, a.ID, a.Scar.Hits)
	}
	return nil
}
