// Package mutate proves (or disproves) that the tests guarding a diff can
// actually fail: it flips conditions on the changed lines, runs the suite in
// a throwaway git worktree and reports every mutant that survives. A
// surviving mutant means the new tests are decoration, not tests.
package mutate

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type Mutant struct {
	File     string
	LineNo   int // 1-based line number in the post-diff file
	Original string
	Mutated  string
	Op       string
}

type Outcome struct {
	Mutant   Mutant
	Survived bool
	Err      error
}

var supportedExt = map[string]bool{
	".go": true, ".ts": true, ".tsx": true, ".js": true, ".jsx": true,
	".py": true, ".rs": true, ".java": true, ".kt": true,
}

type mutator struct {
	op    string
	re    *regexp.Regexp
	repl  string
	angle bool // mutates < or >, which double as TS generic brackets
}

// Order matters: longer operators first so "<=" is not caught by "<".
var mutators = []mutator{
	{op: "== to !=", re: regexp.MustCompile(`==`), repl: "!="},
	{op: "!= to ==", re: regexp.MustCompile(`!=`), repl: "=="},
	{op: "<= to >", re: regexp.MustCompile(`<=`), repl: ">"},
	{op: ">= to <", re: regexp.MustCompile(`>=`), repl: "<"},
	{op: "&& to ||", re: regexp.MustCompile(`&&`), repl: "||"},
	{op: "|| to &&", re: regexp.MustCompile(`\|\|`), repl: "&&"},
	{op: "< to >=", re: regexp.MustCompile(`([^<=!])<([^<=])`), repl: "${1}>=${2}", angle: true},
	{op: "> to <=", re: regexp.MustCompile(`([^>=!-])>([^>=])`), repl: "${1}<=${2}", angle: true},
}

// tsGenericRe matches an identifier immediately followed by `<` (no space) —
// the shape of a TypeScript generic (Pick<T>, Promise<T>, Array<T>). Real
// comparisons (`a < b`) carry whitespace; generics are erased at runtime, so
// mutating their angle brackets only produces equivalent mutants (noise).
var tsGenericRe = regexp.MustCompile(`[A-Za-z_$][\w$]*<`)

func isTypeScript(file string) bool {
	ext := filepath.Ext(file)
	return ext == ".ts" || ext == ".tsx"
}

var hunkRe = regexp.MustCompile(`^@@ -\d+(?:,\d+)? \+(\d+)(?:,\d+)? @@`)

// FromDiff extracts candidate mutants from the added lines of a unified diff.
func FromDiff(diff string, cap int) []Mutant {
	var out []Mutant
	file := ""
	lineNo := 0
	for _, raw := range strings.Split(diff, "\n") {
		switch {
		case strings.HasPrefix(raw, "+++ b/"):
			file = strings.TrimPrefix(raw, "+++ b/")
		case hunkRe.MatchString(raw):
			start, _ := strconv.Atoi(hunkRe.FindStringSubmatch(raw)[1])
			lineNo = start - 1
		case strings.HasPrefix(raw, "-"):
			// removed line: does not advance the new-file counter
		case strings.HasPrefix(raw, "+"):
			lineNo++
			if !supportedExt[filepath.Ext(file)] || strings.HasSuffix(file, "_test.go") ||
				strings.Contains(file, ".test.") || strings.Contains(file, ".spec.") {
				continue
			}
			content := raw[1:]
			trimmed := strings.TrimSpace(content)
			if trimmed == "" || strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "#") {
				continue
			}
			tsGeneric := isTypeScript(file) && tsGenericRe.MatchString(content)
			for _, m := range mutators {
				if len(out) >= cap {
					return out
				}
				if m.angle && tsGeneric {
					continue // mutating TS generic brackets yields equivalent mutants
				}
				mutated := m.re.ReplaceAllString(content, m.repl)
				if mutated != content {
					out = append(out, Mutant{File: file, LineNo: lineNo, Original: content, Mutated: mutated, Op: m.op})
					break // one mutant per line keeps runs short
				}
			}
		default:
			if file != "" && !strings.HasPrefix(raw, "\\") && !strings.HasPrefix(raw, "diff ") &&
				!strings.HasPrefix(raw, "index ") && !strings.HasPrefix(raw, "--- ") {
				lineNo++
			}
		}
	}
	return out
}

