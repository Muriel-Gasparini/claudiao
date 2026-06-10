package main

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// newGitRepo initializes a git repo in a fresh temp dir, configures an
// identity, makes an initial commit, and chdirs into it. It returns the repo
// root. State and HOME are isolated so global rules and the receipt/stats
// store never leak between tests.
func newGitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for _, args := range [][]string{
		{"init", "-q"},
		{"config", "user.email", "t@t"},
		{"config", "user.name", "t"},
		{"config", "commit.gpgsign", "false"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %s", args, out)
		}
	}
	writeFile(t, dir, "README.md", "# initial\n")
	gitRun(t, dir, "add", "-A")
	gitRun(t, dir, "commit", "-q", "-m", "initial commit")
	t.Chdir(dir)
	return dir
}

func gitRun(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %s", args, out)
	}
}

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// isolateState points CLAUDIAO_STATE_DIR and HOME at fresh temp dirs so a test
// never reads or writes another test's (or the developer's) real state.
func isolateState(t *testing.T) {
	t.Helper()
	t.Setenv("CLAUDIAO_STATE_DIR", t.TempDir())
	t.Setenv("HOME", t.TempDir())
}

func TestAppVersion(t *testing.T) {
	version = "1.2.3"
	commit = "abc"
	date = "2026-01-01"
	for _, arg := range []string{"-v", "--version", "version"} {
		stdout := &bytes.Buffer{}
		code := app([]string{"claudiao", arg}, &bytes.Buffer{}, stdout, &bytes.Buffer{})
		if code != 0 {
			t.Errorf("%q: expected exit 0, got %d", arg, code)
		}
		out := stdout.String()
		if !strings.Contains(out, "1.2.3") || !strings.Contains(out, "abc") || !strings.Contains(out, "2026-01-01") {
			t.Errorf("%q output missing version info: %q", arg, out)
		}
	}
}

func TestAppHelp(t *testing.T) {
	for _, arg := range []string{"-h", "--help", "help"} {
		stdout := &bytes.Buffer{}
		code := app([]string{"claudiao", arg}, &bytes.Buffer{}, stdout, &bytes.Buffer{})
		if code != 0 {
			t.Errorf("%q: expected exit 0, got %d", arg, code)
		}
		out := stdout.String()
		for _, want := range []string{"Usage", "claudiao hook", "claudiao receipt", "claudiao mutate", "CLAUDIAO_ENFORCE"} {
			if !strings.Contains(out, want) {
				t.Errorf("%q help missing %q", arg, want)
			}
		}
	}
}

func TestAppHookRequiresEvent(t *testing.T) {
	stderr := &bytes.Buffer{}
	code := app([]string{"claudiao", "hook"}, &bytes.Buffer{}, &bytes.Buffer{}, stderr)
	if code != 1 {
		t.Errorf("expected exit 1, got %d", code)
	}
	if !strings.Contains(stderr.String(), "usage") {
		t.Errorf("expected usage message, got %q", stderr.String())
	}
}

func TestAppHookUnknownEventIsHarmless(t *testing.T) {
	code := app([]string{"claudiao", "hook", "banana"},
		strings.NewReader("{}"), &bytes.Buffer{}, &bytes.Buffer{})
	if code != 0 {
		t.Errorf("unknown hook event must not block the session, got exit %d", code)
	}
}

func TestAppHandlesSubcommandWithoutRunningTUI(t *testing.T) {
	called := false
	orig := runTUI
	runTUI = func() error { called = true; return nil }
	defer func() { runTUI = orig }()

	stdout := &bytes.Buffer{}
	code := app([]string{"claudiao", "version"}, &bytes.Buffer{}, stdout, &bytes.Buffer{})
	if code != 0 {
		t.Errorf("expected exit 0, got %d", code)
	}
	if called {
		t.Error("TUI should not run when a subcommand is handled")
	}
}

func TestAppRunsTUISuccessfully(t *testing.T) {
	orig := runTUI
	runTUI = func() error { return nil }
	defer func() { runTUI = orig }()

	code := app([]string{"claudiao"}, &bytes.Buffer{}, &bytes.Buffer{}, &bytes.Buffer{})
	if code != 0 {
		t.Errorf("expected exit 0 on successful TUI run, got %d", code)
	}
}

func TestAppReturnsOneOnTUIError(t *testing.T) {
	orig := runTUI
	runTUI = func() error { return errors.New("boom") }
	defer func() { runTUI = orig }()

	stderr := &bytes.Buffer{}
	code := app([]string{"claudiao"}, &bytes.Buffer{}, &bytes.Buffer{}, stderr)
	if code != 1 {
		t.Errorf("expected exit 1 on TUI error, got %d", code)
	}
	if !strings.Contains(stderr.String(), "boom") {
		t.Errorf("expected error in stderr, got %q", stderr.String())
	}
}

