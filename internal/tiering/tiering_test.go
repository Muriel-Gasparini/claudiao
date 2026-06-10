package tiering

import (
	"os"
	"path/filepath"
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

func TestPlanAndSensitiveRemindersFireTogether(t *testing.T) {
	t.Setenv("CLAUDIAO_STATE_DIR", t.TempDir())
	// A single edit that is both >50 lines and in a sensitive area must emit
	// both reminders at once.
	big := strings.Repeat("x\n", 60)
	rs, err := RecordEdit("combo", "internal/auth/login.go", big)
	if err != nil {
		t.Fatal(err)
	}
	if len(rs) != 2 {
		t.Fatalf("expected plan + sensitive reminders, got %d: %v", len(rs), rs)
	}
	var plan, sens bool
	for _, r := range rs {
		if strings.Contains(r, "effort threshold") {
			plan = true
		}
		if strings.Contains(r, "sensitive area") {
			sens = true
		}
	}
	if !plan || !sens {
		t.Errorf("both reminders must be present, got %v", rs)
	}
}

func TestEmptyAddedTextDoesNotCountLines(t *testing.T) {
	t.Setenv("CLAUDIAO_STATE_DIR", t.TempDir())
	// 60 edits of the same single file with no added text: lines stays 0 and
	// file count stays 1, so the plan reminder must never fire.
	for i := 0; i < 60; i++ {
		rs, err := RecordEdit("noadd", "only.go", "")
		if err != nil {
			t.Fatal(err)
		}
		if len(rs) != 0 {
			t.Fatalf("empty added text must not trigger reminders, got %v", rs)
		}
	}
}

func TestStateWithNilMapsIsRepaired(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("CLAUDIAO_STATE_DIR", dir)
	// Seed valid JSON whose maps are absent; load must repopulate them so the
	// subsequent edit does not panic on a nil-map write.
	p, err := statePath("nilmaps")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(`{"lines":0,"plan_warned":false}`), 0o600); err != nil {
		t.Fatal(err)
	}
	rs, err := RecordEdit("nilmaps", "internal/auth/x.go", "x\n")
	if err != nil {
		t.Fatalf("load must repair nil maps without erroring: %v", err)
	}
	if len(rs) != 1 || !strings.Contains(rs[0], "sensitive area") {
		t.Errorf("expected the sensitive reminder after repairing nil maps, got %v", rs)
	}
}

func TestLoadPropagatesNonNotExistReadError(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("CLAUDIAO_STATE_DIR", dir)
	// Make the session state path a directory: ReadFile fails with an error
	// that is not os.ErrNotExist, which RecordEdit must surface.
	p, err := statePath("dirstate")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(p, 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := RecordEdit("dirstate", "a.go", "x\n"); err == nil {
		t.Error("a non-not-exist read error must propagate, not be swallowed")
	}
}

func TestStatePathErrorPropagatesWhenStateDirUncreatable(t *testing.T) {
	base := t.TempDir()
	blocker := filepath.Join(base, "file")
	if err := os.WriteFile(blocker, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	// stats.Dir succeeds at returning the path but MkdirAll on it fails because
	// a parent component is a regular file; RecordEdit must return that error.
	t.Setenv("CLAUDIAO_STATE_DIR", filepath.Join(blocker, "child"))

	if _, err := RecordEdit("validid", "a.go", "x\n"); err == nil {
		t.Error("RecordEdit must propagate the statePath/MkdirAll error")
	}
}

func TestStatePersistsAcrossCalls(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("CLAUDIAO_STATE_DIR", dir)
	// First two distinct files: under the 2-file threshold, no reminder.
	if rs, err := RecordEdit("persist", "a.go", "x\n"); err != nil || len(rs) != 0 {
		t.Fatalf("first file: rs=%v err=%v", rs, err)
	}
	if rs, err := RecordEdit("persist", "b.go", "x\n"); err != nil || len(rs) != 0 {
		t.Fatalf("second file: rs=%v err=%v", rs, err)
	}
	// Third distinct file crosses >2 files; reminder relies on the first two
	// being persisted to disk between calls.
	rs, err := RecordEdit("persist", "c.go", "x\n")
	if err != nil {
		t.Fatal(err)
	}
	if len(rs) != 1 || !strings.Contains(rs[0], "effort threshold") {
		t.Errorf("file count must persist across calls; expected reminder, got %v", rs)
	}
}
