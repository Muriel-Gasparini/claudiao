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
