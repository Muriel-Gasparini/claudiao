# claudiao

A TUI installer that drops a **Spec Driven Development** framework into your
[Claude Code](https://claude.com/claude-code) config. Opinionated rules,
interviewer sub-agents, quality gates — everything needed so an LLM doesn't
ship you a house of cards.

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

The LLM is not the problem. The **absence of a process** is.

claudiao is the process, turned into files your Claude Code instance reads on
every conversation: rules that forbid sloppy behavior, sub-agents that force
interviews before implementation, quality gates that refuse to ship untested
code.

## What it installs

Everything lands in `~/.claude/`:

| Module | Where | What |
|---|---|---|
| **Core** | `CLAUDE.md` | Global SDD flow (Discover → Design → Tasks → Implement → Review → Ship) and the rules index |
| **Rules** | `rules/*.md` | 8 enforceable rule files: `testing.md`, `security.md`, `performance.md`, `code-quality.md`, `clarifications.md`, `quality-gates.md`, `git.md`, `concision.md` |
| **Agents** | `agents/sdd-*.md` | Sub-agents per phase: product-owner, architect, dev-lead, implementer, reviewer, release-manager |
| **Output Styles** | `output-styles/sdd-*.md` | Personas the orchestrator wears during each phase |

What it **never** touches:

- `~/.claude/memory/` — your personal memories
- `~/.claude/projects/` — conversation history
- `~/.claude/credentials.json`, `.credentials.json` — OAuth / API tokens
- `~/.claude/settings.local.json` — local overrides
- `~/.claude/todos/`, `shell-snapshots/`, `statsig/`, `history.jsonl`

A full `~/.claude` backup (`~/.claude.backup-<timestamp>`) is taken **before**
any file is written, every time.

## The SDD flow

Every feature goes through six phases. Each phase leaves a spec file under
`specs/<feature_slug>/`. The next phase reads that spec in a fresh context —
that is its **only** handoff.

```
01-discover.md  → 02-design.md  → 03-tasks.md  → 04-implementation.md
                                                  ↓
                                  05-review.md  → 06-ship.md
```

Rules are strict:

- **No implementation** without a filled `01-discover.md`, `02-design.md`, `03-tasks.md`.
- Every interview agent asks via `AskUserQuestion` before writing. If the tool is missing in the runtime, the subagent emits a `[PENDING_USER_QUESTIONS]` block and **stops**; the orchestrator asks the user and re-invokes it. No silent Assumptions.
- **Ready is evidence, not opinion.** The orchestrator runs the phase rubric adversarially, emits a `Blocker / Major / Minor` findings table, and only then declares Ready with an explicit `Evidence` block. "Looks good to me" is rejected at the gate.
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
VERSION=v0.1.2
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
- `agents/sdd-*.md` — sub-agent definitions
- `output-styles/sdd-*.md` — orchestrator personas

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