func TestGitDiffHeadEmptyTreeFallback(t *testing.T) {
	dir := t.TempDir()
	for _, args := range [][]string{
		{"init", "-q"}, {"config", "user.email", "t@t"}, {"config", "user.name", "t"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %s", args, out)
		}
	}
	// Staged file, no commit yet → no HEAD. The fallback must still surface it.
	if err := os.WriteFile(filepath.Join(dir, "a.go"), []byte("package a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	add := exec.Command("git", "add", "a.go")
	add.Dir = dir
	if out, err := add.CombinedOutput(); err != nil {
		t.Fatalf("git add: %s", out)
	}

	t.Chdir(dir)
	diff, err := gitDiffHead()
	if err != nil {
		t.Fatalf("empty-tree fallback errored: %v", err)
	}
	if !strings.Contains(diff, "a.go") {
		t.Errorf("initial-commit diff must include the staged file, got %q", diff)
	}
}

func TestGitDiffHeadNotARepo(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("GIT_CEILING_DIRECTORIES", filepath.Dir(dir))
	t.Chdir(dir)
	if _, err := gitDiffHead(); err == nil {
		t.Error("expected an error outside a git repository")
	}
}

func TestAppReceiptUsage(t *testing.T) {
	stderr := &bytes.Buffer{}
	code := app([]string{"claudiao", "receipt"}, &bytes.Buffer{}, &bytes.Buffer{}, stderr)
	if code != 1 {
		t.Errorf("expected exit 1, got %d", code)
	}
}

// --- receipt ---------------------------------------------------------------

func TestAppReceiptUnknownSubcommand(t *testing.T) {
	isolateState(t)
	newGitRepo(t)

	stderr := &bytes.Buffer{}
	code := app([]string{"claudiao", "receipt", "frobnicate"}, &bytes.Buffer{}, &bytes.Buffer{}, stderr)
	if code != 1 {
		t.Errorf("expected exit 1 for unknown subcommand, got %d", code)
	}
	if !strings.Contains(stderr.String(), "unknown receipt subcommand") {
		t.Errorf("expected unknown-subcommand message, got %q", stderr.String())
	}
}

func TestAppReceiptCreateThenShowAndVerify(t *testing.T) {
	isolateState(t)
	repo := newGitRepo(t)

	// Create binds a receipt to the current (clean) working tree.
	createOut := &bytes.Buffer{}
	if code := app([]string{"claudiao", "receipt", "create", "--reviewer", "sdd-reviewer", "--summary", "blocker:0 major:0 minor:1"},
		&bytes.Buffer{}, createOut, &bytes.Buffer{}); code != 0 {
		t.Fatalf("receipt create: expected exit 0, got %d", code)
	}
	if !strings.Contains(createOut.String(), "receipt recorded for") {
		t.Errorf("create output missing confirmation: %q", createOut.String())
	}

	// Show reads it back; the reviewer and summary must round-trip.
	showOut := &bytes.Buffer{}
	if code := app([]string{"claudiao", "receipt", "show"}, &bytes.Buffer{}, showOut, &bytes.Buffer{}); code != 0 {
		t.Fatalf("receipt show: expected exit 0, got %d", code)
	}
	show := showOut.String()
	for _, want := range []string{"reviewer: sdd-reviewer", "blocker:0 major:0 minor:1", repo} {
		if !strings.Contains(show, want) {
			t.Errorf("show output missing %q: %q", want, show)
		}
	}

	// Verify against the unchanged tree must pass.
	verifyOut := &bytes.Buffer{}
	if code := app([]string{"claudiao", "receipt", "verify"}, &bytes.Buffer{}, verifyOut, &bytes.Buffer{}); code != 0 {
		t.Fatalf("receipt verify (clean): expected exit 0, got %d", code)
	}
	if !strings.Contains(verifyOut.String(), "receipt valid") {
		t.Errorf("verify output missing validity: %q", verifyOut.String())
	}
}

func TestAppReceiptVerifyStaleAfterChange(t *testing.T) {
	isolateState(t)
	repo := newGitRepo(t)

	if code := app([]string{"claudiao", "receipt", "create"}, &bytes.Buffer{}, &bytes.Buffer{}, &bytes.Buffer{}); code != 0 {
		t.Fatalf("receipt create: expected exit 0, got %d", code)
	}

	// Mutate the tree after the review — the fingerprint must no longer match.
	writeFile(t, repo, "new.go", "package main\n")

	stdout := &bytes.Buffer{}
	code := app([]string{"claudiao", "receipt", "verify"}, &bytes.Buffer{}, stdout, &bytes.Buffer{})
	if code != 1 {
		t.Errorf("stale receipt: expected exit 1, got %d", code)
	}
	if !strings.Contains(stdout.String(), "stale") {
		t.Errorf("expected stale message, got %q", stdout.String())
	}
}

func TestAppReceiptVerifyNoReceipt(t *testing.T) {
	isolateState(t)
	newGitRepo(t)

	stdout := &bytes.Buffer{}
	code := app([]string{"claudiao", "receipt", "verify"}, &bytes.Buffer{}, stdout, &bytes.Buffer{})
	if code != 1 {
		t.Errorf("missing receipt: expected exit 1, got %d", code)
	}
	if !strings.Contains(stdout.String(), "receipt invalid") {
		t.Errorf("expected invalid message, got %q", stdout.String())
	}
}

func TestAppReceiptShowNoReceipt(t *testing.T) {
	isolateState(t)
	newGitRepo(t)

	stderr := &bytes.Buffer{}
	code := app([]string{"claudiao", "receipt", "show"}, &bytes.Buffer{}, &bytes.Buffer{}, stderr)
	if code != 1 {
		t.Errorf("missing receipt: expected exit 1, got %d", code)
	}
	if !strings.Contains(stderr.String(), "receipt show") {
		t.Errorf("expected error on stderr, got %q", stderr.String())
	}
}

// --- check -----------------------------------------------------------------

// writeRule drops a rule file with a single claudiao-check block into a global
// rules dir and points checkCmd at it via --rules.
func ruleDirWith(t *testing.T, block string) string {
	t.Helper()
	dir := t.TempDir()
	writeFile(t, dir, "r.md", block)
	return dir
}

func TestAppCheckNoBlocksFoundExitsZero(t *testing.T) {
	newGitRepo(t)
	emptyRules := t.TempDir() // a dir with no .md check blocks

	stderr := &bytes.Buffer{}
	code := app([]string{"claudiao", "check", "--rules", emptyRules}, &bytes.Buffer{}, &bytes.Buffer{}, stderr)
	if code != 0 {
		t.Errorf("no check blocks must not fail the run, got exit %d", code)
	}
	if !strings.Contains(stderr.String(), "no claudiao-check blocks found") {
		t.Errorf("expected no-blocks message, got %q", stderr.String())
	}
}

func TestAppCheckCleanDiffHasNoFindings(t *testing.T) {
	repo := newGitRepo(t)
	rules := ruleDirWith(t, "```claudiao-check\nid: no-eval\nseverity: blocker\npattern: \\beval\\(\n```\n")

	// An added line that does not match the pattern.
	writeFile(t, repo, "app.js", "const x = 1;\n")
	gitRun(t, repo, "add", "-A")

	stdout := &bytes.Buffer{}
	code := app([]string{"claudiao", "check", "--rules", rules}, &bytes.Buffer{}, stdout, &bytes.Buffer{})
	if code != 0 {
		t.Errorf("clean diff: expected exit 0, got %d", code)
	}
	if !strings.Contains(stdout.String(), "no findings") {
		t.Errorf("expected no-findings message, got %q", stdout.String())
	}
}

func TestAppCheckBlockerFindingExitsOne(t *testing.T) {
	repo := newGitRepo(t)
	rules := ruleDirWith(t, "```claudiao-check\nid: no-eval\nseverity: blocker\npattern: \\beval\\(\n```\n")

	writeFile(t, repo, "app.js", "const y = eval('1+1');\n")
	gitRun(t, repo, "add", "-A")

	stdout := &bytes.Buffer{}
	code := app([]string{"claudiao", "check", "--rules", rules}, &bytes.Buffer{}, stdout, &bytes.Buffer{})
	if code != 1 {
		t.Errorf("blocker finding: expected exit 1, got %d", code)
	}
	if !strings.Contains(stdout.String(), "no-eval") {
		t.Errorf("expected the finding id in output, got %q", stdout.String())
	}
}

func TestAppCheckLoadsProjectLocalRules(t *testing.T) {
	repo := newGitRepo(t)
	emptyGlobal := t.TempDir() // global has nothing

	// Project-local rule that claudiao init would have written.
	writeFile(t, repo, ".claude/rules/local-checks.md",
		"```claudiao-check\nid: local-todo\nseverity: blocker\npattern: TODO_FIXME_NOW\n```\n")
	writeFile(t, repo, "code.go", "// TODO_FIXME_NOW handle this\n")
	gitRun(t, repo, "add", "code.go")

	stdout := &bytes.Buffer{}
	code := app([]string{"claudiao", "check", "--rules", emptyGlobal}, &bytes.Buffer{}, stdout, &bytes.Buffer{})
	if code != 1 {
		t.Errorf("project-local check should fire, expected exit 1, got %d", code)
	}
	if !strings.Contains(stdout.String(), "local-todo") {
		t.Errorf("expected project-local finding, got %q", stdout.String())
	}
}

func TestAppCheckBadRuleFileErrors(t *testing.T) {
	newGitRepo(t)
	rules := ruleDirWith(t, "```claudiao-check\nid: broken\npattern: (unclosed\n```\n")

	stderr := &bytes.Buffer{}
	code := app([]string{"claudiao", "check", "--rules", rules}, &bytes.Buffer{}, &bytes.Buffer{}, stderr)
	if code != 1 {
		t.Errorf("bad pattern: expected exit 1, got %d", code)
	}
	if !strings.Contains(stderr.String(), "loading rules") {
		t.Errorf("expected loading error, got %q", stderr.String())
	}
}

func TestAppCheckOutsideRepoErrors(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("GIT_CEILING_DIRECTORIES", filepath.Dir(dir))
	t.Chdir(dir)
	rules := ruleDirWith(t, "```claudiao-check\nid: x\nseverity: minor\npattern: foo\n```\n")

	stderr := &bytes.Buffer{}
	code := app([]string{"claudiao", "check", "--rules", rules}, &bytes.Buffer{}, &bytes.Buffer{}, stderr)
	if code != 1 {
		t.Errorf("outside repo: expected exit 1, got %d", code)
	}
	if !strings.Contains(stderr.String(), "git diff") {
		t.Errorf("expected git diff error, got %q", stderr.String())
	}
}

func TestAppCheckBadFlagErrors(t *testing.T) {
	newGitRepo(t)
	code := app([]string{"claudiao", "check", "--nope"}, &bytes.Buffer{}, &bytes.Buffer{}, &bytes.Buffer{})
	if code != 1 {
		t.Errorf("unknown flag: expected exit 1, got %d", code)
	}
}

// --- mutate ----------------------------------------------------------------

func TestAppMutateNoMutableLines(t *testing.T) {
	isolateState(t)
	repo := newGitRepo(t)

	// A changed Go file with no comparison/boolean operators to flip.
	writeFile(t, repo, "pkg.go", "package pkg\n\nvar Name = \"claudiao\"\n")
	gitRun(t, repo, "add", "-A")

	stdout := &bytes.Buffer{}
	code := app([]string{"claudiao", "mutate"}, &bytes.Buffer{}, stdout, &bytes.Buffer{})
	if code != 0 {
		t.Errorf("no mutable lines: expected exit 0, got %d", code)
	}
	if !strings.Contains(stdout.String(), "no mutable changed lines") {
		t.Errorf("expected no-mutable message, got %q", stdout.String())
	}
}

func TestAppMutateNoMutableLinesJSON(t *testing.T) {
	isolateState(t)
	repo := newGitRepo(t)
	writeFile(t, repo, "pkg.go", "package pkg\n\nvar Name = \"claudiao\"\n")
	gitRun(t, repo, "add", "-A")

	stdout := &bytes.Buffer{}
	code := app([]string{"claudiao", "mutate", "--json"}, &bytes.Buffer{}, stdout, &bytes.Buffer{})
	if code != 0 {
		t.Errorf("no mutable lines (json): expected exit 0, got %d", code)
	}
	out := stdout.String()
	if !strings.Contains(out, `"survivors":[]`) || !strings.Contains(out, "no mutable changed lines") {
		t.Errorf("expected empty-survivor JSON, got %q", out)
	}
}

// goRepoWithTest builds a minimal Go module repo whose committed code passes
// its test, then stages a change introducing a mutable comparison covered by
// the test. Mutating it must make the test fail (the mutant is killed).
func goRepoWithTest(t *testing.T) string {
	repo := newGitRepo(t)
	writeFile(t, repo, "go.mod", "module mut\n\ngo 1.24\n")
	writeFile(t, repo, "calc.go", "package mut\n\nfunc Positive(n int) bool {\n\treturn n > 0\n}\n")
	writeFile(t, repo, "calc_test.go",
		"package mut\n\nimport \"testing\"\n\nfunc TestPositive(t *testing.T) {\n"+
			"\tif Positive(1) != true {\n\t\tt.Fatal(\"1 must be positive\")\n\t}\n"+
			"\tif Positive(-1) != false {\n\t\tt.Fatal(\"-1 must not be positive\")\n\t}\n}\n")
	gitRun(t, repo, "add", "-A")
	gitRun(t, repo, "commit", "-q", "-m", "add calc")

	// Stage a change on the comparison line so it shows up in `git diff HEAD`
	// as a mutable added line. The behavior is unchanged; only the diff matters.
	writeFile(t, repo, "calc.go", "package mut\n\nfunc Positive(n int) bool {\n\treturn n > 0 // guarded\n}\n")
	gitRun(t, repo, "add", "-A")
	return repo
}

func TestAppMutateKillsCoveredMutant(t *testing.T) {
	isolateState(t)
	goRepoWithTest(t)

	stdout := &bytes.Buffer{}
	code := app([]string{"claudiao", "mutate", "--max", "1", "--timeout", "90s"},
		&bytes.Buffer{}, stdout, &bytes.Buffer{})
	if code != 0 {
		t.Fatalf("covered mutant should be killed (exit 0), got %d; output:\n%s", code, stdout.String())
	}
	if !strings.Contains(stdout.String(), "killed") {
		t.Errorf("expected a 'killed' report, got %q", stdout.String())
	}
}

func TestAppMutateReportsSurvivorJSON(t *testing.T) {
	isolateState(t)
	repo := newGitRepo(t)
	// A Go module with a mutable line but NO test exercising it: the mutant
	// survives, and --json must report it with exit 1.
	writeFile(t, repo, "go.mod", "module surv\n\ngo 1.24\n")
	writeFile(t, repo, "calc.go", "package surv\n\nfunc Positive(n int) bool {\n\treturn n > 0\n}\n")
	writeFile(t, repo, "calc_test.go",
		"package surv\n\nimport \"testing\"\n\nfunc TestNothing(t *testing.T) {}\n")
	gitRun(t, repo, "add", "-A")
	gitRun(t, repo, "commit", "-q", "-m", "add calc")
	writeFile(t, repo, "calc.go", "package surv\n\nfunc Positive(n int) bool {\n\treturn n > 0 // x\n}\n")
	gitRun(t, repo, "add", "-A")

	stdout := &bytes.Buffer{}
	code := app([]string{"claudiao", "mutate", "--json", "--max", "1", "--timeout", "90s"},
		&bytes.Buffer{}, stdout, &bytes.Buffer{})
	if code != 1 {
		t.Fatalf("surviving mutant (json) should exit 1, got %d; output:\n%s", code, stdout.String())
	}
	out := stdout.String()
	if !strings.Contains(out, `"file":"calc.go"`) {
		t.Errorf("expected survivor JSON to name calc.go, got %q", out)
	}
}

func TestAppMutateNoTestCommandErrors(t *testing.T) {
	isolateState(t)
	repo := newGitRepo(t)
	// A stack with no recognizable test runner (no go.mod/package.json/etc.),
	// but a mutable changed line so detection is reached.
	writeFile(t, repo, "calc.go", "package x\n\nfunc f(n int) bool { return n > 0 }\n")
	gitRun(t, repo, "add", "-A")

	stderr := &bytes.Buffer{}
	code := app([]string{"claudiao", "mutate"}, &bytes.Buffer{}, &bytes.Buffer{}, stderr)
	if code != 1 {
		t.Fatalf("no detectable test command: expected exit 1, got %d", code)
	}
	if !strings.Contains(stderr.String(), "could not detect a test command") {
		t.Errorf("expected detect-failure message, got %q", stderr.String())
	}
}

func TestAppMutateOutsideRepoErrors(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("GIT_CEILING_DIRECTORIES", filepath.Dir(dir))
	t.Chdir(dir)

	stderr := &bytes.Buffer{}
	code := app([]string{"claudiao", "mutate"}, &bytes.Buffer{}, &bytes.Buffer{}, stderr)
	if code != 1 {
		t.Errorf("outside repo: expected exit 1, got %d", code)
	}
	if !strings.Contains(stderr.String(), "git diff") {
		t.Errorf("expected git diff error, got %q", stderr.String())
	}
}

func TestAppMutateBadFlagErrors(t *testing.T) {
	code := app([]string{"claudiao", "mutate", "--bogus"}, &bytes.Buffer{}, &bytes.Buffer{}, &bytes.Buffer{})
	if code != 1 {
		t.Errorf("unknown flag: expected exit 1, got %d", code)
	}
}

// --- init ------------------------------------------------------------------

func TestAppInitNoStackErrors(t *testing.T) {
	t.Chdir(t.TempDir()) // empty dir, no go.mod/package.json/etc.

	stderr := &bytes.Buffer{}
	code := app([]string{"claudiao", "init"}, &bytes.Buffer{}, &bytes.Buffer{}, stderr)
	if code != 1 {
		t.Errorf("no stack: expected exit 1, got %d", code)
	}
	if !strings.Contains(stderr.String(), "no known stack detected") {
		t.Errorf("expected no-stack message, got %q", stderr.String())
	}
}

func TestAppInitDetectsGoStackAndWrites(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)

	stdout := &bytes.Buffer{}
	code := app([]string{"claudiao", "init"}, &bytes.Buffer{}, stdout, &bytes.Buffer{})
	if code != 0 {
		t.Fatalf("init go: expected exit 0, got %d", code)
	}
	out := stdout.String()
	if !strings.Contains(out, "detected stack: go") {
		t.Errorf("expected go detection, got %q", out)
	}
	if !strings.Contains(out, "wrote") {
		t.Errorf("expected a written file, got %q", out)
	}
	if _, err := os.Stat(filepath.Join(dir, ".claude", "rules", "go-checks.md")); err != nil {
		t.Errorf("expected go-checks.md written: %v", err)
	}
}

func TestAppInitSkipsExistingWithoutForce(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)

	// First run writes the files.
	if code := app([]string{"claudiao", "init"}, &bytes.Buffer{}, &bytes.Buffer{}, &bytes.Buffer{}); code != 0 {
		t.Fatalf("first init: expected exit 0, got %d", code)
	}
	// Second run must skip rather than clobber.
	stdout := &bytes.Buffer{}
	if code := app([]string{"claudiao", "init"}, &bytes.Buffer{}, stdout, &bytes.Buffer{}); code != 0 {
		t.Fatalf("second init: expected exit 0, got %d", code)
	}
	if !strings.Contains(stdout.String(), "skipped") {
		t.Errorf("expected skipped message on re-run, got %q", stdout.String())
	}
}

