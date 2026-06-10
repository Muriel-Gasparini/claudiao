// Package hook implements the Claude Code hook protocol and the three
// claudiao enforcement points: pre-bash (commit gate: trailers + review
// receipts), post-edit (automatic effort tiering) and stop (claim checker).
// The binary itself is the hook executor — no scripts, no runtime deps.
package hook

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/Muriel-Gasparini/claudiao/internal/checks"
	"github.com/Muriel-Gasparini/claudiao/internal/claims"
	"github.com/Muriel-Gasparini/claudiao/internal/immune"
	"github.com/Muriel-Gasparini/claudiao/internal/receipt"
	"github.com/Muriel-Gasparini/claudiao/internal/sensitive"
	"github.com/Muriel-Gasparini/claudiao/internal/stats"
	"github.com/Muriel-Gasparini/claudiao/internal/tiering"
)

const maxInput = 8 * 1024 * 1024

type Input struct {
	SessionID      string          `json:"session_id"`
	TranscriptPath string          `json:"transcript_path"`
	CWD            string          `json:"cwd"`
	HookEventName  string          `json:"hook_event_name"`
	ToolName       string          `json:"tool_name"`
	ToolInput      json.RawMessage `json:"tool_input"`
	StopHookActive bool            `json:"stop_hook_active"`
}

// Run dispatches one hook event. stdin carries the Claude Code JSON payload.
// Exit code contract: 0 = allow, 2 = block (stderr is fed back to Claude).
func Run(event string, stdin io.Reader, stdout, stderr io.Writer) int {
	if os.Getenv("CLAUDIAO_ENFORCE") == "off" {
		return 0
	}
	data, err := io.ReadAll(io.LimitReader(stdin, maxInput))
	if err != nil {
		return 0 // a broken hook must never break the session
	}
	var in Input
	if err := json.Unmarshal(data, &in); err != nil {
		return 0
	}

	switch event {
	case "pre-bash":
		return preBash(in, stderr)
	case "post-edit":
		return postEdit(in, stderr)
	case "stop":
		return stop(in, stdout)
	case "session-start":
		return sessionStart(in, stdout)
	default:
		fmt.Fprintf(stderr, "claudiao hook: unknown event %q\n", event)
		return 0
	}
}

var (
	gitCommitRe   = regexp.MustCompile(`\bgit\b[^|;&]*\bcommit\b`)
	trailerRe     = regexp.MustCompile(`(?i)(co-authored-by:|generated with .{0,20}claude|signed-off-by:[^\n]{0,60}claude)`)
	heredocOpenRe = regexp.MustCompile(`<<-?\s*['"]?([A-Za-z_]\w*)['"]?`)
	quotedRe      = regexp.MustCompile(`"[^"]*"|'[^']*'`)
)

// commandForDetection strips heredoc bodies and quoted strings so the commit
// detector does not fire on a literal "git commit" written INSIDE a heredoc
// (`cat <<EOF ... git commit ... EOF`) or a string (`echo "git commit"`). The
// original command is kept for the trailer check, which needs the message.
func commandForDetection(cmd string) string {
	return quotedRe.ReplaceAllString(stripHeredocs(cmd), "")
}

func stripHeredocs(cmd string) string {
	for {
		m := heredocOpenRe.FindStringSubmatchIndex(cmd)
		if m == nil {
			return cmd
		}
		delim := cmd[m[2]:m[3]]
		after := cmd[m[1]:]
		end := regexp.MustCompile(`(?m)^[ \t]*` + regexp.QuoteMeta(delim) + `[ \t]*$`).FindStringIndex(after)
		if end == nil {
			// Unterminated heredoc: bash only warns and STILL runs the rest of
			// the line (e.g. `cat <<X && git commit …`). Strip just the opener
			// so a trailing `git commit` stays visible — fail closed, never
			// drop the tail.
			cmd = cmd[:m[0]] + after
			continue
		}
		cmd = cmd[:m[0]] + after[end[1]:]
	}
}

