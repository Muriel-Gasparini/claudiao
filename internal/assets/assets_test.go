package assets

import (
	"errors"
	"io/fs"
	"strings"
	"testing"
	"testing/fstest"
)

func TestFSReturnsNonNil(t *testing.T) {
	if FS() == nil {
		t.Fatal("FS() returned nil")
	}
}

func TestFSContainsExpectedTopLevel(t *testing.T) {
	want := map[string]bool{
		"CLAUDE.md":     true,
		"rules":         true,
		"agents":        true,
		"output-styles": true,
	}
	entries, err := fs.ReadDir(FS(), ".")
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	got := map[string]bool{}
	for _, e := range entries {
		got[e.Name()] = true
	}
	for name := range want {
		if !got[name] {
			t.Errorf("expected top-level %q in embedded FS", name)
		}
	}
}

func TestFSRulesDirHasRealFiles(t *testing.T) {
	entries, err := fs.ReadDir(FS(), "rules")
	if err != nil {
		t.Fatalf("ReadDir rules: %v", err)
	}
	count := 0
	for _, e := range entries {
		if !e.IsDir() {
			count++
		}
	}
	if count < 10 {
		t.Errorf("expected at least 10 rule files, got %d", count)
	}
}

func TestFSClaudeMdHasContent(t *testing.T) {
	data, err := fs.ReadFile(FS(), "CLAUDE.md")
	if err != nil {
		t.Fatalf("ReadFile CLAUDE.md: %v", err)
	}
	if len(data) < 100 {
		t.Errorf("CLAUDE.md suspiciously small: %d bytes", len(data))
	}
}

func TestReviewerAgentKeepsReceiptContract(t *testing.T) {
	data, err := fs.ReadFile(FS(), "agents/sdd-reviewer.md")
	if err != nil {
		t.Fatalf("ReadFile agents/sdd-reviewer.md: %v", err)
	}
	content := string(data)
	wantReceipt := `claudiao receipt create --reviewer sdd-reviewer --summary "blocker:N major:N minor:N"`
	if !strings.Contains(content, wantReceipt) {
		t.Errorf("sdd-reviewer.md lost the receipt command the commit gate depends on: %q", wantReceipt)
	}
	if strings.Contains(content, "git diff --stat") || strings.Contains(content, "git diff -U20 -- ") {
		t.Error("sdd-reviewer.md uses bare git diff; must diff against HEAD or staged changes are invisible to the review")
	}
}

func TestMustSubReturnsFS(t *testing.T) {
	want := fstest.MapFS{"a.md": {Data: []byte("x")}}
	got := mustSub(want, nil)
	if got == nil {
		t.Fatal("expected fs, got nil")
	}
	if data, err := fs.ReadFile(got, "a.md"); err != nil || string(data) != "x" {
		t.Errorf("unexpected fs contents: data=%q err=%v", string(data), err)
	}
}

func TestMustSubPanicsOnError(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic")
		}
		msg, _ := r.(string)
		if msg == "" {
			t.Errorf("expected string panic, got %T: %v", r, r)
		}
	}()
	mustSub(nil, errors.New("boom"))
}