func TestAppInitForceOverwrites(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)
	if code := app([]string{"claudiao", "init"}, &bytes.Buffer{}, &bytes.Buffer{}, &bytes.Buffer{}); code != 0 {
		t.Fatalf("first init: expected exit 0, got %d", code)
	}

	stdout := &bytes.Buffer{}
	if code := app([]string{"claudiao", "init", "--force"}, &bytes.Buffer{}, stdout, &bytes.Buffer{}); code != 0 {
		t.Fatalf("force init: expected exit 0, got %d", code)
	}
	if !strings.Contains(stdout.String(), "wrote") {
		t.Errorf("expected re-write under --force, got %q", stdout.String())
	}
}

func TestAppInitBadFlagErrors(t *testing.T) {
	code := app([]string{"claudiao", "init", "--nope"}, &bytes.Buffer{}, &bytes.Buffer{}, &bytes.Buffer{})
	if code != 1 {
		t.Errorf("unknown flag: expected exit 1, got %d", code)
	}
}

// --- learn -----------------------------------------------------------------

func TestAppLearnNoDataRendersReport(t *testing.T) {
	isolateState(t)
	newGitRepo(t)

	stdout := &bytes.Buffer{}
	code := app([]string{"claudiao", "learn"}, &bytes.Buffer{}, stdout, &bytes.Buffer{})
	if code != 0 {
		t.Errorf("learn with no data: expected exit 0, got %d", code)
	}
	out := stdout.String()
	if !strings.Contains(out, "claudiao learn") {
		t.Errorf("expected report header, got %q", out)
	}
	if !strings.Contains(out, "No enforcement telemetry yet") {
		t.Errorf("expected empty-telemetry note, got %q", out)
	}
}

