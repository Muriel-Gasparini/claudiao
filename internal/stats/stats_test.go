package stats

import (
	"bytes"
	"io"
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

func TestSanitizeStripsInvalidUTF8KeepingValidBytes(t *testing.T) {
	// ToValidUTF8 with "" replacement drops the bad byte entirely; the valid
	// surrounding runes survive.
	got := Sanitize("a"+string([]byte{0xff})+"b", 10)
	if got != "ab" {
		t.Errorf("invalid UTF-8 byte must be dropped, valid bytes kept: got %q", got)
	}
}

func TestSanitizeUnderLimitReturnsInput(t *testing.T) {
	if got := Sanitize("hello", 100); got != "hello" {
		t.Errorf("string under the limit must pass through unchanged: %q", got)
	}
}

func TestDirEnvOverrideCreatesDirectory(t *testing.T) {
	base := t.TempDir()
	target := filepath.Join(base, "nested", "claudiao")
	t.Setenv("CLAUDIAO_STATE_DIR", target)

	dir, err := Dir()
	if err != nil {
		t.Fatalf("Dir: %v", err)
	}
	if dir != target {
		t.Errorf("Dir = %q, want %q", dir, target)
	}
	info, err := os.Stat(target)
	if err != nil {
		t.Fatalf("Dir must create the directory: %v", err)
	}
	if !info.IsDir() {
		t.Error("Dir target exists but is not a directory")
	}
}

func TestDirUsesHomeWhenStateDirUnset(t *testing.T) {
	home := t.TempDir()
	t.Setenv("CLAUDIAO_STATE_DIR", "")
	t.Setenv("HOME", home)
	// os.UserHomeDir consults USERPROFILE on Windows; keep both pointing at the
	// fixture so the default-path branch is deterministic cross-platform.
	t.Setenv("USERPROFILE", home)

	dir, err := Dir()
	if err != nil {
		t.Fatalf("Dir: %v", err)
	}
	want := filepath.Join(home, ".claude", "claudiao")
	if dir != want {
		t.Errorf("Dir = %q, want %q", dir, want)
	}
	if info, err := os.Stat(want); err != nil || !info.IsDir() {
		t.Errorf("default state dir not created under home: err=%v", err)
	}
}

func TestDirReturnsErrorWhenStateDirUncreatable(t *testing.T) {
	base := t.TempDir()
	// A regular file occupies the path component MkdirAll needs as a parent
	// directory, so creation must fail.
	blocker := filepath.Join(base, "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CLAUDIAO_STATE_DIR", filepath.Join(blocker, "child"))

	if _, err := Dir(); err == nil {
		t.Error("expected error when the state dir cannot be created")
	}
}

func TestLogReturnsErrorWhenDirUncreatable(t *testing.T) {
	base := t.TempDir()
	blocker := filepath.Join(base, "file")
	if err := os.WriteFile(blocker, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CLAUDIAO_STATE_DIR", filepath.Join(blocker, "child"))

	if err := Log("evt", "s", "/repo", nil); err == nil {
		t.Error("Log must propagate the Dir() error when the state dir is uncreatable")
	}
}

func TestLogReturnsErrorWhenStatsFileUnopenable(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("CLAUDIAO_STATE_DIR", dir)
	// Occupy the stats.jsonl name with a directory so OpenFile for a regular
	// file fails.
	if err := os.Mkdir(filepath.Join(dir, "stats.jsonl"), 0o700); err != nil {
		t.Fatal(err)
	}

	if err := Log("evt", "s", "/repo", nil); err == nil {
		t.Error("Log must return an error when stats.jsonl cannot be opened as a file")
	}
}

func TestLogAppendsAcrossCalls(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("CLAUDIAO_STATE_DIR", dir)

	if err := Log("first", "s", "/repo", nil); err != nil {
		t.Fatal(err)
	}
	if err := Log("second", "s", "/repo", nil); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "stats.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Count(strings.TrimRight(string(data), "\n"), "\n") + 1
	if lines != 2 {
		t.Errorf("expected 2 appended lines, got %d:\n%s", lines, data)
	}
	if !strings.Contains(string(data), "first") || !strings.Contains(string(data), "second") {
		t.Errorf("both events must be present:\n%s", data)
	}
}

func TestSummarizeReportsReceiptLine(t *testing.T) {
	buf := &bytes.Buffer{}
	stream := strings.Join([]string{
		`{"time":"2026-06-01T00:00:00Z","event":"receipt.verified"}`,
		`{"time":"2026-06-02T00:00:00Z","event":"receipt.verified"}`,
		`{"time":"2026-06-03T00:00:00Z","event":"commit.blocked.receipt"}`,
	}, "\n")
	if err := Summarize(strings.NewReader(stream), buf); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "review receipts") {
		t.Errorf("receipt line missing when receipts present:\n%s", out)
	}
	if !strings.Contains(out, "2 verified / 1 blocked") {
		t.Errorf("receipt counts wrong:\n%s", out)
	}
}

func TestSummarizeSortsEventNames(t *testing.T) {
	buf := &bytes.Buffer{}
	stream := strings.Join([]string{
		`{"time":"2026-06-01T00:00:00Z","event":"zeta"}`,
		`{"time":"2026-06-01T00:00:00Z","event":"alpha"}`,
		`{"time":"2026-06-01T00:00:00Z","event":"mike"}`,
	}, "\n")
	if err := Summarize(strings.NewReader(stream), buf); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	ai := strings.Index(out, "alpha")
	mi := strings.Index(out, "mike")
	zi := strings.Index(out, "zeta")
	if !(ai < mi && mi < zi) {
		t.Errorf("event names must be sorted (alpha<mike<zeta), got positions %d,%d,%d:\n%s", ai, mi, zi, out)
	}
}

func TestSummarizeFileReportsNoEventsWhenMissing(t *testing.T) {
	t.Setenv("CLAUDIAO_STATE_DIR", t.TempDir())
	buf := &bytes.Buffer{}
	if err := SummarizeFile(buf); err != nil {
		t.Fatalf("SummarizeFile on missing file must not error: %v", err)
	}
	if !strings.Contains(buf.String(), "no events") {
		t.Errorf("missing stats file should report no events: %q", buf.String())
	}
}

func TestSummarizeFileReadsExistingEvents(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("CLAUDIAO_STATE_DIR", dir)
	if err := Log("commit.blocked.trailer", "s", "/repo", nil); err != nil {
		t.Fatal(err)
	}

	buf := &bytes.Buffer{}
	if err := SummarizeFile(buf); err != nil {
		t.Fatalf("SummarizeFile: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "commit.blocked.trailer") {
		t.Errorf("SummarizeFile must read and summarize the on-disk events:\n%s", out)
	}
	if !strings.Contains(out, "total enforcement blocks") {
		t.Errorf("enforcement summary missing:\n%s", out)
	}
}

func TestSummarizeFileReturnsErrorWhenDirUncreatable(t *testing.T) {
	base := t.TempDir()
	blocker := filepath.Join(base, "file")
	if err := os.WriteFile(blocker, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CLAUDIAO_STATE_DIR", filepath.Join(blocker, "child"))

	if err := SummarizeFile(io.Discard); err == nil {
		t.Error("SummarizeFile must propagate the Dir() error")
	}
}

func TestSummarizeFileReturnsErrorWhenStatsPathIsDir(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("CLAUDIAO_STATE_DIR", dir)
	// stats.jsonl is a directory: Open succeeds (it's a real path) but the
	// error is not IsNotExist, so SummarizeFile must surface it rather than
	// printing the "no events" message.
	if err := os.Mkdir(filepath.Join(dir, "stats.jsonl"), 0o700); err != nil {
		t.Fatal(err)
	}
	err := SummarizeFile(io.Discard)
	if err == nil {
		t.Error("SummarizeFile must error when stats.jsonl cannot be read as a file")
	}
}
