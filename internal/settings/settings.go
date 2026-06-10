// Package settings merges the claudiao hook entries into
// ~/.claude/settings.json without disturbing anything else the user has
// configured there. The merge is idempotent: re-installing never duplicates
// entries, and stale claudiao entries (old binary path) are replaced.
package settings

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type hookSpec struct {
	event   string
	matcher string
	command string
}

func specs(binPath string) []hookSpec {
	quoted := binPath
	if strings.ContainsAny(binPath, " \t") {
		quoted = `"` + binPath + `"`
	}
	return []hookSpec{
		{"PreToolUse", "Bash", quoted + " hook pre-bash"},
		{"PostToolUse", "Edit|Write|MultiEdit", quoted + " hook post-edit"},
		{"Stop", "", quoted + " hook stop"},
		{"SessionStart", "", quoted + " hook session-start"},
	}
}

// MergeHooks installs the three claudiao hooks into the settings file at
// claudePath/settings.json, preserving every unknown field.
func MergeHooks(claudePath, binPath string) error {
	settingsPath := filepath.Join(claudePath, "settings.json")

	root := map[string]any{}
	data, err := os.ReadFile(settingsPath)
	switch {
	case err == nil:
		if err := json.Unmarshal(data, &root); err != nil {
			return fmt.Errorf("settings.json exists but is not valid JSON — fix it manually first: %w", err)
		}
	case os.IsNotExist(err):
		// fresh file
	default:
		return err
	}

	var hooks map[string]any
	switch h := root["hooks"].(type) {
	case nil:
		hooks = map[string]any{}
	case map[string]any:
		hooks = h
	default:
		return fmt.Errorf("settings.json has a \"hooks\" field of unexpected type %T — "+
			"refusing to overwrite it; fix or remove it manually first", root["hooks"])
	}

	for _, s := range specs(binPath) {
		existing := hooks[s.event]
		entries, ok := asEntryList(existing)
		if !ok {
			return fmt.Errorf("settings.json hooks[%q] is of unexpected type %T — "+
				"refusing to overwrite it; fix or remove it manually first", s.event, existing)
		}
		hooks[s.event] = upsert(entries, s)
	}
	root["hooks"] = hooks

	out, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(claudePath, 0o755); err != nil {
		return err
	}
	tmp := settingsPath + ".claudiao.tmp"
	if err := os.WriteFile(tmp, append(out, '\n'), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, settingsPath)
}

func asEntryList(v any) ([]any, bool) {
	switch e := v.(type) {
	case nil:
		return nil, true
	case []any:
		return e, true
	default:
		return nil, false
	}
}

// upsert replaces any existing claudiao entry for this event/matcher and
// appends ours if none was found. User-defined entries are never touched.
func upsert(entries []any, s hookSpec) []any {
	ours := map[string]any{
		"matcher": s.matcher,
		"hooks":   []any{map[string]any{"type": "command", "command": s.command}},
	}
	for i, e := range entries {
		entry, ok := e.(map[string]any)
		if !ok {
			continue
		}
		if isClaudiaoEntry(entry) && matcherOf(entry) == s.matcher {
			entries[i] = ours
			return entries
		}
	}
	return append(entries, ours)
}

func matcherOf(entry map[string]any) string {
	m, _ := entry["matcher"].(string)
	return m
}

func isClaudiaoEntry(entry map[string]any) bool {
	inner, _ := entry["hooks"].([]any)
	for _, h := range inner {
		hm, ok := h.(map[string]any)
		if !ok {
			continue
		}
		cmd, _ := hm["command"].(string)
		if strings.Contains(cmd, "claudiao") && strings.Contains(cmd, " hook ") {
			return true
		}
	}
	return false
}