func TestAppLearnReadsStatsAndFixHistory(t *testing.T) {
	isolateState(t)
	repo := newGitRepo(t)

	// A recorded enforcement block in the isolated state dir.
	stateDir := os.Getenv("CLAUDIAO_STATE_DIR")
	statsLine := `{"event":"commit.blocked.receipt","fields":{"areas":"auth/sessions"}}` + "\n"
	if err := os.WriteFile(filepath.Join(stateDir, "stats.jsonl"), []byte(statsLine), 0o600); err != nil {
		t.Fatal(err)
	}

	// A real fix commit in history so hotspots are non-empty.
	writeFile(t, repo, "buggy.go", "package main\n")
	gitRun(t, repo, "add", "-A")
	gitRun(t, repo, "commit", "-q", "-m", "fix: correct buggy.go")

	stdout := &bytes.Buffer{}
	code := app([]string{"claudiao", "learn", "--commits", "50"}, &bytes.Buffer{}, stdout, &bytes.Buffer{})
	if code != 0 {
		t.Fatalf("learn with data: expected exit 0, got %d", code)
	}
	out := stdout.String()
	if !strings.Contains(out, "Enforcement blocks (1 events)") {
		t.Errorf("expected aggregated stats, got %q", out)
	}
	if !strings.Contains(out, "buggy.go") {
		t.Errorf("expected fix hotspot buggy.go, got %q", out)
	}
}

