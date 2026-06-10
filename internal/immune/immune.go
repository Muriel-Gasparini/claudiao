// Package immune is the repository's immune system: it learns from the repo's
// own scars. By reading the diffs of past fix/revert commits, it finds the
// antipatterns that have already caused bugs here, and turns each recurring one
// into a permanent, deterministic antibody (a claudiao-check) that blocks the
// same mistake from coming back. The expensive part (deciding what hurt) is
// mined once from history; the resulting antibodies run for free forever.
package immune

import (
	"crypto/sha1"
	"encoding/hex"
	"regexp"
	"sort"
	"strings"
)

// Scar is an antipattern that has already hurt this repo: a piece of code
// removed across one or more fix/revert commits.
type Scar struct {
	Signature string   `json:"signature"` // normalized token identifying the antipattern
	Sample    string   `json:"sample"`    // a real removed line that produced it
	Hits      int      `json:"hits"`      // number of distinct fix commits that removed it
	Files     []string `json:"files"`     // where it was removed
}

// Antibody is the permanent, deterministic defense synthesized from a scar.
type Antibody struct {
	ID       string `json:"id"`
	Pattern  string `json:"pattern"`  // regex that matches the antipattern returning
	Severity string `json:"severity"` // blocker | major | minor (scaled by hits)
	Scar     Scar   `json:"scar"`
}

type fixCommit struct {
	sha     string
	removed []removedLine
}

type removedLine struct {
	file string
	text string
}

var (
	// A line whose removal in a fix is interesting carries behavior: it has a
	// call. Pure structure (braces, returns of literals) is ignored.
	callRe = regexp.MustCompile(`[A-Za-z_][A-Za-z0-9_.]*\s*\(`)
	// Signature = the longest call-ish token on the line, with string/number
	// literals blanked so variants of the same mistake group together.
	stringLit = regexp.MustCompile(`"(?:[^"\\]|\\.)*"|'(?:[^'\\]|\\.)*'`)
	numberLit = regexp.MustCompile(`\b\d+(\.\d+)?\b`)
	commentRe = regexp.MustCompile(`^\s*(//|#|\*|/\*)`)
)

// DetectScars parses the output of:
//
//	git log -p --no-merges --format=%x00%H -n <N>  (filtered to fix/revert)
//
// and returns antipatterns removed in at least minHits distinct fix commits,
// most-hit first. Only fix/revert commits should be in the input — those are
// where the code admits it got something wrong.
func DetectScars(gitLogP string, minHits int) []Scar {
	if minHits < 1 {
		minHits = 1
	}
	commits := parseLogP(gitLogP)

	type agg struct {
		sample string
		shas   map[string]bool
		files  map[string]bool
	}
	groups := map[string]*agg{}
	for _, c := range commits {
		// A signature counts once per commit, no matter how many lines.
		seenInCommit := map[string]bool{}
		for _, rl := range c.removed {
			sig := signature(rl.text)
			if sig == "" || seenInCommit[sig] {
				continue
			}
			seenInCommit[sig] = true
			g := groups[sig]
			if g == nil {
				g = &agg{sample: strings.TrimSpace(rl.text), shas: map[string]bool{}, files: map[string]bool{}}
				groups[sig] = g
			}
			g.shas[c.sha] = true
			if rl.file != "" {
				g.files[rl.file] = true
			}
		}
	}

	var scars []Scar
	for sig, g := range groups {
		if len(g.shas) < minHits {
			continue
		}
		scars = append(scars, Scar{
			Signature: sig,
			Sample:    g.sample,
			Hits:      len(g.shas),
			Files:     sortedKeys(g.files),
		})
	}
	sort.Slice(scars, func(i, j int) bool {
		if scars[i].Hits != scars[j].Hits {
			return scars[i].Hits > scars[j].Hits
		}
		return scars[i].Signature < scars[j].Signature
	})
	return scars
}

