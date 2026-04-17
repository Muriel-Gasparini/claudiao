package main

import (
	"bytes"
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