func TestAppLearnBadFlagErrors(t *testing.T) {
	code := app([]string{"claudiao", "learn", "--nope"}, &bytes.Buffer{}, &bytes.Buffer{}, &bytes.Buffer{})
	if code != 1 {
		t.Errorf("unknown flag: expected exit 1, got %d", code)
	}
}

// --- immune ----------------------------------------------------------------

func TestAppImmuneStatusNoLedger(t *testing.T) {
	newGitRepo(t)

	stdout := &bytes.Buffer{}
	code := app([]string{"claudiao", "immune", "status"}, &bytes.Buffer{}, stdout, &bytes.Buffer{})
	if code != 0 {
		t.Errorf("status with no ledger: expected exit 0, got %d", code)
	}
	if !strings.Contains(stdout.String(), "no immunity yet") {
		t.Errorf("expected no-immunity message, got %q", stdout.String())
	}
}

func TestAppImmuneDefaultsToStatus(t *testing.T) {
	newGitRepo(t)

	stdout := &bytes.Buffer{}
	code := app([]string{"claudiao", "immune"}, &bytes.Buffer{}, stdout, &bytes.Buffer{})
	if code != 0 {
		t.Errorf("bare immune (defaults to status): expected exit 0, got %d", code)
	}
	if !strings.Contains(stdout.String(), "no immunity yet") {
		t.Errorf("expected status output by default, got %q", stdout.String())
	}
}

