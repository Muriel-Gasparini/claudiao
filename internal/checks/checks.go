// Package checks makes rules self-verifying: a rule markdown file can embed
// fenced ```claudiao-check blocks declaring antipattern regexes, and
// `claudiao check` runs every declared check against the added lines of the
// current diff. One file is both the documentation the model reads and the
// test the binary executes.
package checks

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type Severity string

const (
	SevBlocker Severity = "blocker"
	SevMajor   Severity = "major"
	SevMinor   Severity = "minor"
)

type Check struct {
	ID       string
	Severity Severity
	Pattern  *regexp.Regexp
	Files    string // glob on the file base name; empty = all files
	Source   string // rule file the check came from
}

type Finding struct {
	Check Check
	File  string
	Line  string
}

const fenceOpen = "```claudiao-check"

// ParseFile extracts the claudiao-check blocks from one markdown rule file.
// The format is a deliberate small subset of YAML (key: value per line) so
// the binary stays dependency-free.
func ParseFile(name string, content []byte) ([]Check, error) {
	var out []Check
	lines := strings.Split(string(content), "\n")
	for i := 0; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) != fenceOpen {
			continue
		}
		fields := map[string]string{}
		j := i + 1
		for ; j < len(lines); j++ {
			l := strings.TrimSpace(lines[j])
			if l == "```" {
				break
			}
			if l == "" || strings.HasPrefix(l, "#") {
				continue
			}
			k, v, ok := strings.Cut(l, ":")
			if !ok {
				return nil, fmt.Errorf("%s: malformed check line %d: %q", name, j+1, l)
			}
			fields[strings.TrimSpace(k)] = strings.TrimSpace(v)
		}
		if j == len(lines) {
			return nil, fmt.Errorf("%s: unterminated claudiao-check block at line %d", name, i+1)
		}
		i = j

		c, err := build(name, fields)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, nil
}

func build(source string, fields map[string]string) (Check, error) {
	id := fields["id"]
	if id == "" {
		return Check{}, fmt.Errorf("%s: check without id", source)
	}
	pat := fields["pattern"]
	if pat == "" {
		return Check{}, fmt.Errorf("%s: check %q without pattern", source, id)
	}
	re, err := regexp.Compile(pat)
	if err != nil {
		return Check{}, fmt.Errorf("%s: check %q: bad pattern: %w", source, id, err)
	}
	sev := Severity(fields["severity"])
	switch sev {
	case SevBlocker, SevMajor, SevMinor:
	case "":
		sev = SevMajor
	default:
		return Check{}, fmt.Errorf("%s: check %q: unknown severity %q", source, id, sev)
	}
	return Check{ID: id, Severity: sev, Pattern: re, Files: fields["files"], Source: source}, nil
}

// LoadDir parses every .md file in a rules directory.
func LoadDir(dir string) ([]Check, error) {
	var out []Check
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".md") {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		cs, err := ParseFile(filepath.Base(path), data)
		if err != nil {
			return err
		}
		out = append(out, cs...)
		return nil
	})
	return out, err
}

// RunOnDiff applies the checks to the added lines of a unified diff.
func RunOnDiff(cs []Check, diff string) []Finding {
	var findings []Finding
	file := ""
	for _, line := range strings.Split(diff, "\n") {
		if strings.HasPrefix(line, "+++ b/") {
			file = strings.TrimPrefix(line, "+++ b/")
			continue
		}
		if !strings.HasPrefix(line, "+") || strings.HasPrefix(line, "+++") {
			continue
		}
		content := line[1:]
		for _, c := range cs {
			if c.Files != "" {
				if ok, _ := filepath.Match(c.Files, filepath.Base(file)); !ok {
					continue
				}
			}
			if c.Pattern.MatchString(content) {
				findings = append(findings, Finding{Check: c, File: file, Line: strings.TrimSpace(content)})
			}
		}
	}
	return findings
}

// Report prints findings and returns the exit code (1 when any blocker or
// major finding exists).
func Report(findings []Finding, w io.Writer) int {
	if len(findings) == 0 {
		fmt.Fprintln(w, "claudiao check: no findings")
		return 0
	}
	fail := false
	for _, f := range findings {
		fmt.Fprintf(w, "[%s] %s — %s\n    %s (from %s)\n",
			f.Check.Severity, f.Check.ID, f.File, truncate(f.Line, 160), f.Check.Source)
		if f.Check.Severity != SevMinor {
			fail = true
		}
	}
	if fail {
		return 1
	}
	return 0
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
