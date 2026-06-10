// Package learn closes the feedback loop: it mines the enforcement telemetry
// (stats.jsonl) and the repository's own fix history to surface where bugs
// actually cluster, and suggests claudiao-check patterns grounded in real
// data rather than guesswork.
package learn

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
)

type StatsSummary struct {
	BlocksByType map[string]int
	AreaBlocks   map[string]int // sensitive area -> times it blocked a commit
	Total        int
}

type event struct {
	Name   string            `json:"event"`
	Fields map[string]string `json:"fields"`
}

// AggregateStats folds a stats.jsonl stream into block counts and the
// sensitive areas that most often gated a commit.
func AggregateStats(r io.Reader) StatsSummary {
	s := StatsSummary{BlocksByType: map[string]int{}, AreaBlocks: map[string]int{}}
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	for sc.Scan() {
		var e event
		if json.Unmarshal(sc.Bytes(), &e) != nil {
			continue
		}
		s.Total++
		if strings.Contains(e.Name, "blocked") {
			s.BlocksByType[e.Name]++
		}
		if areas := e.Fields["areas"]; areas != "" && strings.Contains(e.Name, "blocked") {
			for _, a := range strings.Split(areas, ", ") {
				if a = strings.TrimSpace(a); a != "" {
					s.AreaBlocks[a]++
				}
			}
		}
	}
	return s
}

type FixCommit struct {
	Subject string
	Files   []string
}

// ParseGitLog parses the output of:
//
//	git log --no-merges --format=%x00%s --name-only -n <N>
//
// Each record starts with a NUL byte, then the subject line, then the changed
// files until the next NUL. Only fix/revert commits are returned — they are
// where the codebase admits it got something wrong.
func ParseGitLog(raw string) []FixCommit {
	var out []FixCommit
	records := strings.Split(raw, "\x00")
	for _, rec := range records {
		rec = strings.TrimLeft(rec, "\n")
		if rec == "" {
			continue
		}
		lines := strings.Split(rec, "\n")
		subject := strings.TrimSpace(lines[0])
		if !isFix(subject) {
			continue
		}
		var files []string
		for _, l := range lines[1:] {
			if l = strings.TrimSpace(l); l != "" {
				files = append(files, unquotePath(l))
			}
		}
		out = append(out, FixCommit{Subject: subject, Files: files})
	}
	return out
}

func isFix(subject string) bool {
	s := strings.ToLower(subject)
	return strings.HasPrefix(s, "fix") || strings.HasPrefix(s, "revert") || strings.HasPrefix(s, "hotfix")
}

// unquotePath collapses git's C-style quoting (non-ASCII paths are wrapped in
// double quotes with octal escapes) so the same physical file keys the same
// hotspot regardless of how a given commit happened to render it. Go's string
// syntax decodes git's octal/standard escapes; on any failure keep the raw
// form rather than dropping the file.
func unquotePath(s string) string {
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		if unq, err := strconv.Unquote(s); err == nil {
			return unq
		}
	}
	return s
}

type Hotspot struct {
	File  string
	Fixes int
}

// Hotspots ranks files by how often they appear in fix commits.
func Hotspots(commits []FixCommit, top int) []Hotspot {
	count := map[string]int{}
	for _, c := range commits {
		for _, f := range c.Files {
			count[f]++
		}
	}
	var hs []Hotspot
	for f, n := range count {
		hs = append(hs, Hotspot{File: f, Fixes: n})
	}
	sort.Slice(hs, func(i, j int) bool {
		if hs[i].Fixes != hs[j].Fixes {
			return hs[i].Fixes > hs[j].Fixes
		}
		return hs[i].File < hs[j].File
	})
	if top > 0 && len(hs) > top {
		hs = hs[:top]
	}
	return hs
}

// Report renders the human-facing summary and suggestions.
func Report(s StatsSummary, fixes []FixCommit, w io.Writer) {
	fmt.Fprint(w, "# claudiao learn — grounded in your enforcement data and fix history\n\n")

	if s.Total == 0 {
		fmt.Fprintln(w, "No enforcement telemetry yet (install the Hooks module and work a few sessions).")
	} else {
		fmt.Fprintf(w, "## Enforcement blocks (%d events)\n\n", s.Total)
		printCounts(w, s.BlocksByType)
		if len(s.AreaBlocks) > 0 {
			fmt.Fprintln(w, "\nSensitive areas that most often gated a commit:")
			printCounts(w, s.AreaBlocks)
			fmt.Fprintln(w, "\n→ These are your recurring risk surfaces. The reviewer should look hardest here.")
		}
	}

	hs := Hotspots(fixes, 10)
	fmt.Fprintf(w, "\n## Fix hotspots (%d fix/revert commits)\n\n", len(fixes))
	if len(hs) == 0 {
		fmt.Fprintln(w, "No fix/revert commits found in the recent history.")
	} else {
		for _, h := range hs {
			fmt.Fprintf(w, "  %3d  %s\n", h.Fixes, h.File)
		}
		fmt.Fprintln(w, "\n→ Files fixed repeatedly are bug-prone. Consider a targeted test suite or a")
		fmt.Fprintln(w, "  claudiao-check for the pattern that keeps breaking. Add it to")
		fmt.Fprintln(w, "  .claude/rules/ (see `claudiao init`) so the next regression is caught mechanically.")
	}
}

func printCounts(w io.Writer, m map[string]int) {
	type kv struct {
		k string
		v int
	}
	var items []kv
	for k, v := range m {
		items = append(items, kv{k, v})
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].v != items[j].v {
			return items[i].v > items[j].v
		}
		return items[i].k < items[j].k
	})
	for _, it := range items {
		fmt.Fprintf(w, "  %3d  %s\n", it.v, it.k)
	}
}