func TestAppImmuneVerifyNoAntibodies(t *testing.T) {
	repo := newGitRepo(t)
	writeFile(t, repo, "x.go", "package main\n")
	gitRun(t, repo, "add", "-A")

	stdout := &bytes.Buffer{}
	code := app([]string{"claudiao", "immune", "verify"}, &bytes.Buffer{}, stdout, &bytes.Buffer{})
	if code != 0 {
		t.Errorf("verify with no antibodies: expected exit 0, got %d", code)
	}
	if !strings.Contains(stdout.String(), "no antibodies recorded yet") {
		t.Errorf("expected no-antibodies message, got %q", stdout.String())
	}
}

func TestAppImmuneScanNoFixesDryRun(t *testing.T) {
	newGitRepo(t) // only the "initial commit", no fix/revert history

	stdout := &bytes.Buffer{}
	code := app([]string{"claudiao", "immune", "scan", "--commits", "50"}, &bytes.Buffer{}, stdout, &bytes.Buffer{})
	if code != 0 {
		t.Errorf("scan with no fixes: expected exit 0, got %d", code)
	}
	if !strings.Contains(stdout.String(), "no recurring antipatterns found") {
		t.Errorf("expected no-scars message, got %q", stdout.String())
	}
}

func TestAppImmuneScanApplyRecordsAntibodies(t *testing.T) {
	repo := newGitRepo(t)

	// Two fix commits removing the same call signature → a recurring scar.
	writeFile(t, repo, "a.go", "package main\nfunc a() { db.Query(\"SELECT 1\") }\n")
	gitRun(t, repo, "add", "-A")
	gitRun(t, repo, "commit", "-q", "-m", "add a")
	writeFile(t, repo, "a.go", "package main\nfunc a() {}\n")
	gitRun(t, repo, "add", "-A")
	gitRun(t, repo, "commit", "-q", "-m", "fix: drop unsafe db.Query in a")

	writeFile(t, repo, "b.go", "package main\nfunc b() { db.Query(\"DELETE x\") }\n")
	gitRun(t, repo, "add", "-A")
	gitRun(t, repo, "commit", "-q", "-m", "add b")
	writeFile(t, repo, "b.go", "package main\nfunc b() {}\n")
	gitRun(t, repo, "add", "-A")
	gitRun(t, repo, "commit", "-q", "-m", "fix: drop unsafe db.Query in b")

	stdout := &bytes.Buffer{}
	code := app([]string{"claudiao", "immune", "scan", "--apply", "--commits", "50", "--min-hits", "2"},
		&bytes.Buffer{}, stdout, &bytes.Buffer{})
	if code != 0 {
		t.Fatalf("scan --apply: expected exit 0, got %d", code)
	}
	if !strings.Contains(stdout.String(), "immunized") {
		t.Errorf("expected immunized confirmation, got %q", stdout.String())
	}
	if _, err := os.Stat(filepath.Join(repo, ".claude", "immune", "ledger.json")); err != nil {
		t.Errorf("expected a ledger written on --apply: %v", err)
	}

	// Status now reports the recorded antibody.
	statusOut := &bytes.Buffer{}
	if code := app([]string{"claudiao", "immune", "status"}, &bytes.Buffer{}, statusOut, &bytes.Buffer{}); code != 0 {
		t.Fatalf("status after apply: expected exit 0, got %d", code)
	}
	if !strings.Contains(statusOut.String(), "protecting this repo") {
		t.Errorf("expected antibody count in status, got %q", statusOut.String())
	}
}

