package projinit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Muriel-Gasparini/claudiao/internal/checks"
)

func TestDetectStacks(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "go.mod", "module x")
	write(t, dir, "package.json", "{}")

	got := Detect(dir)
	ids := map[string]bool{}
	for _, s := range got {
		ids[s.ID] = true
	}
	if !ids["go"] || !ids["node"] {
		t.Errorf("expected go and node, got %v", ids)
	}
}

func TestDetectDedupesPython(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "pyproject.toml", "")
	write(t, dir, "requirements.txt", "")
	got := Detect(dir)
	n := 0
	for _, s := range got {
		if s.ID == "python" {
			n++
		}
	}
	if n != 1 {
		t.Errorf("python detected %d times, want 1 (deduped)", n)
	}
}

func TestGenerateWritesChecksAndIsIdempotent(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "go.mod", "module x")

	res, err := Generate(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Written) == 0 {
		t.Fatal("nothing written")
	}
	checksFile := filepath.Join(dir, ".claude", "rules", "go-checks.md")
	if _, err := os.Stat(checksFile); err != nil {
		t.Fatalf("go-checks.md not written: %v", err)
	}

	// Second run without --force must not overwrite.
	res2, err := Generate(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(res2.Written) != 0 {
		t.Errorf("re-run without force wrote files: %v", res2.Written)
	}
	if len(res2.Skipped) == 0 {
		t.Error("re-run should report skipped files")
	}
}

func TestGeneratedChecksAreValidAndCatchAntipatterns(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "go.mod", "module x")
	if _, err := Generate(dir, false); err != nil {
		t.Fatal(err)
	}

	// The generated blocks must parse as real claudiao-check rules.
	cs, err := checks.LoadDir(filepath.Join(dir, ".claude", "rules"))
	if err != nil {
		t.Fatalf("generated checks do not parse: %v", err)
	}
	if len(cs) == 0 {
		t.Fatal("no checks parsed from generated file")
	}

	// And they must actually fire on a discarded-error line — but NOT on the
	// legitimate `_, err :=` assignment where err is used afterwards.
	diff := "+++ b/server.go\n@@ -0,0 +1 @@\n+\t_ = err\n"
	hit := false
	for _, f := range checks.RunOnDiff(cs, diff) {
		if f.Check.ID == "go-discarded-error" {
			hit = true
		}
	}
	if !hit {
		t.Error("go-discarded-error check did not fire on `_ = err`")
	}

	legit := "+++ b/ok.go\n@@ -0,0 +1 @@\n+\t_, err := doThing()\n"
	for _, f := range checks.RunOnDiff(cs, legit) {
		if f.Check.ID == "go-discarded-error" {
			t.Error("false positive: `_, err :=` (err used later) must not fire")
		}
	}
}

func TestGenerateNoStackErrors(t *testing.T) {
	dir := t.TempDir()
	if _, err := Generate(dir, false); err == nil {
		t.Error("expected error when no stack is detected")
	}
}

func TestPythonBareExceptPrecision(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "pyproject.toml", "")
	if _, err := Generate(dir, false); err != nil {
		t.Fatal(err)
	}
	cs, err := checks.LoadDir(filepath.Join(dir, ".claude", "rules"))
	if err != nil {
		t.Fatal(err)
	}
	fired := func(line string) bool {
		diff := "+++ b/x.py\n@@ -0,0 +1 @@\n+" + line + "\n"
		for _, f := range checks.RunOnDiff(cs, diff) {
			if f.Check.ID == "py-bare-except" {
				return true
			}
		}
		return false
	}
	if !fired("    except:") {
		t.Error("bare `except:` must fire")
	}
	if fired("    except ValueError:") {
		t.Error("false positive: typed `except ValueError:` must NOT fire")
	}
	if fired("    except Exception as e:") {
		t.Error("false positive: `except Exception as e:` must NOT fire")
	}
}

func TestGenerateRefusesSymlinkedRules(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "go.mod", "module x")
	outside := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".claude"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(dir, ".claude", "rules")); err != nil {
		t.Fatal(err)
	}
	if _, err := Generate(dir, false); err == nil {
		t.Error("Generate must refuse to write through a symlinked .claude/rules")
	}
	if _, err := os.Stat(filepath.Join(outside, "go-checks.md")); err == nil {
		t.Error("files must not have been written through the symlink")
	}
}

func write(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestForceOverwrites(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "go.mod", "module x")
	if _, err := Generate(dir, false); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(dir, ".claude", "rules", "go-checks.md")
	if err := os.WriteFile(target, []byte("hand-edited"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Generate(dir, true); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(target)
	if strings.Contains(string(got), "hand-edited") {
		t.Error("--force did not overwrite the file")
	}
}
