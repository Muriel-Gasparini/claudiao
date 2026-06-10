package hook

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Muriel-Gasparini/claudiao/internal/receipt"
)

func payload(t *testing.T, tool string, toolInput any, extra map[string]any) string {
	t.Helper()
	m := map[string]any{
		"session_id": "sess-hook",
		"tool_name":  tool,
		"tool_input": toolInput,
	}
	for k, v := range extra {
		m[k] = v
	}
	data, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func initRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for _, args := range [][]string{
		{"init", "-q"}, {"config", "user.email", "t@t"}, {"config", "user.name", "t"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %s", args, out)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "f.txt"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"add", "."}, {"commit", "-qm", "init"}} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %s", args, out)
		}
	}
	return dir
}

func TestPreBashBlocksTrailer(t *testing.T) {
	t.Setenv("CLAUDIAO_STATE_DIR", t.TempDir())
	in := payload(t, "Bash", map[string]string{
		"command": `git commit -m "feat: x" -m "Co-Authored-By: Claude <noreply@anthropic.com>"`,
	}, nil)
	stderr := &bytes.Buffer{}
	code := Run("pre-bash", strings.NewReader(in), &bytes.Buffer{}, stderr)
	if code != 2 {
		t.Fatalf("trailer commit must block (exit 2), got %d", code)
	}
	if !strings.Contains(stderr.String(), "trailer") {
		t.Errorf("stderr should explain the block: %q", stderr.String())
	}
}