func preBash(in Input, stderr io.Writer) int {
	var ti struct {
		Command string `json:"command"`
	}
	if json.Unmarshal(in.ToolInput, &ti) != nil || !gitCommitRe.MatchString(commandForDetection(ti.Command)) {
		return 0
	}
	if strings.Contains(ti.Command, "CLAUDIAO_SKIP=1") {
		_ = stats.Log("commit.skipped.manual", in.SessionID, in.CWD, nil)
		return 0
	}

	if trailerRe.MatchString(ti.Command) {
		_ = stats.Log("commit.blocked.trailer", in.SessionID, in.CWD, nil)
		fmt.Fprintln(stderr, "[claudiao] Commit blocked: the commit message contains an AI attribution trailer "+
			"(Co-Authored-By / Generated with Claude). The git rule forbids these. Remove the trailer and retry.")
		return 2
	}

	// The commit may target a different repo than the Bash cwd via the
	// git-global options `-C` / `--git-dir` / `--work-tree` (cumulative,
	// valid only BEFORE the subcommand). Let git itself resolve the target
	// work tree — hand-rolled path arithmetic got the cumulative and
	// separated-dir cases wrong. If globals are present but the work tree
	// cannot be resolved, fail CLOSED: an unresolvable target is exactly
	// where a bypass would hide.
	repoDir, ok := resolveRepoDir(in.CWD, ti.Command)
	if !ok {
		_ = stats.Log("commit.blocked.unresolved", in.SessionID, in.CWD, nil)
		fmt.Fprintln(stderr, "[claudiao] Commit blocked: this command uses git-global options "+
			"(-C / --git-dir / --work-tree) and claudiao could not resolve the target work tree to gate it. "+
			"Run the commit from the repository directory, or override with CLAUDIAO_SKIP=1 (user-approved only).")
		return 2
	}

	diff, isRepo, err := workingDiff(repoDir)
	if err != nil || !isRepo {
		return 0 // not a git repo: nothing to enforce
	}

	// Antipattern checks (rule-embedded + project-local) are regex over the
	// diff — cheap enough to run on every commit. Only blocker-level findings
	// on production code hard-block; major/minor (incl. broad immune
	// antibodies) stay advisory so a broad match never forces CLAUDIAO_SKIP=1.
	if serious := gateBlockingFindings(in.SessionID, repoDir, diff); len(serious) > 0 {
		_ = stats.Log("commit.blocked.check", in.SessionID, repoDir,
			map[string]string{"findings": fmt.Sprint(len(serious))})
		fmt.Fprintln(stderr, "[claudiao] Commit blocked: blocker-level antipattern(s) in the diff:")
		for _, f := range serious {
			fmt.Fprintf(stderr, "  [%s] %s — %s\n      %s\n", f.Check.Severity, f.Check.ID, f.File,
				truncateLine(f.Line, 160))
		}
		fmt.Fprintln(stderr, "Fix these, or override with CLAUDIAO_SKIP=1 (user-approved only).")
		return 2
	}

	areas := sensitive.MatchDiff(diff)
	for _, p := range untrackedFiles(repoDir) {
		areas = mergeAreas(areas, sensitive.MatchPath(p))
	}
	if len(areas) == 0 {
		_ = stats.Log("commit.ok", in.SessionID, repoDir, nil)
		return 0
	}

	r, err := receipt.Verify(repoDir)
	switch {
	case err == nil:
		_ = stats.Log("receipt.verified", in.SessionID, repoDir,
			map[string]string{"areas": sensitive.Join(areas), "reviewer": r.Reviewer,
				"reviewer_ran_in_session": fmt.Sprint(reviewerRanInSession(in.TranscriptPath))})
		return 0
	case errors.Is(err, receipt.ErrNoReceipt):
		_ = stats.Log("commit.blocked.receipt", in.SessionID, repoDir,
			map[string]string{"areas": sensitive.Join(areas), "cause": "missing"})
		fmt.Fprintf(stderr, "[claudiao] Commit blocked: this diff touches sensitive areas (%s) and no review receipt exists.\n"+
			"Run the adversarial reviewer first: Agent(subagent_type: \"sdd-reviewer\") — it records the receipt itself\n"+
			"after a clean pass. Resolve Blockers/Majors, then verify with: claudiao receipt verify\n"+
			"(Manual override, only with explicit user approval: prefix the commit with CLAUDIAO_SKIP=1.)\n",
			sensitive.Join(areas))
		return 2
	case errors.Is(err, receipt.ErrStale):
		_ = stats.Log("commit.blocked.receipt", in.SessionID, repoDir,
			map[string]string{"areas": sensitive.Join(areas), "cause": "stale"})
		fmt.Fprintln(stderr, "[claudiao] Commit blocked: the working tree changed after the last review — the receipt is stale.\n"+
			"Re-run sdd-reviewer on the current diff; it records a fresh receipt after a clean pass.")
		return 2
	default:
		// Sensitive areas were already confirmed; a receipt-verification
		// infrastructure error here must NOT silently allow the commit.
		_ = stats.Log("commit.blocked.receipt", in.SessionID, repoDir,
			map[string]string{"areas": sensitive.Join(areas), "cause": "verify-error"})
		fmt.Fprintf(stderr, "[claudiao] Commit blocked: this diff touches sensitive areas (%s) and the review "+
			"receipt could not be verified (%v). Record a receipt with claudiao receipt create, "+
			"or override with CLAUDIAO_SKIP=1 (user-approved only).\n", sensitive.Join(areas), err)
		return 2
	}
}