func TestAppImmuneVerifyDetectsReturningScar(t *testing.T) {
	repo := newGitRepo(t)

	// Three fix commits removing the same call → a scar with >=3 hits, which
	// Synthesize scales to severity "major" so the gate actually fails.
	for _, name := range []string{"a", "b", "c"} {
		f := name + ".go"
		writeFile(t, repo, f, "package main\nfunc "+name+"() { db.Query(\"SELECT 1\") }\n")
		gitRun(t, repo, "add", "-A")
		gitRun(t, repo, "commit", "-q", "-m", "add "+name)
		writeFile(t, repo, f, "package main\nfunc "+name+"() {}\n")
		gitRun(t, repo, "add", "-A")
		gitRun(t, repo, "commit", "-q", "-m", "fix: drop db.Query "+name)
	}
	if code := app([]string{"claudiao", "immune", "scan", "--apply", "--commits", "50", "--min-hits", "2"},
		&bytes.Buffer{}, &bytes.Buffer{}, &bytes.Buffer{}); code != 0 {
		t.Fatalf("scan --apply: expected exit 0, got %d", code)
	}

	// Reintroduce the antipattern in a new staged change.
	writeFile(t, repo, "d.go", "package main\nfunc d() { db.Query(\"DROP TABLE x\") }\n")
	gitRun(t, repo, "add", "d.go")

	stdout := &bytes.Buffer{}
	code := app([]string{"claudiao", "immune", "verify"}, &bytes.Buffer{}, stdout, &bytes.Buffer{})
	if code != 1 {
		t.Fatalf("returning major scar should fail verify (exit 1), got %d; output:\n%s", code, stdout.String())
	}
	if !strings.Contains(stdout.String(), "d.go") {
		t.Errorf("expected the reintroducing file d.go in the finding, got %q", stdout.String())
	}
}

