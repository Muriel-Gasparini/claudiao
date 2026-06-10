package settings

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func readSettings(t *testing.T, dir string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, "settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	var root map[string]any
	if err := json.Unmarshal(data, &root); err != nil {
		t.Fatalf("settings.json invalid after merge: %v", err)
	}
	return root
}

func hookCommands(t *testing.T, root map[string]any) []string {
	t.Helper()
	var cmds []string
	hooks, _ := root["hooks"].(map[string]any)
	for _, entries := range hooks {
		list, _ := entries.([]any)
		for _, e := range list {
			em, _ := e.(map[string]any)
			inner, _ := em["hooks"].([]any)
			for _, h := range inner {
				hm, _ := h.(map[string]any)
				if c, ok := hm["command"].(string); ok {
					cmds = append(cmds, c)
				}
			}
		}
	}
	return cmds
}

func TestMergeIntoFreshDir(t *testing.T) {
	dir := t.TempDir()
	if err := MergeHooks(dir, "/usr/local/bin/claudiao"); err != nil {
		t.Fatal(err)
	}
	root := readSettings(t, dir)
	cmds := hookCommands(t, root)
	if len(cmds) != 4 {
		t.Fatalf("want 4 hook commands, got %v", cmds)
	}
	for _, c := range cmds {
		if !strings.HasPrefix(c, "/usr/local/bin/claudiao hook ") {
			t.Errorf("unexpected command %q", c)
		}
	}
}

func TestMergeIsIdempotent(t *testing.T) {
	dir := t.TempDir()
	for i := 0; i < 3; i++ {
		if err := MergeHooks(dir, "/bin/claudiao"); err != nil {
			t.Fatal(err)
		}
	}
	if cmds := hookCommands(t, readSettings(t, dir)); len(cmds) != 4 {
		t.Errorf("re-merge must not duplicate, got %d commands", len(cmds))
	}
}

func TestMergeUpdatesStaleBinaryPath(t *testing.T) {
	dir := t.TempDir()
	if err := MergeHooks(dir, "/old/claudiao"); err != nil {
		t.Fatal(err)
	}
	if err := MergeHooks(dir, "/new/claudiao"); err != nil {
		t.Fatal(err)
	}
	for _, c := range hookCommands(t, readSettings(t, dir)) {
		if strings.HasPrefix(c, "/old/") {
			t.Errorf("stale path survived merge: %q", c)
		}
	}
}

func TestMergePreservesUserContent(t *testing.T) {
	dir := t.TempDir()
	existing := `{
  "model": "opus",
  "hooks": {
    "PreToolUse": [
      {"matcher": "Bash", "hooks": [{"type": "command", "command": "my-own-guard.sh"}]}
    ]
  },
  "permissions": {"allow": ["Bash(ls:*)"]}
}`
	if err := os.WriteFile(filepath.Join(dir, "settings.json"), []byte(existing), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := MergeHooks(dir, "/bin/claudiao"); err != nil {
		t.Fatal(err)
	}
	root := readSettings(t, dir)
	if root["model"] != "opus" {
		t.Error("unrelated top-level field lost")
	}
	if _, ok := root["permissions"]; !ok {
		t.Error("permissions lost")
	}
	cmds := hookCommands(t, root)
	foundUser := false
	for _, c := range cmds {
		if c == "my-own-guard.sh" {
			foundUser = true
		}
	}
	if !foundUser {
		t.Errorf("user hook entry lost: %v", cmds)
	}
	if len(cmds) != 5 {
		t.Errorf("want user hook + 4 claudiao hooks, got %v", cmds)
	}
}

func TestMergeRefusesInvalidJSON(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "settings.json"), []byte("{broken"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := MergeHooks(dir, "/bin/claudiao"); err == nil {
		t.Error("must refuse to overwrite an unparseable settings.json")
	}
}

func TestMergeQuotesPathsWithSpaces(t *testing.T) {
	dir := t.TempDir()
	if err := MergeHooks(dir, "/home/u/My Apps/claudiao"); err != nil {
		t.Fatal(err)
	}
	cmds := hookCommands(t, readSettings(t, dir))
	for _, c := range cmds {
		if !strings.HasPrefix(c, `"/home/u/My Apps/claudiao" hook `) {
			t.Errorf("path with spaces must be quoted: %q", c)
		}
	}
	if err := MergeHooks(dir, "/home/u/My Apps/claudiao"); err != nil {
		t.Fatal(err)
	}
	if cmds := hookCommands(t, readSettings(t, dir)); len(cmds) != 4 {
		t.Errorf("quoted entries must still be idempotent, got %d", len(cmds))
	}
}

func TestMergeRefusesNonObjectHooks(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "settings.json"),
		[]byte(`{"hooks": [1,2,3], "model": "opus"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := MergeHooks(dir, "/bin/claudiao"); err == nil {
		t.Fatal("must refuse when hooks is an array, not silently overwrite")
	}
	data, _ := os.ReadFile(filepath.Join(dir, "settings.json"))
	if !strings.Contains(string(data), "1,2,3") {
		t.Error("original hooks value must be preserved on refusal")
	}
}

func TestMergeRefusesNonListEvent(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "settings.json"),
		[]byte(`{"hooks": {"PreToolUse": {"oops": true}}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := MergeHooks(dir, "/bin/claudiao"); err == nil {
		t.Error("must refuse when an event value is an object, not a list")
	}
}
