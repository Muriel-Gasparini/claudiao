package immune

import (
	"regexp"
	"strings"
	"testing"
)

// logRecord builds one NUL-prefixed `git log -p` record with removed lines.
func logRecord(sha string, file string, removed ...string) string {
	var b strings.Builder
	b.WriteString("\x00" + sha + "\n")
	b.WriteString("diff --git a/" + file + " b/" + file + "\n")
	b.WriteString("--- a/" + file + "\n+++ b/" + file + "\n")
	b.WriteString("@@ -1,2 +1,2 @@\n")
	for _, r := range removed {
		b.WriteString("-" + r + "\n")
	}
	b.WriteString("+something else\n")
	return b.String()
}

func TestDetectScarsFindsRecurringRemoval(t *testing.T) {
	log := logRecord("aaa", "h.go", `db.Query("SELECT " + id)`) +
		logRecord("bbb", "g.go", `db.Query("DELETE " + k)`)
	scars := DetectScars(log, 2)
	if len(scars) != 1 {
		t.Fatalf("want 1 scar, got %d: %+v", len(scars), scars)
	}
	if scars[0].Signature != "db.Query" {
		t.Errorf("signature = %q, want db.Query", scars[0].Signature)
	}
	if scars[0].Hits != 2 {
		t.Errorf("hits = %d, want 2", scars[0].Hits)
	}
	if len(scars[0].Files) != 2 {
		t.Errorf("files = %v, want 2 distinct", scars[0].Files)
	}
}

func TestDetectScarsRespectsMinHits(t *testing.T) {
	log := logRecord("aaa", "h.go", `dangerous("x")`)
	if scars := DetectScars(log, 2); len(scars) != 0 {
		t.Errorf("a single occurrence must not become a scar with min-hits 2, got %v", scars)
	}
	if scars := DetectScars(log, 1); len(scars) != 1 {
		t.Errorf("min-hits 1 should surface it, got %v", scars)
	}
}

func TestDetectScarsDedupsWithinOneCommit(t *testing.T) {
	// Same signature removed twice in ONE commit counts as one hit, not two.
	log := logRecord("aaa", "h.go", `db.Query("a" + x)`, `db.Query("b" + y)`)
	scars := DetectScars(log, 1)
	if len(scars) != 1 || scars[0].Hits != 1 {
		t.Errorf("intra-commit dedup failed: %+v", scars)
	}
}

func TestSignatureIgnoresNoiseAndComments(t *testing.T) {
	cases := map[string]string{
		`if (x > 0) {`:           "", // control flow only
		`// db.Query("x")`:       "", // comment
		`}`:                      "", // trivial
		`return foo()`:           "foo",
		`  result := compute(a)`: "compute",
	}
	for line, want := range cases {
		if got := signature(line); got != want {
			t.Errorf("signature(%q) = %q, want %q", line, got, want)
		}
	}
}

func TestSignatureBlanksLiteralsSoVariantsGroup(t *testing.T) {
	a := signature(`db.Exec("INSERT 1")`)
	b := signature(`db.Exec("DELETE 99")`)
	if a == "" || a != b {
		t.Errorf("literal-varying calls must share a signature: %q vs %q", a, b)
	}
}

func TestSynthesizeProducesValidScaledAntibody(t *testing.T) {
	for _, tc := range []struct {
		hits int
		sev  string
	}{{1, "minor"}, {2, "minor"}, {3, "major"}, {5, "major"}} {
		ab := Synthesize(Scar{Signature: "db.Query", Sample: "x", Hits: tc.hits})
		if ab.Severity != tc.sev {
			t.Errorf("hits=%d severity=%q want %q", tc.hits, ab.Severity, tc.sev)
		}
		if _, err := regexp.Compile(ab.Pattern); err != nil {
			t.Errorf("antibody pattern does not compile: %q: %v", ab.Pattern, err)
		}
		if !strings.HasPrefix(ab.ID, "immune-") {
			t.Errorf("id should be namespaced: %q", ab.ID)
		}
	}
}

func TestSynthesizeQuotesHostileSignature(t *testing.T) {
	// A signature with regex metacharacters must still yield a valid, literal pattern.
	ab := Synthesize(Scar{Signature: "a.b[(", Sample: "x", Hits: 2})
	re, err := regexp.Compile(ab.Pattern)
	if err != nil {
		t.Fatalf("hostile signature broke the pattern: %v", err)
	}
	if !re.MatchString("a.b[(  (") {
		t.Errorf("pattern should match the literal call: %q", ab.Pattern)
	}
	if re.MatchString("axbZ(") {
		t.Errorf("metacharacters must be literal, not wildcards: %q", ab.Pattern)
	}
}

func TestSignaturePrefersTopLevelCall(t *testing.T) {
	cases := map[string]string{
		`assert(x == compute(reallyLongName))`:  "assert",    // not the longer inner compute
		`result = unrelated() + danger(secret)`: "unrelated", // left-most top-level
		`logger.WithField("k", v).Error(err)`:   "logger.WithField",
	}
	for line, want := range cases {
		if got := signature(line); got != want {
			t.Errorf("signature(%q) = %q, want %q", line, got, want)
		}
	}
}

func TestSignatureRejectsShortAndNoisyCalls(t *testing.T) {
	for _, line := range []string{
		`result = a(b(c()))`,        // single-letter signature → too short
		`x := make([]int, 0)`,       // builtin noise
		`s := fmt.Sprintf("%d", n)`, // ubiquitous formatting noise
	} {
		if got := signature(line); got != "" {
			t.Errorf("signature(%q) = %q, want empty (short/noise)", line, got)
		}
	}
}