func postEdit(in Input, stderr io.Writer) int {
	var ti struct {
		FilePath  string `json:"file_path"`
		Content   string `json:"content"`
		NewString string `json:"new_string"`
		Edits     []struct {
			NewString string `json:"new_string"`
		} `json:"edits"`
	}
	if json.Unmarshal(in.ToolInput, &ti) != nil || ti.FilePath == "" {
		return 0
	}
	added := ti.Content + ti.NewString
	for _, e := range ti.Edits {
		added += e.NewString
	}

	reminders, _ := tiering.RecordEdit(in.SessionID, ti.FilePath, added)
	if len(reminders) == 0 {
		return 0
	}
	for _, r := range reminders {
		_ = stats.Log("tier.reminder", in.SessionID, in.CWD,
			map[string]string{"reminder": stats.Sanitize(r, 200)})
		fmt.Fprintln(stderr, r)
	}
	return 2 // PostToolUse: exit 2 surfaces stderr to Claude without undoing the edit
}

func stop(in Input, stdout io.Writer) int {
	if in.StopHookActive {
		return 0 // we already blocked once this turn; never loop
	}
	f, err := os.Open(in.TranscriptPath)
	if err != nil {
		return 0
	}
	defer f.Close()

	res := claims.Analyze(f)
	_ = stats.Log("stop.checked", in.SessionID, in.CWD, map[string]string{
		"reviewer_ran": fmt.Sprint(res.Evidence.ReviewerRan),
		"tests_ran":    fmt.Sprint(res.Evidence.TestsRan),
	})
	if !res.Block {
		return 0
	}
	_ = stats.Log("stop.blocked.claim", in.SessionID, in.CWD,
		map[string]string{"claims": stats.Sanitize(strings.Join(res.Claims, "; "), 300)})
	out, err := json.Marshal(map[string]string{"decision": "block", "reason": res.Reason})
	if err != nil {
		return 0
	}
	fmt.Fprintln(stdout, string(out))
	return 0
}

// workingDiff returns the diff to gate, whether dir is a git repo, and any
// error. A repo with no commits yet has no HEAD, so `git diff HEAD` fails;
// that must NOT fail open (the initial commit is the easiest place to slip
// sensitive code in), so we fall back to the staged+unstaged diff against
// the empty tree.
func workingDiff(dir string) (diff string, isRepo bool, err error) {
	if _, e := runGit(dir, "rev-parse", "--git-dir"); e != nil {
		return "", false, nil // genuinely not a repo
	}
	// `--` forces HEAD to parse as a revision (a worktree file named HEAD
	// otherwise makes git warn and exit 0, masking the diff).
	if out, e := runGit(dir, "diff", "HEAD", "--"); e == nil {
		return out, true, nil
	}
	// No HEAD: compare the whole index+worktree to the empty tree.
	const emptyTree = "4b825dc642cb6eb9a060e54bf8d69288fbee4904"
	out, e := runGit(dir, "diff", emptyTree, "--")
	if e != nil {
		return "", true, e
	}
	return out, true, nil
}

