package stats

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLogAndSummarize(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("CLAUDIAO_STATE_DIR", dir)

	events := []string{"commit.blocked.trailer", "commit.blocked.trailer", "receipt.verified", "stop.checked"}
	for _, e := range events {
		if err := Log(e, "s1", "/repo", map[string]string{"k": "v"}); err != nil {
			t.Fatal(err)
		}
	}

	f, err := os.Open(filepath.Join(dir, "stats.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	buf := &bytes.Buffer{}
	if err := Summarize(f, buf); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{"commit.blocked.trailer", "2", "receipt.verified", "total enforcement blocks"} {
		if !strings.Contains(out, want) {
			t.Errorf("summary missing %q:\n%s", want, out)
		}
	}
}

func TestSummarizeEmptyAndCorrupt(t *testing.T) {
	buf := &bytes.Buffer{}
	if err := Summarize(strings.NewReader(""), buf); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "no events") {
		t.Errorf("empty summary message missing: %q", buf.String())
	}

	buf.Reset()
	mixed := "garbage line\n" + `{"time":"2026-06-09T00:00:00Z","event":"commit.ok"}` + "\n"
	if err := Summarize(strings.NewReader(mixed), buf); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "commit.ok") {
		t.Errorf("corrupt lines must be skipped, valid ones kept: %q", buf.String())
	}
}

func TestSanitize(t *testing.T) {
	if got := Sanitize("abc", 2); got != "ab" {
		t.Errorf("Sanitize truncation failed: %q", got)
	}
	if got := Sanitize(string([]byte{0xff, 'a'}), 10); strings.ContainsRune(got, 0xfffd) == false && got != "a" {
		t.Errorf("invalid UTF-8 not cleaned: %q", got)
	}
}
