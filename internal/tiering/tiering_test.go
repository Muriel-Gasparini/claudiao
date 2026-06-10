package tiering

import (
	"os"
	"strings"
	"testing"
)

func TestPlanReminderFiresOnceOnLineThreshold(t *testing.T) {
	t.Setenv("CLAUDIAO_STATE_DIR", t.TempDir())
	big := strings.Repeat("line\n", 60)

	rs, err := RecordEdit("sess-1", "main.go", big)
	if err != nil {
		t.Fatal(err)
	}
	if len(rs) != 1 || !strings.Contains(rs[0], "effort threshold") {
		t.Fatalf("expected one plan reminder, got %v", rs)
	}

	rs, err = RecordEdit("sess-1", "main.go", big)
	if err != nil {
		t.Fatal(err)
	}
	if len(rs) != 0 {
		t.Errorf("plan reminder must fire once per session, got %v", rs)
	}
}

func TestPlanReminderOnFileCount(t *testing.T) {
	t.Setenv("CLAUDIAO_STATE_DIR", t.TempDir())
	var got []string
	for _, f := range []string{"a.go", "b.go", "c.go"} {
		rs, err := RecordEdit("sess-2", f, "x\n")
		if err != nil {
			t.Fatal(err)
		}
		got = append(got, rs...)
	}
	if len(got) != 1 {
		t.Errorf("crossing 2 files should remind exactly once, got %v", got)
	}
}

func TestSensitiveReminderPerArea(t *testing.T) {
	t.Setenv("CLAUDIAO_STATE_DIR", t.TempDir())
	rs, err := RecordEdit("sess-3", "internal/auth/login.go", "x\n")
	if err != nil {
		t.Fatal(err)
	}
	if len(rs) != 1 || !strings.Contains(rs[0], "sensitive area") {
		t.Fatalf("expected sensitive reminder, got %v", rs)
	}
	rs, _ = RecordEdit("sess-3", "internal/auth/logout.go", "x\n")
	if len(rs) != 0 {
		t.Errorf("same area must not remind twice, got %v", rs)
	}
}

func TestInvalidSessionIDIsIgnored(t *testing.T) {
	t.Setenv("CLAUDIAO_STATE_DIR", t.TempDir())
	rs, err := RecordEdit("../../etc/passwd", "auth/login.go", "x\n")
	if err != nil {
		t.Fatal(err)
	}
	if rs != nil {
		t.Errorf("path-traversal session id must be ignored, got %v", rs)
	}
}

func TestCorruptStateRecovers(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("CLAUDIAO_STATE_DIR", dir)
	if _, err := RecordEdit("sess-4", "a.go", "x\n"); err != nil {
		t.Fatal(err)
	}
	p, err := statePath("sess-4")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte("{{{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := RecordEdit("sess-4", "b.go", "x\n"); err != nil {
		t.Errorf("corrupt state must reset, not error: %v", err)
	}
}