// sessionStart runs the immune detector (non-invasively — writes nothing) and,
// if the repo has recurring antipatterns not yet immunized, injects a short
// nudge as session context. Always exits 0; opening a session is never blocked.
func sessionStart(in Input, stdout io.Writer) int {
	if in.CWD == "" {
		return 0
	}
	fresh, err := immune.DetectNew(in.CWD, 300, 2)
	if err != nil || len(fresh) == 0 {
		return 0 // not a repo / git error / nothing new → stay silent
	}
	var b strings.Builder
	fmt.Fprintf(&b, "[claudiao immune] %d recurring antipattern(s) this repo already fixed are not yet immunized:\n", len(fresh))
	for i, s := range fresh {
		if i >= 5 {
			fmt.Fprintf(&b, "  …and %d more\n", len(fresh)-5)
			break
		}
		fmt.Fprintf(&b, "  - %s (fixed %dx before)\n", s.Signature, s.Hits)
	}
	b.WriteString("Run `claudiao immune scan --apply` to turn these into permanent checks.")

	out, err := json.Marshal(map[string]any{
		"hookSpecificOutput": map[string]string{
			"hookEventName":     "SessionStart",
			"additionalContext": b.String(),
		},
	})
	if err != nil {
		return 0
	}
	fmt.Fprintln(stdout, string(out))
	return 0
}

// gateBlockingFindings loads the rule-embedded checks (global ~/.claude/rules
// plus the project-local .claude/rules) and returns only the findings that
// should HARD-BLOCK a commit: severity blocker (curated, high precision) on a
// production code file. major/minor (including broad immune antibodies) stay
// advisory — surfaced by `claudiao check`/CI, not blocked here — so an
// intentionally-broad or doc/test-only match never forces CLAUDIAO_SKIP=1
// (which would also bypass the trailer and receipt gates).
func gateBlockingFindings(sessionID, repoDir, diff string) []checks.Finding {
	var cs []checks.Check
	for _, dir := range checkRuleDirs(repoDir) {
		if info, err := os.Stat(dir); err != nil || !info.IsDir() {
			continue // no rules directory there — not an error
		}
		loaded, err := checks.LoadDir(dir)
		if err != nil {
			// A malformed rule in ONE dir must not discard the others — a
			// project-local bad rule must never nuke the global security checks.
			_ = stats.Log("commit.check.ruleload.error", sessionID, dir,
				map[string]string{"err": stats.Sanitize(err.Error(), 200)})
			continue
		}
		cs = append(cs, loaded...)
	}
	var blocking []checks.Finding
	for _, f := range checks.RunOnDiff(cs, diff) {
		if f.Check.Severity == checks.SevBlocker && !isDocOrTest(f.File) {
			blocking = append(blocking, f)
		}
	}
	return blocking
}

// isDocOrTest excludes documentation and test files from the commit gate's
// antipattern block: those legitimately contain antipattern strings (fixtures,
// docs about the very antipattern). They stay covered by advisory `claudiao
// check`, just not hard-blocked at commit time.
func isDocOrTest(file string) bool {
	switch filepath.Ext(file) {
	case ".md", ".markdown", ".txt", ".rst", ".adoc":
		return true
	}
	base := filepath.Base(file)
	if strings.HasPrefix(base, "test_") || strings.HasSuffix(base, "_test.go") ||
		strings.Contains(base, ".test.") || strings.Contains(base, ".spec.") {
		return true
	}
	for _, part := range strings.Split(filepath.ToSlash(file), "/") {
		switch part {
		case "test", "tests", "__tests__", "testdata", "spec", "fixtures":
			return true
		}
	}
	return false
}