func parseLogP(raw string) []fixCommit {
	var commits []fixCommit
	var cur *fixCommit
	file := ""
	for _, line := range strings.Split(raw, "\n") {
		if strings.HasPrefix(line, "\x00") {
			if cur != nil {
				commits = append(commits, *cur)
			}
			cur = &fixCommit{sha: strings.TrimPrefix(line, "\x00")}
			file = ""
			continue
		}
		if cur == nil {
			continue
		}
		switch {
		case strings.HasPrefix(line, "+++ b/"):
			file = strings.TrimPrefix(line, "+++ b/")
		case strings.HasPrefix(line, "---") || strings.HasPrefix(line, "+++"):
			// diff header, not a removed line
		case strings.HasPrefix(line, "-"):
			cur.removed = append(cur.removed, removedLine{file: file, text: line[1:]})
		}
	}
	if cur != nil {
		commits = append(commits, *cur)
	}
	return commits
}

const minSignatureLen = 3

// signature reduces a removed line to a stable key: the call that anchors the
// statement. It prefers the LEFT-MOST TOP-LEVEL call (the one the statement is
// really about) over a longer incidental or argument-nested call — `longest`
// routinely picked the wrong target (e.g. `compute` inside `assert(x ==
// compute(y))`). Returns "" for lines not worth tracking.
func signature(line string) string {
	t := strings.TrimSpace(line)
	if t == "" || commentRe.MatchString(t) || len(t) < 4 || !callRe.MatchString(t) {
		return ""
	}
	norm := numberLit.ReplaceAllString(stringLit.ReplaceAllString(t, `""`), `0`)

	best := ""
	bestDepth, bestPos := 1<<30, 1<<30
	for _, loc := range callRe.FindAllStringIndex(norm, -1) {
		name := strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(norm[loc[0]:loc[1]]), "("))
		if isNoiseCall(name) || len(name) < minSignatureLen {
			continue
		}
		d := parenDepthAt(norm, loc[0])
		if d < bestDepth || (d == bestDepth && loc[0] < bestPos) {
			best, bestDepth, bestPos = name, d, loc[0]
		}
	}
	return best
}

// parenDepthAt counts unclosed '(' before pos — the nesting depth of a call.
func parenDepthAt(s string, pos int) int {
	d := 0
	for i := 0; i < pos && i < len(s); i++ {
		switch s[i] {
		case '(':
			d++
		case ')':
			if d > 0 {
				d--
			}
		}
	}
	return d
}

// isNoiseCall filters control-flow and ubiquitous calls that would produce
// useless, high-false-positive antibodies. Case-insensitive: real code carries
// both `make` (builtin) and `String`/`fmt.Sprintf` (PascalCase/qualified).
func isNoiseCall(name string) bool {
	switch strings.ToLower(name) {
	case "if", "for", "while", "switch", "return", "func", "catch",
		"print", "println", "printf", "len", "cap", "append", "make", "new",
		"string", "sprint", "sprintf", "fmt.sprint", "fmt.sprintf":
		return true
	}
	return false
}

func sortedKeys(m map[string]bool) []string {
	var out []string
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// Synthesize turns a scar into an antibody. The pattern is a literal match of
// the call signature (QuoteMeta guarantees a valid regex even from hostile
// input), so the check fires whenever that call returns. It is intentionally
// broad — it watches the call, not the specific dangerous use; precision is the
// immunologist's semantic-test job. Severity scales with how many times the
// repo was bitten, never reaching blocker.
func Synthesize(s Scar) Antibody {
	pat := regexp.QuoteMeta(s.Signature) + `\s*\(`
	// A deterministic antibody matches the whole call, so it is broad and can
	// fire on legitimate use of that call. It therefore never escalates to
	// blocker — failing the gate on a legitimate call would just train the user
	// to ignore the check. Precision (distinguishing the dangerous use) is the
	// immunologist's job, via a semantic regression test. Severity scales only
	// minor → major, by how many times the repo was actually bitten.
	sev := "minor"
	if s.Hits >= 3 {
		sev = "major"
	}
	sum := sha1.Sum([]byte(s.Signature))
	return Antibody{
		ID:       "immune-" + sanitizeID(s.Signature) + "-" + hex.EncodeToString(sum[:3]),
		Pattern:  pat,
		Severity: sev,
		Scar:     s,
	}
}

var idUnsafe = regexp.MustCompile(`[^a-z0-9]+`)

func sanitizeID(sig string) string {
	id := idUnsafe.ReplaceAllString(strings.ToLower(sig), "-")
	id = strings.Trim(id, "-")
	if len(id) > 32 {
		id = id[:32]
	}
	if id == "" {
		id = "x"
	}
	return id
}
