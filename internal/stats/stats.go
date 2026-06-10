// Package stats records agreement-adherence events (blocks, warnings,
// receipts, claims) as JSONL so `claudiao stats` can report whether the
// working agreement is actually being followed — behavior, not config.
package stats

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type Event struct {
	Time    time.Time         `json:"time"`
	Name    string            `json:"event"`
	Session string            `json:"session,omitempty"`
	Repo    string            `json:"repo,omitempty"`
	Fields  map[string]string `json:"fields,omitempty"`
}

// Dir returns the claudiao state directory, creating it if needed.
func Dir() (string, error) {
	if d := os.Getenv("CLAUDIAO_STATE_DIR"); d != "" {
		return d, os.MkdirAll(d, 0o700)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	d := filepath.Join(home, ".claude", "claudiao")
	return d, os.MkdirAll(d, 0o700)
}

// Log appends one event. Logging must never break a hook: callers ignore
// the error and the hook proceeds regardless.
func Log(name, session, repo string, fields map[string]string) error {
	dir, err := Dir()
	if err != nil {
		return err
	}
	ev := Event{Time: time.Now().UTC(), Name: name, Session: session, Repo: repo, Fields: fields}
	data, err := json.Marshal(ev)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(filepath.Join(dir, "stats.jsonl"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(append(data, '\n'))
	return err
}

// Summarize aggregates a stats.jsonl stream into a human report.
func Summarize(r io.Reader, w io.Writer) error {
	counts := map[string]int{}
	var first, last time.Time
	total := 0

	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		var ev Event
		if err := json.Unmarshal(sc.Bytes(), &ev); err != nil {
			continue // a corrupt line must not kill the report
		}
		counts[ev.Name]++
		total++
		if first.IsZero() || ev.Time.Before(first) {
			first = ev.Time
		}
		if ev.Time.After(last) {
			last = ev.Time
		}
	}
	if err := sc.Err(); err != nil {
		return err
	}

	if total == 0 {
		fmt.Fprintln(w, "no events recorded yet — install the Hooks module and work a few sessions")
		return nil
	}

	fmt.Fprintf(w, "agreement adherence — %d events from %s to %s\n\n",
		total, first.Format("2006-01-02"), last.Format("2006-01-02"))

	names := make([]string, 0, len(counts))
	for n := range counts {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		fmt.Fprintf(w, "  %-32s %d\n", n, counts[n])
	}

	enforced := counts["commit.blocked.trailer"] + counts["commit.blocked.receipt"] + counts["stop.blocked.claim"]
	fmt.Fprintf(w, "\n  %-32s %d\n", "total enforcement blocks", enforced)
	if counts["receipt.verified"] > 0 || counts["commit.blocked.receipt"] > 0 {
		fmt.Fprintf(w, "  %-32s %d verified / %d blocked\n", "review receipts",
			counts["receipt.verified"], counts["commit.blocked.receipt"])
	}
	return nil
}

// SummarizeFile reads the default stats file.
func SummarizeFile(w io.Writer) error {
	dir, err := Dir()
	if err != nil {
		return err
	}
	f, err := os.Open(filepath.Join(dir, "stats.jsonl"))
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Fprintln(w, "no events recorded yet — install the Hooks module and work a few sessions")
			return nil
		}
		return err
	}
	defer f.Close()
	return Summarize(f, w)
}

// Sanitize trims values destined for log fields so a hostile transcript
// cannot bloat the stats file.
func Sanitize(s string, max int) string {
	s = strings.ToValidUTF8(s, "")
	if len(s) > max {
		s = s[:max]
	}
	return s
}