func checkRuleDirs(repoDir string) []string {
	var dirs []string
	if home, err := os.UserHomeDir(); err == nil {
		dirs = append(dirs, filepath.Join(home, ".claude", "rules"))
	}
	if repoDir != "" {
		dirs = append(dirs, filepath.Join(repoDir, ".claude", "rules"))
	}
	return dirs
}

func truncateLine(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) > n {
		return s[:n] + "…"
	}
	return s
}

func runGit(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}

var (
	gitWordRe    = regexp.MustCompile(`\bgit\b`)
	commitWordRe = regexp.MustCompile(`\bcommit\b`)
)

// commandPrefix returns the part of the command between `git` and the
// `commit` subcommand, where git-global options live. The `commit -C
// <commit>` reuse-message flag lives AFTER `commit`, so it is excluded.
func commandPrefix(command string) string {
	gm := gitWordRe.FindStringIndex(command)
	if gm == nil {
		return ""
	}
	rest := command[gm[1]:]
	cm := commitWordRe.FindStringIndex(rest)
	if cm == nil {
		return ""
	}
	return rest[:cm[0]]
}

// resolveRepoDir returns the work tree the commit will actually write to.
// When the command carries git-global directory options, git resolves them
// (cumulative -C, --git-dir, --work-tree) far more reliably than manual path
// joins. The second return is false only when globals are present but
// unresolvable — the caller fails closed in that case.
func resolveRepoDir(cwd, command string) (string, bool) {
	globals := globalDirFlags(commandPrefix(command))
	if len(globals) == 0 {
		return cwd, true
	}
	args := append(append([]string{}, globals...), "rev-parse", "--show-toplevel")
	out, err := runGit(cwd, args...)
	if err != nil {
		return "", false
	}
	top := strings.TrimSpace(out)
	if top == "" {
		return "", false
	}
	return top, true
}

// globalDirFlags extracts the -C / --git-dir / --work-tree tokens (and their
// values) from the pre-subcommand prefix. Fields-based tokenization means
// shell metacharacters never reach a subshell — the tokens are passed to git
// as literal argv, and a malformed flag simply makes `git rev-parse` fail.
func globalDirFlags(prefix string) []string {
	fields := strings.Fields(prefix)
	var out []string
	for i := 0; i < len(fields); i++ {
		f := fields[i]
		switch {
		case f == "-C" || f == "--git-dir" || f == "--work-tree":
			out = append(out, f)
			if i+1 < len(fields) {
				out = append(out, fields[i+1])
				i++
			}
		case strings.HasPrefix(f, "--git-dir=") || strings.HasPrefix(f, "--work-tree="):
			out = append(out, f)
		}
	}
	return out
}

// untrackedFiles lists files git would not show in a diff (never staged), so
// the gate can still classify sensitive paths in an initial/path-scoped commit.
func untrackedFiles(dir string) []string {
	out, err := runGit(dir, "ls-files", "--others", "--exclude-standard")
	if err != nil {
		return nil
	}
	var files []string
	for _, l := range strings.Split(out, "\n") {
		if l = strings.TrimSpace(l); l != "" {
			files = append(files, l)
		}
	}
	return files
}

func mergeAreas(a, b []sensitive.Area) []sensitive.Area {
	seen := map[sensitive.Area]bool{}
	for _, x := range a {
		seen[x] = true
	}
	for _, x := range b {
		if !seen[x] {
			seen[x] = true
			a = append(a, x)
		}
	}
	return a
}

// reviewerRanInSession reports whether sdd-reviewer appears in the session
// transcript. The receipt cannot be cryptographic proof of a review (the
// agent can run `claudiao receipt create` itself), so this is recorded
// alongside every verified receipt to make a forged one detectable in stats.
func reviewerRanInSession(transcriptPath string) bool {
	if transcriptPath == "" {
		return false
	}
	f, err := os.Open(transcriptPath)
	if err != nil {
		return false
	}
	defer f.Close()
	return claims.Analyze(f).Evidence.ReviewerRan
}
