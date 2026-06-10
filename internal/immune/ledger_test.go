package immune

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Muriel-Gasparini/claudiao/internal/checks"
)

func sampleAntibody(id string, hits int) Antibody {
	return Antibody{
		ID:       id,
		Pattern:  `db\.Query\s*\(`,
		Severity: "major",
		Scar:     Scar{Signature: "db.Query", Sample: `db.Query("x" + y)`, Hits: hits},
	}
}

func TestLedgerAddReportsOnlyNew(t *testing.T) {
	l := &Ledger{Antibodies: map[string]Antibody{}}
	added := l.Add([]Antibody{sampleAntibody("a", 2), sampleAntibody("b", 3)})
	if len(added) != 2 {
		t.Fatalf("first add should report 2 new, got %d", len(added))
	}
	added = l.Add([]Antibody{sampleAntibody("a", 5)})
	if len(added) != 0 {
		t.Errorf("re-adding existing id must not report new, got %d", len(added))
	}
	if l.Antibodies["a"].Scar.Hits != 5 {
		t.Errorf("existing antibody should refresh in place (hits 2→5), got %d", l.Antibodies["a"].Scar.Hits)
	}
}

func TestLedgerSaveLoadRoundtrip(t *testing.T) {
	repo := t.TempDir()
	l := &Ledger{Antibodies: map[string]Antibody{}}
	l.Add([]Antibody{sampleAntibody("immune-x", 2)})
	if err := l.Save(repo); err != nil {
		t.Fatal(err)
	}
	got, err := Load(repo)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := got.Antibodies["immune-x"]; !ok {
		t.Error("antibody lost across save/load")
	}
}

func TestRenderedChecksAreParseable(t *testing.T) {
	// The whole point: the generated file must be valid claudiao-check input.
	repo := t.TempDir()
	l := &Ledger{Antibodies: map[string]Antibody{}}
	l.Add([]Antibody{sampleAntibody("immune-dbquery-abc", 2)})
	if err := l.Save(repo); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(checksPath(repo))
	if err != nil {
		t.Fatal(err)
	}
	cs, err := checks.ParseFile("immune-checks.md", data)
	if err != nil {
		t.Fatalf("generated immune checks do not parse: %v", err)
	}
	if len(cs) != 1 || cs[0].ID != "immune-dbquery-abc" {
		t.Errorf("parsed checks wrong: %+v", cs)
	}
	// And it must actually fire on the antipattern.
	diff := "+++ b/x.go\n@@ -0,0 +1 @@\n+\tdb.Query(\"a\" + b)\n"
	if len(checks.RunOnDiff(cs, diff)) == 0 {
		t.Error("rendered antibody did not fire on the antipattern returning")
	}
}

func TestLedgerLoadMissingIsEmpty(t *testing.T) {
	l, err := Load(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if len(l.Antibodies) != 0 {
		t.Errorf("missing ledger should load empty, got %d", len(l.Antibodies))
	}
}

func TestSaveRefusesSymlinkedClaude(t *testing.T) {
	repo := t.TempDir()
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(repo, ".claude")); err != nil {
		t.Fatal(err)
	}
	l := &Ledger{Antibodies: map[string]Antibody{}}
	l.Add([]Antibody{sampleAntibody("immune-x", 2)})
	if err := l.Save(repo); err == nil {
		t.Error("Save must refuse to write through a symlinked .claude")
	}
	// Nothing may have been created through the link.
	if _, err := os.Stat(filepath.Join(outside, "immune")); err == nil {
		t.Error("Save created files outside the repo via the symlink before refusing")
	}
}

func TestRenderSkipsUncompilablePattern(t *testing.T) {
	repo := t.TempDir()
	l := &Ledger{Antibodies: map[string]Antibody{}}
	good := sampleAntibody("immune-good", 2)
	bad := Antibody{ID: "immune-bad", Pattern: `([unterminated`, Severity: "minor", Scar: Scar{Signature: "x", Hits: 1}}
	l.Add([]Antibody{good, bad})
	if err := l.Save(repo); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(checksPath(repo))
	cs, err := checks.ParseFile("immune-checks.md", data)
	if err != nil {
		t.Fatalf("a corrupt ledger entry broke the whole checks file: %v", err)
	}
	if len(cs) != 1 || cs[0].ID != "immune-good" {
		t.Errorf("expected only the good antibody rendered, got %+v", cs)
	}
}