func TestAppImmuneVerifyClearWhenScarAbsent(t *testing.T) {
	repo := newGitRepo(t)
	for _, name := range []string{"a", "b"} {
		f := name + ".go"
		writeFile(t, repo, f, "package main\nfunc "+name+"() { db.Query(\"SELECT 1\") }\n")
		gitRun(t, repo, "add", "-A")
		gitRun(t, repo, "commit", "-q", "-m", "add "+name)
		writeFile(t, repo, f, "package main\nfunc "+name+"() {}\n")
		gitRun(t, repo, "add", "-A")
		gitRun(t, repo, "commit", "-q", "-m", "fix: drop db.Query "+name)
	}
	if code := app([]string{"claudiao", "immune", "scan", "--apply", "--commits", "50", "--min-hits", "2"},
		&bytes.Buffer{}, &bytes.Buffer{}, &bytes.Buffer{}); code != 0 {
		t.Fatalf("scan --apply: expected exit 0, got %d", code)
	}

	// A staged change that does not reintroduce the scar.
	writeFile(t, repo, "clean.go", "package main\nfunc clean() {}\n")
	gitRun(t, repo, "add", "clean.go")

	stdout := &bytes.Buffer{}
	code := app([]string{"claudiao", "immune", "verify"}, &bytes.Buffer{}, stdout, &bytes.Buffer{})
	if code != 0 {
		t.Fatalf("clean diff should pass verify (exit 0), got %d; output:\n%s", code, stdout.String())
	}
	if !strings.Contains(stdout.String(), "clear") {
		t.Errorf("expected 'clear' message, got %q", stdout.String())
	}
}

func TestAppImmuneUnknownSubcommand(t *testing.T) {
	newGitRepo(t)

	stderr := &bytes.Buffer{}
	code := app([]string{"claudiao", "immune", "frobnicate"}, &bytes.Buffer{}, &bytes.Buffer{}, stderr)
	if code != 1 {
		t.Errorf("unknown immune subcommand: expected exit 1, got %d", code)
	}
	if !strings.Contains(stderr.String(), "usage: claudiao immune") {
		t.Errorf("expected usage message, got %q", stderr.String())
	}
}

func TestAppImmuneOutsideRepoErrors(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("GIT_CEILING_DIRECTORIES", filepath.Dir(dir))
	t.Chdir(dir)

	stderr := &bytes.Buffer{}
	code := app([]string{"claudiao", "immune", "status"}, &bytes.Buffer{}, &bytes.Buffer{}, stderr)
	if code != 1 {
		t.Errorf("outside repo: expected exit 1, got %d", code)
	}
	if !strings.Contains(stderr.String(), "immune status") {
		t.Errorf("expected error on stderr, got %q", stderr.String())
	}
}

func TestAppImmuneScanBadFlagErrors(t *testing.T) {
	newGitRepo(t)
	code := app([]string{"claudiao", "immune", "scan", "--nope"}, &bytes.Buffer{}, &bytes.Buffer{}, &bytes.Buffer{})
	if code != 1 {
		t.Errorf("unknown flag: expected exit 1, got %d", code)
	}
}

// --- stats -----------------------------------------------------------------

func TestAppStatsNoEvents(t *testing.T) {
	isolateState(t)

	stdout := &bytes.Buffer{}
	code := app([]string{"claudiao", "stats"}, &bytes.Buffer{}, stdout, &bytes.Buffer{})
	if code != 0 {
		t.Errorf("stats with no events: expected exit 0, got %d", code)
	}
	if !strings.Contains(stdout.String(), "no events recorded yet") {
		t.Errorf("expected empty-stats message, got %q", stdout.String())
	}
}

func TestAppStatsSummarizesRecordedEvents(t *testing.T) {
	isolateState(t)
	stateDir := os.Getenv("CLAUDIAO_STATE_DIR")
	jsonl := strings.Join([]string{
		`{"time":"2026-01-01T00:00:00Z","event":"commit.blocked.trailer"}`,
		`{"time":"2026-01-02T00:00:00Z","event":"receipt.verified"}`,
	}, "\n") + "\n"
	if err := os.WriteFile(filepath.Join(stateDir, "stats.jsonl"), []byte(jsonl), 0o600); err != nil {
		t.Fatal(err)
	}

	stdout := &bytes.Buffer{}
	code := app([]string{"claudiao", "stats"}, &bytes.Buffer{}, stdout, &bytes.Buffer{})
	if code != 0 {
		t.Fatalf("stats: expected exit 0, got %d", code)
	}
	out := stdout.String()
	if !strings.Contains(out, "agreement adherence — 2 events") {
		t.Errorf("expected 2-event summary, got %q", out)
	}
	if !strings.Contains(out, "commit.blocked.trailer") {
		t.Errorf("expected trailer block in summary, got %q", out)
	}
}
