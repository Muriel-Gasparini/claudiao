package mutate

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const diff = `diff --git a/calc.go b/calc.go
--- a/calc.go
+++ b/calc.go
@@ -10,4 +10,6 @@ func clamp(v int) int {
 	x := v
+	if v >= limit && enabled {
+		return limit
+	}
 	return x
`

func TestFromDiffFindsMutableLines(t *testing.T) {
	ms := FromDiff(diff, 10)
	if len(ms) == 0 {
		t.Fatal("expected mutants from a conditional line")
	}
	m := ms[0]
	if m.File != "calc.go" {
		t.Errorf("file = %q", m.File)
	}
	if m.LineNo != 11 {
		t.Errorf("line = %d, want 11", m.LineNo)
	}
	if m.Mutated == m.Original {
		t.Error("mutant identical to original")
	}
}

func TestFromDiffRespectsCap(t *testing.T) {
	big := strings.Repeat("+\tif a == b {\n", 50)
	d := "+++ b/x.go\n@@ -1,0 +1,50 @@\n" + big
	if got := FromDiff(d, 5); len(got) != 5 {
		t.Errorf("cap not respected: %d", len(got))
	}
}

func TestFromDiffSkipsTestsAndComments(t *testing.T) {
	d := `+++ b/calc_test.go
@@ -1,0 +1,1 @@
+	if got != want {
+++ b/main.go
@@ -1,0 +1,2 @@
+	// a == b would be wrong here
+	# not code
`
	if ms := FromDiff(d, 10); len(ms) != 0 {
		t.Errorf("test files and comments must not produce mutants: %v", ms)
	}
}

func TestFromDiffSkipsUnsupportedExtensions(t *testing.T) {
	d := "+++ b/notes.md\n@@ -1,0 +1,1 @@\n+if a == b then panic\n"
	if ms := FromDiff(d, 10); len(ms) != 0 {
		t.Errorf("markdown must not produce mutants: %v", ms)
	}
}

func TestDetectTestCmd(t *testing.T) {
	dir := t.TempDir()
	if got := DetectTestCmd(dir); got != "" {
		t.Errorf("empty dir should detect nothing, got %q", got)
	}
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := DetectTestCmd(dir); got != "go test ./..." {
		t.Errorf("go project should run go test, got %q", got)
	}
}

func TestLineNumberTracksContext(t *testing.T) {
	d := `+++ b/a.go
@@ -5,4 +5,5 @@
 ctx1
 ctx2
+	if x > 0 {
 ctx3
`
	ms := FromDiff(d, 10)
	if len(ms) != 1 {
		t.Fatalf("want 1 mutant, got %d", len(ms))
	}
	if ms[0].LineNo != 7 {
		t.Errorf("line = %d, want 7 (5 + two context lines)", ms[0].LineNo)
	}
}

func TestFromDiffSkipsTSGenericAngleBrackets(t *testing.T) {
	// `<` inside Pick<>/Promise<> is a type bracket, erased at runtime —
	// mutating it is an equivalent mutant. It must NOT become a candidate.
	d := "+++ b/parse.ts\n@@ -0,0 +1,2 @@\n" +
		"+type T = Pick<User, \"id\">\n" +
		"+async function f(): Promise<void> {}\n"
	for _, m := range FromDiff(d, 10) {
		if m.Op == "< to >=" || m.Op == "> to <=" {
			t.Errorf("TS generic bracket became a mutant: %+v", m)
		}
	}
}

func TestFromDiffStillMutatesRealTSComparison(t *testing.T) {
	// A genuine comparison in TS must still be mutated.
	d := "+++ b/calc.ts\n@@ -0,0 +1 @@\n+  if (count > limit) reject()\n"
	if len(FromDiff(d, 10)) == 0 {
		t.Error("a real comparison in TS must still produce a mutant")
	}
}

func TestFromDiffMutatesAngleBracketsInGo(t *testing.T) {
	// Go has no generics-as-comparison ambiguity here; `<` comparison mutates.
	d := "+++ b/calc.go\n@@ -0,0 +1 @@\n+\tif i < n {\n"
	if len(FromDiff(d, 10)) == 0 {
		t.Error("Go comparison must still produce a mutant")
	}
}

func TestRunToleratesBinaryFileInWorkingTree(t *testing.T) {
	// Regression: a binary file (e.g. a .ts with a NUL byte) modified in the
	// working tree must not break the whole mutation run.
	dir := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		c := exec.Command("git", args...)
		c.Dir = dir
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %s", args, out)
		}
	}
	run("init", "-q")
	run("config", "user.email", "t@t")
	run("config", "user.name", "t")
	if err := os.WriteFile(filepath.Join(dir, "calc.go"), []byte("package x\nfunc f(a int) bool { return a > 0 }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "data.bin"), []byte("clean\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", ".")
	run("commit", "-qm", "init")

	// Modify both: the code (mutation target) and the binary file (NUL byte).
	if err := os.WriteFile(filepath.Join(dir, "calc.go"), []byte("package x\nfunc f(a int) bool { return a >= 0 }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "data.bin"), []byte("x\x00y\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	diff, err := exec.Command("git", "-C", dir, "diff", "HEAD").Output()
	if err != nil {
		t.Fatal(err)
	}
	mutants := FromDiff(string(diff), 5)
	if len(mutants) == 0 {
		t.Skip("no mutants from the code change; nothing to exercise")
	}
	// testCmd "true" makes every mutant survive; we only assert Run does not
	// error out applying the working diff that contains a binary file.
	if _, err := Run(dir, "true", mutants, 5*time.Second, nil); err != nil {
		t.Fatalf("Run failed on a working tree with a binary file: %v", err)
	}
}
