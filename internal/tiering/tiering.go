// Package tiering mechanizes the effort-tiering rule: instead of the model
// deciding "did this diff cross 50 lines / touch a sensitive area?", the
// post-edit hook tracks edits per session and injects a reminder exactly
// once when a threshold is crossed.
package tiering

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/Muriel-Gasparini/claudiao/internal/sensitive"
	"github.com/Muriel-Gasparini/claudiao/internal/stats"
)

const (
	lineThreshold = 50
	fileThreshold = 2
)

type State struct {
	Files          map[string]bool `json:"files"`
	Lines          int             `json:"lines"`
	PlanWarned     bool            `json:"plan_warned"`
	SensitiveAreas map[string]bool `json:"sensitive_areas"`
}

var sessionIDRe = regexp.MustCompile(`^[A-Za-z0-9_-]{1,128}$`)

func statePath(sessionID string) (string, error) {
	dir, err := stats.Dir()
	if err != nil {
		return "", err
	}
	sdir := filepath.Join(dir, "sessions")
	if err := os.MkdirAll(sdir, 0o700); err != nil {
		return "", err
	}
	return filepath.Join(sdir, sessionID+".json"), nil
}

func load(sessionID string) (*State, error) {
	p, err := statePath(sessionID)
	if err != nil {
		return nil, err
	}
	st := &State{Files: map[string]bool{}, SensitiveAreas: map[string]bool{}}
	data, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return st, nil
		}
		return nil, err
	}
	if err := json.Unmarshal(data, st); err != nil {
		return &State{Files: map[string]bool{}, SensitiveAreas: map[string]bool{}}, nil
	}
	if st.Files == nil {
		st.Files = map[string]bool{}
	}
	if st.SensitiveAreas == nil {
		st.SensitiveAreas = map[string]bool{}
	}
	return st, nil
}

func save(sessionID string, st *State) error {
	p, err := statePath(sessionID)
	if err != nil {
		return err
	}
	data, err := json.Marshal(st)
	if err != nil {
		return err
	}
	return os.WriteFile(p, data, 0o600)
}

// RecordEdit registers one Edit/Write and returns reminders to inject —
// each reminder fires at most once per session. sessionID is untrusted
// input (it becomes a file name), so it is validated strictly.
func RecordEdit(sessionID, filePath, addedText string) ([]string, error) {
	if !sessionIDRe.MatchString(sessionID) {
		return nil, nil
	}
	st, err := load(sessionID)
	if err != nil {
		return nil, err
	}

	st.Files[filePath] = true
	if addedText != "" {
		st.Lines += strings.Count(addedText, "\n") + 1
	}

	var reminders []string

	if !st.PlanWarned && (st.Lines > lineThreshold || len(st.Files) > fileThreshold) {
		st.PlanWarned = true
		reminders = append(reminders,
			"[claudiao] This session's edits crossed the effort threshold (>50 lines or >2 files). "+
				"Per the working agreement, a short plan (TodoWrite/tasks) is required if one doesn't exist yet.")
	}

	for _, area := range sensitive.MatchPath(filePath) {
		if !st.SensitiveAreas[string(area)] {
			st.SensitiveAreas[string(area)] = true
			reminders = append(reminders,
				"[claudiao] Edit touched a sensitive area ("+string(area)+"). "+
					"The adversarial reviewer (sdd-reviewer) is mandatory before commit; "+
					"a review receipt will be required by the commit hook.")
		}
	}

	if err := save(sessionID, st); err != nil {
		return reminders, err
	}
	return reminders, nil
}
