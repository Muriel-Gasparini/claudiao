package main

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

func TestHandleFlagVersion(t *testing.T) {
	version = "1.2.3"
	commit = "abc"
	date = "2026-01-01"
	for _, arg := range []string{"-v", "--version", "version"} {
		buf := &bytes.Buffer{}
		if !handleFlag([]string{"claudiao", arg}, buf) {
			t.Errorf("%q should be handled", arg)
		}
		out := buf.String()
		if !strings.Contains(out, "1.2.3") || !strings.Contains(out, "abc") || !strings.Contains(out, "2026-01-01") {
			t.Errorf("%q output missing version info: %q", arg, out)
		}
	}
}

func TestHandleFlagHelp(t *testing.T) {
	for _, arg := range []string{"-h", "--help", "help"} {
		buf := &bytes.Buffer{}
		if !handleFlag([]string{"claudiao", arg}, buf) {
			t.Errorf("%q should be handled", arg)
		}
		out := buf.String()
		for _, want := range []string{"Usage", "claudiao version", "CLAUDIAO_ASSETS_DIR"} {
			if !strings.Contains(out, want) {
				t.Errorf("%q help missing %q", arg, want)
			}
		}
	}
}

func TestHandleFlagNoArgs(t *testing.T) {
	buf := &bytes.Buffer{}
	if handleFlag([]string{"claudiao"}, buf) {
		t.Error("no args should not be handled as flag")
	}
	if buf.Len() != 0 {
		t.Errorf("expected no output, got %q", buf.String())
	}
}

func TestHandleFlagUnknownArg(t *testing.T) {
	buf := &bytes.Buffer{}
	if handleFlag([]string{"claudiao", "banana"}, buf) {
		t.Error("unknown args should not be handled")
	}
}

func TestAppHandlesFlagWithoutRunningTUI(t *testing.T) {
	called := false
	orig := runTUI
	runTUI = func() error { called = true; return nil }
	defer func() { runTUI = orig }()

	stdout := &bytes.Buffer{}
	code := app([]string{"claudiao", "version"}, stdout, &bytes.Buffer{})
	if code != 0 {
		t.Errorf("expected exit 0, got %d", code)
	}
	if called {
		t.Error("TUI should not run when a flag is handled")
	}
	if !strings.Contains(stdout.String(), "claudiao") {
		t.Errorf("expected version output, got %q", stdout.String())
	}
}

func TestAppRunsTUISuccessfully(t *testing.T) {
	orig := runTUI
	runTUI = func() error { return nil }
	defer func() { runTUI = orig }()

	code := app([]string{"claudiao"}, &bytes.Buffer{}, &bytes.Buffer{})
	if code != 0 {
		t.Errorf("expected exit 0 on successful TUI run, got %d", code)
	}
}

func TestAppReturnsOneOnTUIError(t *testing.T) {
	orig := runTUI
	runTUI = func() error { return errors.New("boom") }
	defer func() { runTUI = orig }()

	stderr := &bytes.Buffer{}
	code := app([]string{"claudiao"}, &bytes.Buffer{}, stderr)
	if code != 1 {
		t.Errorf("expected exit 1 on TUI error, got %d", code)
	}
	if !strings.Contains(stderr.String(), "boom") {
		t.Errorf("expected error in stderr, got %q", stderr.String())
	}
}
