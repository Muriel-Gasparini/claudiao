package checks

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const ruleMD = "# Rule\n\nprose here\n\n" +
	"```claudiao-check\n" +
	"id: no-trailers\n" +
	"severity: blocker\n" +
	"pattern: (?i)co-authored-by:\n" +
	"```\n\nmore prose\n\n" +
	"```claudiao-check\n" +
	"id: no-todo\n" +
	"severity: minor\n" +
	"pattern: TODO\n" +
	"files: *.go\n" +
	"```\n"

func TestParseFile(t *testing.T) {
	cs, err := ParseFile("rule.md", []byte(ruleMD))
	if err != nil {
		t.Fatal(err)
	}
	if len(cs) != 2 {
		t.Fatalf("want 2 checks, got %d", len(cs))
	}
	if cs[0].ID != "no-trailers" || cs[0].Severity != SevBlocker {
		t.Errorf("first check parsed wrong: %+v", cs[0])
	}
	if cs[1].Files != "*.go" {
		t.Errorf("files glob lost: %+v", cs[1])
	}
}

func TestParseFileDefaultsSeverity(t *testing.T) {
	cs, err := ParseFile("r.md", []byte("```claudiao-check\nid: x\npattern: y\n```\n"))
	if err != nil {
		t.Fatal(err)
	}
	if cs[0].Severity != SevMajor {
		t.Errorf("default severity should be major, got %s", cs[0].Severity)
	}
}

func TestParseFileErrors(t *testing.T) {
	cases := map[string]string{
		"no id":        "```claudiao-check\npattern: x\n```\n",
		"no pattern":   "```claudiao-check\nid: x\n```\n",
		"bad severity": "```claudiao-check\nid: x\npattern: y\nseverity: nuclear\n```\n",
		"bad regex":    "```claudiao-check\nid: x\npattern: ([\n```\n",
		"unterminated": "```claudiao-check\nid: x\npattern: y\n",
	}
	for name, content := range cases {
		if _, err := ParseFile("r.md", []byte(content)); err == nil {
			t.Errorf("%s: expected error", name)
		}
	}
}

func TestRunOnDiff(t *testing.T) {
	cs, err := ParseFile("rule.md", []byte(ruleMD))
	if err != nil {
		t.Fatal(err)
	}
	diff := `diff --git a/x.go b/x.go
--- a/x.go
+++ b/x.go
@@ -1,2 +1,3 @@
 package x
+// TODO fix this
-old := "Co-Authored-By: someone"
+msg := "Co-Authored-By: Claude"
`
	findings := RunOnDiff(cs, diff)
	ids := map[string]bool{}
	for _, f := range findings {
		ids[f.Check.ID] = true
	}
	if !ids["no-trailers"] || !ids["no-todo"] {
		t.Errorf("expected both findings, got %v", findings)
	}
}

func TestRunOnDiffFileGlob(t *testing.T) {
	cs, _ := ParseFile("rule.md", []byte(ruleMD))
	diff := "+++ b/notes.md\n@@ -0,0 +1 @@\n+TODO later\n"
	for _, f := range RunOnDiff(cs, diff) {
		if f.Check.ID == "no-todo" {
			t.Error("*.go glob must not match notes.md")
		}
	}
}

func TestLoadDir(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.md"), []byte(ruleMD), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "plain.md"), []byte("# no checks here\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cs, err := LoadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(cs) != 2 {
		t.Errorf("want 2 checks from dir, got %d", len(cs))
	}
}

func TestReportExitCodes(t *testing.T) {
	buf := &bytes.Buffer{}
	if code := Report(nil, buf); code != 0 {
		t.Errorf("no findings should exit 0, got %d", code)
	}
	blocker := Finding{Check: Check{ID: "x", Severity: SevBlocker}, File: "a.go", Line: "bad"}
	minor := Finding{Check: Check{ID: "y", Severity: SevMinor}, File: "a.go", Line: "meh"}
	if code := Report([]Finding{minor}, buf); code != 0 {
		t.Errorf("minor-only should exit 0, got %d", code)
	}
	buf.Reset()
	if code := Report([]Finding{blocker}, buf); code != 1 {
		t.Errorf("blocker should exit 1, got %d", code)
	}
	if !strings.Contains(buf.String(), "blocker") {
		t.Errorf("report should mention severity: %q", buf.String())
	}
}
