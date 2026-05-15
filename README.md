# claudiao

A TUI installer that drops a **compact AI coding agreement** into your
[Claude Code](https://claude.com/claude-code) config. Three habits — ask
only when ambiguous, self-critique before declaring done, adversarial
reviewer auto-fires on sensitive areas — turned into files Claude Code
reads on every conversation.

![status](https://img.shields.io/badge/status-alpha-orange) ![license](https://img.shields.io/badge/license-MIT-blue) ![go](https://img.shields.io/badge/go-1.24+-00ADD8)

---

## Why this exists

AI-assisted coding is everywhere. Most of it is awful.

- **Vibe coders** ship features they do not understand, wiring three libraries
  together via copy-paste and praying the tests pass.
- **Security is an afterthought.** Credentials in repos, SQL injection back from
  the dead, auth middlewares that trust the client.
- **No process.** Every prompt is a blank slate; context evaporates between
  sessions; agents invent APIs and lie about it.
- **Tests are theatre.** Generated to satisfy coverage, not to catch bugs —
  mocks returning the exact value the assertion expects.
- **Self-approval is the default.** The model writes code, the model declares
  it "secure", and a real reviewer later finds three vulnerabilities.

The LLM is not the problem. The **absence of process and adversarial review** is.

claudiao is the process, turned into files your Claude Code instance reads on
every conversation: rules that forbid sloppy behavior, an adversarial reviewer
subagent that fires automatically on sensitive areas, quality gates that
refuse to declare safety without evidence.

## What it installs

Everything lands in `~/.claude/`:

| Module | Where | What |
|---|---|---|
| **Core** | `CLAUDE.md` | The compact flow + rules index |
| **Rules** | `rules/*.md` | 10 rule files. Pre-loaded every turn: `concision.md`, `clarifications.md`, `git.md`. On-demand (Read when the surface is touched): `testing.md`, `code-quality.md`, `security.md`, `performance.md`, `ui-ux.md`, `effort-tiering.md`, `quality-gates.md` |
| **Agent** | `agents/sdd-reviewer.md` | Adversarial reviewer subagent. Posture: *"wrong until proven otherwise"*. Auto-invoked when the diff touches a sensitive area |
| **Output Style** | `output-styles/sdd-orchestrator.md` | Orchestrator persona — compact flow, no phases, no spec files |

What it **never** touches:

- `~/.claude/memory/` — your personal memories
- `~/.claude/projects/` — conversation history
- `~/.claude/credentials.json`, `.credentials.json` — OAuth / API tokens
- `~/.claude/settings.local.json` — local overrides
- `~/.claude/todos/`, `shell-snapshots/`, `statsig/`, `history.jsonl`

A full `~/.claude` backup (`~/.claude.backup-<timestamp>`) is taken **before**
any file is written, every time.

## The flow

No phases, no numbered spec files, no Ready/Evidence ceremony.
Conversation + diff are the memory.

```
[Ask if ambiguous] → [Short plan] → [Implement] → [Self-critique] →
[Auto-reviewer if sensitive] → [Resolve findings] → [Commit]
```

Rules are strict:

- **Ask via `AskUserQuestion`** only when there is real ambiguity — 1-3 grouped questions, no mandatory rounds. Subagents that lack the tool emit a `[PENDING_USER_QUESTIONS]` block and stop; the orchestrator brings the question to the user.
- **Self-critique before declaring done.** List 3 ways the code could be wrong and verify each one. If you cannot list 3, you have not looked enough.
- **Adversarial reviewer is mandatory** when the diff touches a sensitive area: auth, crypto, input validation, DB queries, public endpoints, authz, secrets, schema migrations, new dependencies, paths from user input (traversal), URLs from user input (SSRF), templates rendering user content (XSS). The orchestrator calls `sdd-reviewer` automatically — it does not ask the user whether to review. Blockers and Majors are fixed before commit.
- **Never auto-declare "secure / safe / protected"** without concrete evidence (reviewer ran, security lint ran, antipattern grep ran). Otherwise: *"implemented; security not verified."*
- **No numbered spec files.** `01-discover.md`, `02-design.md`, etc. are forbidden — conversation + diff carry the context.
- **Testing**: no mocks of internal code, no tests written to pass, mutation-mental check on every function, happy-path + error + edge case required, coverage floor 80% unit.
- **Security**: secrets never in repo/logs/URLs; parameterized queries; allow-list authz; SSRF guards; JWT `alg:none` rejected; CSRF on cookie-auth mutations; constant-time compares.
- **Performance**: latency/memory budgets declared per feature; no N+1, no `SELECT *`, no offset pagination on large tables; timeouts on every external call; bounded concurrency; regression guards in CI.
- **Code quality**: strong typing (no `any`/`interface{}`), SOLID, no god objects, cyclomatic ≤ 10, files ≤ 1000 lines, illegal states unrepresentable.
- Commits never carry AI-attribution trailers.

## Install

### From source (current)

```bash
git clone https://github.com/Muriel-Gasparini/claudiao
cd claudiao
go build -o claudiao ./cmd/claudiao
./claudiao
```

Requires Go 1.24+.

### From releases

Pre-built binaries for Linux, macOS (Intel + Apple Silicon), and Windows live at
[Releases](https://github.com/Muriel-Gasparini/claudiao/releases).

Linux / macOS one-liner (replace the version with the latest tag):

```bash
VERSION=v0.2.0
OS=$(uname | tr '[:upper:]' '[:lower:]')                # linux | darwin → macos
ARCH=$(uname -m | sed 's/x86_64/x86_64/;s/aarch64/arm64/')
[ "$OS" = "darwin" ] && OS=macos

curl -L -o /tmp/claudiao.tar.gz \
  "https://github.com/Muriel-Gasparini/claudiao/releases/download/${VERSION}/claudiao_${VERSION#v}_${OS}_${ARCH}.tar.gz"
tar -xzf /tmp/claudiao.tar.gz -C /tmp
sudo mv /tmp/claudiao /usr/local/bin/claudiao
claudiao version
```

## Usage

Run the binary. You get a six-screen TUI:

```
Welcome        → detect ~/.claude, confirm
Select modules → pick which groups to install (Core, Rules, Agents, Styles)
Install mode   → Copy (default, safe) or Symlink (live, for contributors)
Preview        → every file with status: + create · = same · ~ differ
Installing     → spinner while writing, backup kept
Result         → ✓ done · N files installed · backup path
```

Keys: `↑/↓` navigate · `space` toggle · `a` toggle all · `enter` next · `b` back · `q` quit.

Re-running is safe: unchanged files are skipped, modified ones are previewed
with the new version before overwriting. Your backups pile up under `~/.claude.backup-*`.

## Install modes

- **Copy** — files are written into `~/.claude/`. Edit them freely; future
  claudiao runs will flag your edits as `~ differ` before touching them.
- **Symlink** — files become symlinks back to the repo. Pulling upstream
  changes applies them instantly. Intended for contributors. Requires
  `CLAUDIAO_ASSETS_DIR=/path/to/repo/internal/assets/files` or running
  `claudiao` from alongside the checked-out tree.

## Contributing

The framework lives entirely under `internal/assets/files/`:

- `CLAUDE.md` — top-level flow
- `rules/*.md` — behavior rules
- `agents/sdd-reviewer.md` — adversarial reviewer subagent
- `output-styles/sdd-orchestrator.md` — orchestrator persona

Edit the markdown, rebuild (`go build ./cmd/claudiao`), and the new content
ships with the binary (everything is embedded via `go:embed`).

PRs welcome — especially new rules, tighter quality gates, and improvements
to agent interview protocols.

### Dev workflow

```bash
go test ./...                                           # unit tests
go test ./... -coverprofile=coverage.out                # with coverage
go tool cover -func=coverage.out                        # coverage report
go run ./cmd/claudiao                                   # run the TUI
```

Coverage floor is 80%. Tests must exercise real behavior; mocks of internal
code are rejected in review.

## Safety notes

- claudiao never executes anything from the installed files. It is a file copier.
- Symlink mode writes absolute paths. Do not delete the source tree while
  symlinks exist — Claude Code will see dangling links.
- The installer refuses to copy files whose top-level path matches a protected
  name (`memory`, `projects`, `credentials.json`, `settings.local.json`, etc.)
  even if you ask it to.

## License

MIT — see [LICENSE](./LICENSE).