// DetectTestCmd picks the suite runner from project markers.
func DetectTestCmd(dir string) string {
	switch {
	case exists(filepath.Join(dir, "go.mod")):
		return "go test ./..."
	case exists(filepath.Join(dir, "package.json")):
		return "npm test --silent"
	case exists(filepath.Join(dir, "pyproject.toml")) || exists(filepath.Join(dir, "pytest.ini")):
		return "python -m pytest -q"
	case exists(filepath.Join(dir, "Cargo.toml")):
		return "cargo test -q"
	}
	return ""
}

// Run executes each mutant in an isolated worktree that mirrors the current
// working tree (HEAD + working diff). Tracked changes only: untracked files
// are not part of `git diff HEAD` and are reported as a limitation upstream.
func Run(repoDir, testCmd string, mutants []Mutant, timeout time.Duration, progress func(i int, m Mutant)) ([]Outcome, error) {
	wt, err := os.MkdirTemp("", "claudiao-mutate-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(wt)

	if out, err := git(repoDir, "worktree", "add", "--detach", wt, "HEAD"); err != nil {
		return nil, fmt.Errorf("worktree add: %s: %w", out, err)
	}
	defer git(repoDir, "worktree", "remove", "--force", wt)

	// --binary includes the full content of binary files (e.g. a .ts with an
	// accidental NUL byte, which git treats as binary). Without it, the diff
	// is just "Binary files differ" and `git apply` fails, taking down the
	// whole mutation run over a file that was never a mutation target.
	diff, err := git(repoDir, "diff", "HEAD", "--binary")
	if err != nil {
		return nil, fmt.Errorf("git diff: %w", err)
	}
	if strings.TrimSpace(diff) != "" {
		apply := exec.Command("git", "apply", "--binary", "-")
		apply.Dir = wt
		apply.Stdin = strings.NewReader(diff)
		if out, err := apply.CombinedOutput(); err != nil {
			return nil, fmt.Errorf("apply working diff: %s: %w", out, err)
		}
	}

	var outcomes []Outcome
	for i, m := range mutants {
		if progress != nil {
			progress(i, m)
		}
		o := runOne(wt, testCmd, m, timeout)
		outcomes = append(outcomes, o)
	}
	return outcomes, nil
}

func runOne(wt, testCmd string, m Mutant, timeout time.Duration) Outcome {
	target := filepath.Join(wt, filepath.FromSlash(m.File))
	data, err := os.ReadFile(target)
	if err != nil {
		return Outcome{Mutant: m, Err: err}
	}
	lines := strings.Split(string(data), "\n")
	if m.LineNo < 1 || m.LineNo > len(lines) || lines[m.LineNo-1] != m.Original {
		return Outcome{Mutant: m, Err: fmt.Errorf("line drifted; skipping mutant")}
	}
	lines[m.LineNo-1] = m.Mutated
	if err := os.WriteFile(target, []byte(strings.Join(lines, "\n")), 0o644); err != nil {
		return Outcome{Mutant: m, Err: err}
	}
	defer func() {
		lines[m.LineNo-1] = m.Original
		_ = os.WriteFile(target, []byte(strings.Join(lines, "\n")), 0o644)
	}()

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "sh", "-c", testCmd)
	cmd.Dir = wt
	err = cmd.Run()
	if ctx.Err() == context.DeadlineExceeded {
		// A hung suite counts as killed: the mutant changed behavior.
		return Outcome{Mutant: m, Survived: false}
	}
	return Outcome{Mutant: m, Survived: err == nil}
}

func git(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func exists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}
