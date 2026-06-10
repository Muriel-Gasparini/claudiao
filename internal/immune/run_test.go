package immune

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// git runs a git subcommand in dir, failing the test on error.
func git(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
}

// writeFile writes content to repo/name.
func writeFile(t *testing.T, repo, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(repo, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// commitFile stages name with the given content and commits it with msg.
func commitFile(t *testing.T, repo, name, content, msg string) {
	t.Helper()
	writeFile(t, repo, name, content)
	git(t, repo, "add", name)
	git(t, repo, "commit", "-m", msg)
}

// scarredRepo builds a git repo whose history removed `db.Query` across three
// independent "fix:" commits — a real scar with 3 hits → a "major" antibody.
// The chronology per file: a feat adds db.Query("..."+id); a fix replaces it
// with the parameterized form, removing the db.Query call line.
func scarredRepo(t *testing.T) string {
	t.Helper()
	repo := t.TempDir()
	git(t, repo, "init", "-q")
	git(t, repo, "config", "commit.gpgsign", "false")

	for _, f := range []string{"a", "b", "c"} {
		bad := "package p\n\nfunc run" + f + "(id string) {\n\tdb.Query(\"SELECT \" + id)\n}\n"
		good := "package p\n\nfunc run" + f + "(id string) {\n\tdb.QueryRow(\"SELECT ?\", id)\n}\n"
		commitFile(t, repo, f+".go", bad, "feat: add "+f)
		commitFile(t, repo, f+".go", good, "fix: parameterize "+f)
	}
	return repo
}

func TestRepoRootResolvesWorkTree(t *testing.T) {
	repo := scarredRepo(t)
	sub := filepath.Join(repo, "nested")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}

	root, err := RepoRoot(sub)
	if err != nil {
		t.Fatalf("RepoRoot from inside repo: %v", err)
	}

	// macOS /var → /private/var and similar prefixes make the raw strings
	// differ; compare canonical paths.
	gotReal, _ := filepath.EvalSymlinks(root)
	wantReal, _ := filepath.EvalSymlinks(repo)
	if gotReal != wantReal {
		t.Errorf("RepoRoot = %q, want %q", root, repo)
	}
}

func TestRepoRootErrorsOutsideRepo(t *testing.T) {
	if _, err := RepoRoot(t.TempDir()); err == nil {
		t.Error("RepoRoot must error outside any git repository")
	}
}

func TestScanDryRunListsScarWithoutWriting(t *testing.T) {
	t.Setenv("CLAUDIAO_STATE_DIR", t.TempDir())
	repo := scarredRepo(t)

	var out bytes.Buffer
	if err := Scan(repo, 20, 2, false, &out); err != nil {
		t.Fatalf("Scan dry run: %v", err)
	}

	if !strings.Contains(out.String(), "db.Query") {
		t.Errorf("dry run should report the db.Query scar, got:\n%s", out.String())
	}
	if !strings.Contains(out.String(), "dry run") {
		t.Errorf("dry run should announce itself, got:\n%s", out.String())
	}
	// A dry run must not materialize anything.
	if _, err := os.Stat(ledgerPath(repo)); !os.IsNotExist(err) {
		t.Error("dry run wrote the ledger")
	}
	if _, err := os.Stat(checksPath(repo)); !os.IsNotExist(err) {
		t.Error("dry run wrote the checks file")
	}
}

func TestScanWithNoScarsReportsNothingFound(t *testing.T) {
	t.Setenv("CLAUDIAO_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	git(t, repo, "init", "-q")
	git(t, repo, "config", "commit.gpgsign", "false")
	commitFile(t, repo, "x.go", "package p\n", "feat: nothing to fix")

	var out bytes.Buffer
	if err := Scan(repo, 20, 2, false, &out); err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if !strings.Contains(out.String(), "no recurring antipatterns") {
		t.Errorf("empty history should report no scars, got:\n%s", out.String())
	}
	if _, err := os.Stat(ledgerPath(repo)); !os.IsNotExist(err) {
		t.Error("a no-scar scan must not write a ledger")
	}
}

func TestScanApplyMaterializesLedgerAndChecks(t *testing.T) {
	t.Setenv("CLAUDIAO_STATE_DIR", t.TempDir())
	repo := scarredRepo(t)

	var out bytes.Buffer
	if err := Scan(repo, 20, 2, true, &out); err != nil {
		t.Fatalf("Scan apply: %v", err)
	}
	if !strings.Contains(out.String(), "immunized") {
		t.Errorf("apply should announce immunization, got:\n%s", out.String())
	}

	// The ledger now holds the db.Query antibody.
	led, err := Load(repo)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, a := range led.Antibodies {
		if a.Scar.Signature == "db.Query" && a.Scar.Hits == 3 {
			found = true
		}
	}
	if !found {
		t.Errorf("apply did not persist the db.Query (3 hits) antibody: %+v", led.Antibodies)
	}

	// The generated rules file exists and re-parses as a claudiao-check that
	// fires on the antipattern returning.
	data, err := os.ReadFile(checksPath(repo))
	if err != nil {
		t.Fatalf("apply did not write the checks file: %v", err)
	}
	if !strings.Contains(string(data), "db\\.Query") {
		t.Errorf("checks file missing the rendered db.Query pattern:\n%s", data)
	}
}

func TestDetectNewSurfacesUnimmunizedThenGoesQuietAfterApply(t *testing.T) {
	t.Setenv("CLAUDIAO_STATE_DIR", t.TempDir())
	repo := scarredRepo(t)

	before, err := DetectNew(repo, 20, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(before) != 1 || before[0].Signature != "db.Query" {
		t.Fatalf("before apply DetectNew should surface the db.Query scar, got %+v", before)
	}

	var sink bytes.Buffer
	if err := Scan(repo, 20, 2, true, &sink); err != nil {
		t.Fatal(err)
	}

	after, err := DetectNew(repo, 20, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(after) != 0 {
		t.Errorf("after immunizing, DetectNew should be empty, got %+v", after)
	}
}

func TestStatusSummarizesImmunity(t *testing.T) {
	t.Setenv("CLAUDIAO_STATE_DIR", t.TempDir())
	repo := scarredRepo(t)

	var empty bytes.Buffer
	if err := Status(repo, &empty); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(empty.String(), "no immunity yet") {
		t.Errorf("an unscanned repo should report no immunity, got:\n%s", empty.String())
	}

	var sink bytes.Buffer
	if err := Scan(repo, 20, 2, true, &sink); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	if err := Status(repo, &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "immune-db-query") {
		t.Errorf("status should list the db.Query antibody by id, got:\n%s", out.String())
	}
	if !strings.Contains(out.String(), "bitten 3x") {
		t.Errorf("status should report the 3 hits, got:\n%s", out.String())
	}
}

func TestVerifyFiresWhenScarReturnsAndClearsOtherwise(t *testing.T) {
	t.Setenv("CLAUDIAO_STATE_DIR", t.TempDir())
	repo := scarredRepo(t)

	var sink bytes.Buffer
	if err := Scan(repo, 20, 2, true, &sink); err != nil {
		t.Fatal(err)
	}

	// A diff reintroducing db.Query (the 3-hit scar → "major") must fail.
	returning := "+++ b/d.go\n@@ -0,0 +1 @@\n+\tdb.Query(\"SELECT \" + id)\n"
	var fired bytes.Buffer
	code, err := Verify(repo, returning, &fired)
	if err != nil {
		t.Fatal(err)
	}
	if code != 1 {
		t.Errorf("a returning major scar must exit non-zero, got %d:\n%s", code, fired.String())
	}
	if !strings.Contains(fired.String(), "immune-db-query") {
		t.Errorf("verify should name the returning scar, got:\n%s", fired.String())
	}

	// A diff with no scar is clear.
	clean := "+++ b/d.go\n@@ -0,0 +1 @@\n+\tfmt.Println(\"ok\")\n"
	var ok bytes.Buffer
	code, err = Verify(repo, clean, &ok)
	if err != nil {
		t.Fatal(err)
	}
	if code != 0 {
		t.Errorf("a clean diff must exit zero, got %d", code)
	}
	if !strings.Contains(ok.String(), "clear") {
		t.Errorf("verify should report clear, got:\n%s", ok.String())
	}
}

func TestVerifyWithNoAntibodiesIsNoop(t *testing.T) {
	t.Setenv("CLAUDIAO_STATE_DIR", t.TempDir())
	repo := scarredRepo(t)

	diff := "+++ b/d.go\n@@ -0,0 +1 @@\n+\tdb.Query(\"x\" + id)\n"
	var out bytes.Buffer
	code, err := Verify(repo, diff, &out)
	if err != nil {
		t.Fatal(err)
	}
	if code != 0 {
		t.Errorf("with no recorded antibodies verify must exit zero, got %d", code)
	}
	if !strings.Contains(out.String(), "no antibodies recorded") {
		t.Errorf("verify should say nothing is recorded yet, got:\n%s", out.String())
	}
}

func TestRunCommandsErrorOutsideRepo(t *testing.T) {
	dir := t.TempDir() // not a git repo

	if err := Scan(dir, 20, 2, false, &bytes.Buffer{}); err == nil {
		t.Error("Scan must error outside a repo")
	}
	if err := Status(dir, &bytes.Buffer{}); err == nil {
		t.Error("Status must error outside a repo")
	}
	if _, err := DetectNew(dir, 20, 2); err == nil {
		t.Error("DetectNew must error outside a repo")
	}
	if _, err := Verify(dir, "", &bytes.Buffer{}); err == nil {
		t.Error("Verify must error outside a repo")
	}
}