func TestPreBashAllowsCleanCommitInCleanArea(t *testing.T) {
	t.Setenv("CLAUDIAO_STATE_DIR", t.TempDir())
	repo := initRepo(t)
	if err := os.WriteFile(filepath.Join(repo, "f.txt"), []byte("safe doc change\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	in := payload(t, "Bash", map[string]string{"command": `git commit -am "docs: tweak"`},
		map[string]any{"cwd": repo})
	if code := Run("pre-bash", strings.NewReader(in), &bytes.Buffer{}, &bytes.Buffer{}); code != 0 {
		t.Errorf("clean commit should pass, got %d", code)
	}
}

func TestPreBashBlocksSensitiveDiffWithoutReceipt(t *testing.T) {
	t.Setenv("CLAUDIAO_STATE_DIR", t.TempDir())
	repo := initRepo(t)
	if err := os.WriteFile(filepath.Join(repo, "f.txt"),
		[]byte(`token := jwt.Sign(claims, secretKey)`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	in := payload(t, "Bash", map[string]string{"command": `git commit -am "feat: auth"`},
		map[string]any{"cwd": repo})
	stderr := &bytes.Buffer{}
	code := Run("pre-bash", strings.NewReader(in), &bytes.Buffer{}, stderr)
	if code != 2 {
		t.Fatalf("sensitive diff without receipt must block, got %d (stderr %q)", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "sdd-reviewer") {
		t.Errorf("block message should point to the reviewer: %q", stderr.String())
	}
}

func TestPreBashAllowsSensitiveDiffWithReceipt(t *testing.T) {
	t.Setenv("CLAUDIAO_STATE_DIR", t.TempDir())
	repo := initRepo(t)
	if err := os.WriteFile(filepath.Join(repo, "f.txt"),
		[]byte(`token := jwt.Sign(claims, secretKey)`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := receipt.Create(repo, "sdd-reviewer", ""); err != nil {
		t.Fatal(err)
	}
	in := payload(t, "Bash", map[string]string{"command": `git commit -am "feat: auth"`},
		map[string]any{"cwd": repo})
	stderr := &bytes.Buffer{}
	if code := Run("pre-bash", strings.NewReader(in), &bytes.Buffer{}, stderr); code != 0 {
		t.Errorf("valid receipt should allow the commit, got %d (stderr %q)", code, stderr.String())
	}
}

func TestPreBashGatesGitDashCTarget(t *testing.T) {
	t.Setenv("CLAUDIAO_STATE_DIR", t.TempDir())
	clean := initRepo(t)  // cwd: clean, no sensitive diff
	target := initRepo(t) // the repo the commit actually writes to
	if err := os.WriteFile(filepath.Join(target, "f.txt"),
		[]byte(`token := jwt.Sign(claims, secretKey)`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	in := payload(t, "Bash", map[string]string{
		"command": `git -C ` + target + ` commit -am "feat: auth"`,
	}, map[string]any{"cwd": clean})
	stderr := &bytes.Buffer{}
	code := Run("pre-bash", strings.NewReader(in), &bytes.Buffer{}, stderr)
	if code != 2 {
		t.Fatalf("git -C must gate the target repo, not cwd; got %d (stderr %q)", code, stderr.String())
	}
}

func TestPreBashGatesFreshRepoInitialCommit(t *testing.T) {
	t.Setenv("CLAUDIAO_STATE_DIR", t.TempDir())
	dir := t.TempDir()
	for _, args := range [][]string{{"init", "-q"}, {"config", "user.email", "t@t"}, {"config", "user.name", "t"}} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %s", args, out)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "auth.go"),
		[]byte(`token := jwt.Sign(claims, secretKey)`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("git", "add", ".")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git add: %s", out)
	}
	in := payload(t, "Bash", map[string]string{"command": `git commit -m "feat: auth"`},
		map[string]any{"cwd": dir})
	stderr := &bytes.Buffer{}
	code := Run("pre-bash", strings.NewReader(in), &bytes.Buffer{}, stderr)
	if code != 2 {
		t.Fatalf("initial commit to a fresh repo must be gated, not fail open; got %d (stderr %q)", code, stderr.String())
	}
}

func TestPreBashGatesGitDirTarget(t *testing.T) {
	t.Setenv("CLAUDIAO_STATE_DIR", t.TempDir())
	clean := initRepo(t)
	target := initRepo(t)
	if err := os.WriteFile(filepath.Join(target, "f.txt"),
		[]byte(`token := jwt.Sign(claims, secretKey)`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	in := payload(t, "Bash", map[string]string{
		"command": `git --git-dir=` + filepath.Join(target, ".git") + ` --work-tree=` + target + ` commit -am "feat: auth"`,
	}, map[string]any{"cwd": clean})
	stderr := &bytes.Buffer{}
	code := Run("pre-bash", strings.NewReader(in), &bytes.Buffer{}, stderr)
	if code != 2 {
		t.Fatalf("--git-dir target must be gated, not fail open; got %d (stderr %q)", code, stderr.String())
	}
}

func TestPreBashGatesCumulativeDashC(t *testing.T) {
	t.Setenv("CLAUDIAO_STATE_DIR", t.TempDir())
	parent := t.TempDir()
	// A repo nested two levels below the cwd, reached via cumulative -C.
	nested := filepath.Join(parent, "a", "sensitive")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"init", "-q"}, {"config", "user.email", "t@t"}, {"config", "user.name", "t"}} {
		cmd := exec.Command("git", args...)
		cmd.Dir = nested
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %s", args, out)
		}
	}
	if err := os.WriteFile(filepath.Join(nested, "auth.go"),
		[]byte(`token := jwt.Sign(claims, secretKey)`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("git", "add", ".")
	cmd.Dir = nested
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git add: %s", out)
	}
	in := payload(t, "Bash", map[string]string{"command": `git -C a -C sensitive commit -m "feat: auth"`},
		map[string]any{"cwd": parent})
	stderr := &bytes.Buffer{}
	code := Run("pre-bash", strings.NewReader(in), &bytes.Buffer{}, stderr)
	if code != 2 {
		t.Fatalf("cumulative -C must gate the real target a/sensitive, got %d (stderr %q)", code, stderr.String())
	}
}

func TestPreBashGatesUntrackedSensitiveFile(t *testing.T) {
	t.Setenv("CLAUDIAO_STATE_DIR", t.TempDir())
	repo := initRepo(t) // has a HEAD already
	// A brand-new sensitive file, never staged — absent from `git diff HEAD`.
	if err := os.WriteFile(filepath.Join(repo, "session_store.go"),
		[]byte(`func mint() string { return jwt.Sign(claims, secretKey) }`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	in := payload(t, "Bash", map[string]string{"command": `git add session_store.go && git commit -m "feat: sessions"`},
		map[string]any{"cwd": repo})
	stderr := &bytes.Buffer{}
	code := Run("pre-bash", strings.NewReader(in), &bytes.Buffer{}, stderr)
	if code != 2 {
		t.Fatalf("untracked sensitive file must be classified and gated, got %d (stderr %q)", code, stderr.String())
	}
}

func TestPreBashCommitDashCReuseMessageStillGates(t *testing.T) {
	t.Setenv("CLAUDIAO_STATE_DIR", t.TempDir())
	repo := initRepo(t)
	if err := os.WriteFile(filepath.Join(repo, "f.txt"),
		[]byte(`token := jwt.Sign(claims, secretKey)`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// `-C HEAD` here means reuse-commit-message, NOT a directory. The gate
	// must not mistake it for a repo path and fail open.
	in := payload(t, "Bash", map[string]string{"command": `git commit -am x -C HEAD`},
		map[string]any{"cwd": repo})
	stderr := &bytes.Buffer{}
	code := Run("pre-bash", strings.NewReader(in), &bytes.Buffer{}, stderr)
	if code != 2 {
		t.Fatalf("commit -C <commit> must still gate the cwd repo, got %d (stderr %q)", code, stderr.String())
	}
}

func TestPreBashIgnoresNonRepo(t *testing.T) {
	t.Setenv("CLAUDIAO_STATE_DIR", t.TempDir())
	dir := t.TempDir()
	t.Setenv("GIT_CEILING_DIRECTORIES", filepath.Dir(dir))
	in := payload(t, "Bash", map[string]string{"command": `git commit -m "x"`},
		map[string]any{"cwd": dir})
	if code := Run("pre-bash", strings.NewReader(in), &bytes.Buffer{}, &bytes.Buffer{}); code != 0 {
		t.Errorf("a non-repo dir has nothing to enforce, must fail open; got %d", code)
	}
}

func TestPreBashIgnoresGitCommitInHeredoc(t *testing.T) {
	t.Setenv("CLAUDIAO_STATE_DIR", t.TempDir())
	repo := initRepo(t)
	// A sensitive diff: without the heredoc-aware detector, the gate would
	// treat the "git commit" inside the heredoc as a real commit and block.
	if err := os.WriteFile(filepath.Join(repo, "f.txt"),
		[]byte(`token := jwt.Sign(claims, secretKey)`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := "cat >> test_file.go << 'EOF'\nfunc x() { /* git commit -am wip */ }\nEOF\n"
	in := payload(t, "Bash", map[string]string{"command": cmd}, map[string]any{"cwd": repo})
	if code := Run("pre-bash", strings.NewReader(in), &bytes.Buffer{}, &bytes.Buffer{}); code != 0 {
		t.Errorf("a `git commit` string inside a heredoc must not be treated as a commit, got %d", code)
	}
}

func TestPreBashUnterminatedHeredocDoesNotBypassGate(t *testing.T) {
	t.Setenv("CLAUDIAO_STATE_DIR", t.TempDir())
	// bash only WARNS on an unterminated heredoc and still runs the rest of
	// the line, so a trailing real `git commit` must still be gated.
	in := payload(t, "Bash", map[string]string{
		"command": `cat <<DATA && git commit -am wip -m "Co-Authored-By: Claude <x@y>"`,
	}, nil)
	stderr := &bytes.Buffer{}
	code := Run("pre-bash", strings.NewReader(in), &bytes.Buffer{}, stderr)
	if code != 2 {
		t.Fatalf("unterminated heredoc before a real commit must NOT bypass the trailer gate, got %d", code)
	}
	if !strings.Contains(stderr.String(), "trailer") {
		t.Errorf("trailer block expected, got %q", stderr.String())
	}
}

func TestPreBashUnterminatedHeredocStillRequiresReceipt(t *testing.T) {
	t.Setenv("CLAUDIAO_STATE_DIR", t.TempDir())
	repo := initRepo(t)
	if err := os.WriteFile(filepath.Join(repo, "f.txt"),
		[]byte(`token := jwt.Sign(claims, secretKey)`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	in := payload(t, "Bash", map[string]string{
		"command": `cat <<DATA && git commit -am "feat: auth"`,
	}, map[string]any{"cwd": repo})
	stderr := &bytes.Buffer{}
	code := Run("pre-bash", strings.NewReader(in), &bytes.Buffer{}, stderr)
	if code != 2 {
		t.Fatalf("unterminated heredoc before a sensitive commit must still require a receipt, got %d (stderr %q)", code, stderr.String())
	}
}

func TestPreBashIgnoresGitCommitInQuotedString(t *testing.T) {
	t.Setenv("CLAUDIAO_STATE_DIR", t.TempDir())
	repo := initRepo(t)
	in := payload(t, "Bash", map[string]string{"command": `echo "run git commit to save"`},
		map[string]any{"cwd": repo})
	if code := Run("pre-bash", strings.NewReader(in), &bytes.Buffer{}, &bytes.Buffer{}); code != 0 {
		t.Errorf("a `git commit` inside a quoted string must not be treated as a commit, got %d", code)
	}
}

func TestPreBashStillDetectsRealCommitWithQuotedMessage(t *testing.T) {
	t.Setenv("CLAUDIAO_STATE_DIR", t.TempDir())
	repo := initRepo(t)
	if err := os.WriteFile(filepath.Join(repo, "f.txt"),
		[]byte(`token := jwt.Sign(claims, secretKey)`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Real commit with a quoted message — detection must survive stripping.
	in := payload(t, "Bash", map[string]string{"command": `git commit -am "feat: add auth"`},
		map[string]any{"cwd": repo})
	if code := Run("pre-bash", strings.NewReader(in), &bytes.Buffer{}, &bytes.Buffer{}); code != 2 {
		t.Errorf("a real commit with a quoted message must still be gated, got %d", code)
	}
}

func TestPreBashIgnoresNonCommitCommands(t *testing.T) {
	t.Setenv("CLAUDIAO_STATE_DIR", t.TempDir())
	in := payload(t, "Bash", map[string]string{"command": "ls -la && git status"}, nil)
	if code := Run("pre-bash", strings.NewReader(in), &bytes.Buffer{}, &bytes.Buffer{}); code != 0 {
		t.Errorf("non-commit command must pass, got %d", code)
	}
}

func TestEnforceOffDisablesEverything(t *testing.T) {
	t.Setenv("CLAUDIAO_STATE_DIR", t.TempDir())
	t.Setenv("CLAUDIAO_ENFORCE", "off")
	in := payload(t, "Bash", map[string]string{
		"command": `git commit -m "x" -m "Co-Authored-By: Claude"`,
	}, nil)
	if code := Run("pre-bash", strings.NewReader(in), &bytes.Buffer{}, &bytes.Buffer{}); code != 0 {
		t.Errorf("CLAUDIAO_ENFORCE=off must disable enforcement, got %d", code)
	}
}

func TestPostEditInjectsReminderOnSensitivePath(t *testing.T) {
	t.Setenv("CLAUDIAO_STATE_DIR", t.TempDir())
	in := payload(t, "Edit", map[string]string{
		"file_path":  "/repo/internal/auth/login.go",
		"new_string": "x := 1\n",
	}, nil)
	stderr := &bytes.Buffer{}
	code := Run("post-edit", strings.NewReader(in), &bytes.Buffer{}, stderr)
	if code != 2 {
		t.Fatalf("sensitive edit should surface a reminder (exit 2), got %d", code)
	}
	if !strings.Contains(stderr.String(), "sensitive area") {
		t.Errorf("reminder missing: %q", stderr.String())
	}
}

func TestStopBlocksUnbackedClaim(t *testing.T) {
	t.Setenv("CLAUDIAO_STATE_DIR", t.TempDir())
	transcript := filepath.Join(t.TempDir(), "tr.jsonl")
	line := `{"type":"assistant","message":{"content":[{"type":"text","text":"Everything is secure now."}]}}` + "\n"
	if err := os.WriteFile(transcript, []byte(line), 0o644); err != nil {
		t.Fatal(err)
	}
	in := payload(t, "", nil, map[string]any{"transcript_path": transcript})
	stdout := &bytes.Buffer{}
	if code := Run("stop", strings.NewReader(in), stdout, &bytes.Buffer{}); code != 0 {
		t.Fatalf("stop hook blocks via JSON, not exit code; got %d", code)
	}
	var out map[string]string
	if err := json.Unmarshal(stdout.Bytes(), &out); err != nil {
		t.Fatalf("expected JSON decision, got %q", stdout.String())
	}
	if out["decision"] != "block" {
		t.Errorf("expected block decision, got %v", out)
	}
}

func TestStopRespectsStopHookActive(t *testing.T) {
	t.Setenv("CLAUDIAO_STATE_DIR", t.TempDir())
	transcript := filepath.Join(t.TempDir(), "tr.jsonl")
	line := `{"type":"assistant","message":{"content":[{"type":"text","text":"Everything is secure now."}]}}` + "\n"
	if err := os.WriteFile(transcript, []byte(line), 0o644); err != nil {
		t.Fatal(err)
	}
	in := payload(t, "", nil, map[string]any{"transcript_path": transcript, "stop_hook_active": true})
	stdout := &bytes.Buffer{}
	Run("stop", strings.NewReader(in), stdout, &bytes.Buffer{})
	if stdout.Len() != 0 {
		t.Errorf("stop_hook_active must suppress blocking (no loops), got %q", stdout.String())
	}
}

func TestGarbageStdinNeverBreaksSession(t *testing.T) {
	t.Setenv("CLAUDIAO_STATE_DIR", t.TempDir())
	for _, event := range []string{"pre-bash", "post-edit", "stop", "session-start"} {
		if code := Run(event, strings.NewReader("not json"), &bytes.Buffer{}, &bytes.Buffer{}); code != 0 {
			t.Errorf("%s: garbage stdin must be a no-op, got %d", event, code)
		}
	}
}

func TestPreBashChecksBlockAntipattern(t *testing.T) {
	t.Setenv("CLAUDIAO_STATE_DIR", t.TempDir())
	t.Setenv("HOME", t.TempDir()) // isolate: no real global rules
	repo := initRepo(t)
	rulesDir := filepath.Join(repo, ".claude", "rules")
	if err := os.MkdirAll(rulesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	rule := "```claudiao-check\nid: no-eval\nseverity: blocker\npattern: eval\\(\n```\n"
	if err := os.WriteFile(filepath.Join(rulesDir, "r.md"), []byte(rule), 0o644); err != nil {
		t.Fatal(err)
	}
	in := commitPayload(t, repo, "handler.js", "var x = eval(userInput)\n")
	stderr := &bytes.Buffer{}
	code := Run("pre-bash", strings.NewReader(in), &bytes.Buffer{}, stderr)
	if code != 2 {
		t.Fatalf("blocker antipattern in production code must block the commit, got %d", code)
	}
	if !strings.Contains(stderr.String(), "no-eval") {
		t.Errorf("block message should name the check: %q", stderr.String())
	}
}

func writeRule(t *testing.T, repo, content string) {
	t.Helper()
	dir := filepath.Join(repo, ".claude", "rules")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "r.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func commitPayload(t *testing.T, repo, file, content string) string {
	t.Helper()
	if err := os.WriteFile(filepath.Join(repo, file), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	// Stage it — an untracked file is neither committed nor in `git diff HEAD`.
	cmd := exec.Command("git", "add", file)
	cmd.Dir = repo
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git add %s: %s", file, out)
	}
	return payload(t, "Bash", map[string]string{"command": `git commit -m wip`}, map[string]any{"cwd": repo})
}

func TestGateDoesNotBlockMajorOnlyFindings(t *testing.T) {
	t.Setenv("CLAUDIAO_STATE_DIR", t.TempDir())
	t.Setenv("HOME", t.TempDir())
	repo := initRepo(t)
	writeRule(t, repo, "```claudiao-check\nid: broad-major\nseverity: major\npattern: db\\.Query\\(\n```\n")
	in := commitPayload(t, repo, "q.go", "x := db.Query(stmt, arg)\n")
	if code := Run("pre-bash", strings.NewReader(in), &bytes.Buffer{}, &bytes.Buffer{}); code != 0 {
		t.Errorf("a major (advisory) finding must NOT hard-block the commit, got %d", code)
	}
}

func TestGateDoesNotBlockAntipatternInDocsOrTests(t *testing.T) {
	t.Setenv("CLAUDIAO_STATE_DIR", t.TempDir())
	t.Setenv("HOME", t.TempDir())
	repo := initRepo(t)
	writeRule(t, repo, "```claudiao-check\nid: no-eval\nseverity: blocker\npattern: eval\\(\n```\n")
	for _, f := range []string{"README.md", "parser_test.go", "tests/fixture.js"} {
		if err := os.MkdirAll(filepath.Join(repo, filepath.Dir(f)), 0o755); err != nil {
			t.Fatal(err)
		}
		in := commitPayload(t, repo, f, "an example of eval(x) here\n")
		if code := Run("pre-bash", strings.NewReader(in), &bytes.Buffer{}, &bytes.Buffer{}); code != 0 {
			t.Errorf("antipattern in %s (doc/test) must not block, got %d", f, code)
		}
	}
}

func TestGateMalformedLocalRuleKeepsGlobalChecks(t *testing.T) {
	t.Setenv("CLAUDIAO_STATE_DIR", t.TempDir())
	home := t.TempDir()
	t.Setenv("HOME", home)
	// Global rule: a real blocker.
	gdir := filepath.Join(home, ".claude", "rules")
	if err := os.MkdirAll(gdir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(gdir, "sec.md"),
		[]byte("```claudiao-check\nid: g-eval\nseverity: blocker\npattern: eval\\(\n```\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	repo := initRepo(t)
	// Local rule: malformed (unterminated fence) — must not nuke the global one.
	writeRule(t, repo, "```claudiao-check\nid: broken\nseverity: blocker\n")
	in := commitPayload(t, repo, "h.js", "var x = eval(userInput)\n")
	stderr := &bytes.Buffer{}
	code := Run("pre-bash", strings.NewReader(in), &bytes.Buffer{}, stderr)
	if code != 2 {
		t.Fatalf("a malformed LOCAL rule must not disable the GLOBAL blocker check, got %d", code)
	}
	if !strings.Contains(stderr.String(), "g-eval") {
		t.Errorf("global check should still fire: %q", stderr.String())
	}
}

func TestPreBashChecksAllowCleanDiff(t *testing.T) {
	t.Setenv("CLAUDIAO_STATE_DIR", t.TempDir())
	t.Setenv("HOME", t.TempDir())
	repo := initRepo(t)
	rulesDir := filepath.Join(repo, ".claude", "rules")
	if err := os.MkdirAll(rulesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	rule := "```claudiao-check\nid: no-eval\nseverity: blocker\npattern: eval\\(\n```\n"
	if err := os.WriteFile(filepath.Join(rulesDir, "r.md"), []byte(rule), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "f.txt"), []byte("just a clean doc line\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	in := payload(t, "Bash", map[string]string{"command": `git commit -am docs`},
		map[string]any{"cwd": repo})
	stderr := &bytes.Buffer{}
	if code := Run("pre-bash", strings.NewReader(in), &bytes.Buffer{}, stderr); code != 0 {
		t.Errorf("a clean diff must pass the check gate, got %d (stderr %q)", code, stderr.String())
	}
}

func TestSessionStartNudgesOnUnimmunizedScar(t *testing.T) {
	t.Setenv("CLAUDIAO_STATE_DIR", t.TempDir())
	repo := initRepo(t)
	q := "db." + "Query"
	writeCommit(t, repo, "a.go", "package x\nfunc h(){ "+q+"(\"a\"+id) }\n", "feat: a")
	writeCommit(t, repo, "a.go", "package x\nfunc h(){ "+q+"Row(\"a\",id) }\n", "fix: parameterize a")
	writeCommit(t, repo, "b.go", "package y\nfunc g(){ "+q+"(\"b\"+k) }\n", "feat: b")
	writeCommit(t, repo, "b.go", "package y\nfunc g(){ "+q+"Row(\"b\",k) }\n", "fix: parameterize b")

	in := payload(t, "", nil, map[string]any{"cwd": repo})
	stdout := &bytes.Buffer{}
	Run("session-start", strings.NewReader(in), stdout, &bytes.Buffer{})
	if !strings.Contains(stdout.String(), q) || !strings.Contains(stdout.String(), "immune scan") {
		t.Errorf("session-start should nudge about the unimmunized scar, got %q", stdout.String())
	}
}

func writeCommit(t *testing.T, repo, file, content, msg string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(repo, file), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"add", "."}, {"commit", "-qm", msg}} {
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %s", args, out)
		}
	}
}
