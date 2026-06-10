package learn

import (
	"bytes"
	"strings"
	"testing"
)

func TestAggregateStats(t *testing.T) {
	jsonl := strings.Join([]string{
		`{"event":"commit.blocked.receipt","fields":{"areas":"auth/sessions, crypto"}}`,
		`{"event":"commit.blocked.receipt","fields":{"areas":"auth/sessions"}}`,
		`{"event":"commit.blocked.trailer"}`,
		`{"event":"commit.ok"}`,
		`garbage line`,
	}, "\n")

	s := AggregateStats(strings.NewReader(jsonl))
	if s.Total != 4 {
		t.Errorf("Total = %d, want 4 (garbage skipped)", s.Total)
	}
	if s.BlocksByType["commit.blocked.receipt"] != 2 {
		t.Errorf("receipt blocks = %d, want 2", s.BlocksByType["commit.blocked.receipt"])
	}
	if s.AreaBlocks["auth/sessions"] != 2 {
		t.Errorf("auth blocks = %d, want 2", s.AreaBlocks["auth/sessions"])
	}
	if s.AreaBlocks["crypto"] != 1 {
		t.Errorf("crypto blocks = %d, want 1", s.AreaBlocks["crypto"])
	}
	if _, ok := s.BlocksByType["commit.ok"]; ok {
		t.Error("commit.ok is not a block and must not be counted as one")
	}
}

func TestParseGitLogOnlyFixes(t *testing.T) {
	raw := "\x00fix: null deref in auth\nauth/login.go\nauth/session.go\n" +
		"\x00feat: add dashboard\ndashboard.go\n" +
		"\x00revert: bad cache change\ncache.go\n" +
		"\x00refactor: rename\nx.go\n"

	fixes := ParseGitLog(raw)
	if len(fixes) != 2 {
		t.Fatalf("expected 2 fix/revert commits, got %d", len(fixes))
	}
	if fixes[0].Subject != "fix: null deref in auth" {
		t.Errorf("first subject = %q", fixes[0].Subject)
	}
	if len(fixes[0].Files) != 2 {
		t.Errorf("first fix files = %v", fixes[0].Files)
	}
}

func TestParseGitLogUnquotesPaths(t *testing.T) {
	// git wraps non-ASCII paths in quotes with octal escapes; the same file
	// must key the same hotspot whether quoted or not.
	raw := "\x00fix: accents\n\"src/m\\303\\251n.go\"\n" +
		"\x00fix: again\nsrc/mén.go\n"
	fixes := ParseGitLog(raw)
	if len(fixes) != 2 {
		t.Fatalf("expected 2 fixes, got %d", len(fixes))
	}
	hs := Hotspots(fixes, 5)
	if len(hs) != 1 {
		t.Fatalf("quoted and unquoted forms must collapse to one file, got %d: %+v", len(hs), hs)
	}
	if hs[0].File != "src/mén.go" || hs[0].Fixes != 2 {
		t.Errorf("hotspot = %+v, want src/mén.go with 2 fixes", hs[0])
	}
}

func TestHotspotsRanking(t *testing.T) {
	commits := []FixCommit{
		{Subject: "fix: a", Files: []string{"auth.go", "util.go"}},
		{Subject: "fix: b", Files: []string{"auth.go"}},
		{Subject: "fix: c", Files: []string{"auth.go", "db.go"}},
	}
	hs := Hotspots(commits, 2)
	if len(hs) != 2 {
		t.Fatalf("expected top 2, got %d", len(hs))
	}
	if hs[0].File != "auth.go" || hs[0].Fixes != 3 {
		t.Errorf("top hotspot = %+v, want auth.go/3", hs[0])
	}
}

func TestReportRendersBothSections(t *testing.T) {
	s := AggregateStats(strings.NewReader(
		`{"event":"commit.blocked.receipt","fields":{"areas":"crypto"}}`))
	fixes := []FixCommit{{Subject: "fix: x", Files: []string{"a.go"}}}
	buf := &bytes.Buffer{}
	Report(s, fixes, buf)
	out := buf.String()
	for _, want := range []string{"Enforcement blocks", "crypto", "Fix hotspots", "a.go"} {
		if !strings.Contains(out, want) {
			t.Errorf("report missing %q:\n%s", want, out)
		}
	}
}

func TestReportEmptyStats(t *testing.T) {
	buf := &bytes.Buffer{}
	Report(AggregateStats(strings.NewReader("")), nil, buf)
	if !strings.Contains(buf.String(), "No enforcement telemetry") {
		t.Errorf("empty report should say so: %q", buf.String())
	}
}
