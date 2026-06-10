package receipt

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func initRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	run(t, dir, "git", "init", "-q")
	run(t, dir, "git", "config", "user.email", "t@t")
	run(t, dir, "git", "config", "user.name", "t")
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("one\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run(t, dir, "git", "add", ".")
	run(t, dir, "git", "commit", "-qm", "init")
	return dir
}

func run(t *testing.T, dir string, name string, args ...string) {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%s %v: %s: %v", name, args, out, err)
	}
}

func TestCreateAndVerify(t *testing.T) {
	t.Setenv("CLAUDIAO_STATE_DIR", t.TempDir())
	repo := initRepo(t)
	if err := os.WriteFile(filepath.Join(repo, "a.txt"), []byte("changed\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	r, err := Create(repo, "sdd-reviewer", "blocker:0 major:0 minor:1")
	if err != nil {
		t.Fatal(err)
	}
	if r.Reviewer != "sdd-reviewer" || len(r.Hash) != 64 {
		t.Errorf("receipt malformed: %+v", r)
	}

	if _, err := Verify(repo); err != nil {
		t.Errorf("receipt should verify right after creation: %v", err)
	}
}

func TestVerifyDetectsStaleAfterEdit(t *testing.T) {
	t.Setenv("CLAUDIAO_STATE_DIR", t.TempDir())
	repo := initRepo(t)
	if _, err := Create(repo, "sdd-reviewer", ""); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "a.txt"), []byte("edited after review\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := Verify(repo)
	if !errors.Is(err, ErrStale) {
		t.Errorf("expected ErrStale after post-review edit, got %v", err)
	}
}

func TestVerifyDetectsStaleAfterNewUntrackedFile(t *testing.T) {
	t.Setenv("CLAUDIAO_STATE_DIR", t.TempDir())
	repo := initRepo(t)
	if _, err := Create(repo, "sdd-reviewer", ""); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "sneaky.go"), []byte("package x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := Verify(repo)
	if !errors.Is(err, ErrStale) {
		t.Errorf("a new untracked file must invalidate the receipt, got %v", err)
	}
}

func TestVerifyNoReceipt(t *testing.T) {
	t.Setenv("CLAUDIAO_STATE_DIR", t.TempDir())
	repo := initRepo(t)
	_, err := Verify(repo)
	if !errors.Is(err, ErrNoReceipt) {
		t.Errorf("expected ErrNoReceipt, got %v", err)
	}
}

func TestNotARepo(t *testing.T) {
	t.Setenv("CLAUDIAO_STATE_DIR", t.TempDir())
	dir := t.TempDir()
	t.Setenv("GIT_CEILING_DIRECTORIES", filepath.Dir(dir))
	if _, err := Create(dir, "x", ""); err == nil {
		t.Error("Create outside a git repo must fail")
	}
}
