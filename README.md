# claudiao

A working agreement for AI-assisted coding — **and a binary that actually
enforces it** — for [Claude Code](https://claude.com/claude-code).

Most "AI coding rules" are just text the model reads and may quietly ignore.
claudiao ships the same text **plus** a small Go binary that checks the rules
were followed. The motto:

> **Rules say. The binary checks.**

![status](https://img.shields.io/badge/status-alpha-orange) ![license](https://img.shields.io/badge/license-MIT-blue) ![go](https://img.shields.io/badge/go-1.24+-00ADD8)

---

## Why this exists

AI writes a lot of code now. Most of it is reviewed by no one.

- The model writes code, **calls it "secure" itself**, and a real reviewer later finds three holes.
- **Tests are theatre** — generated to make coverage green, not to catch bugs.
- **Secrets, SQL injection, auth that trusts the client** sneak in because nothing checks.
- Every session is a blank slate; the same mistakes come back.

The model isn't the problem. The **lack of process and of an independent check** is. claudiao is that check — turned into files Claude Code reads, backed by a binary that won't take the model's word for it.

## How it works, in plain words

Think of it as **a contract + an inspector**.

- **The contract** is plain Markdown your Claude Code reads every conversation: a short workflow, behavior rules, and an adversarial reviewer it must call on risky changes.
- **The inspector** is the `claudiao` binary, wired in as Claude Code *hooks*. When the model tries to commit, finish, or save, the inspector runs and **blocks** if the contract wasn't honored.

Here are the inspector's tricks — each one replaces something the model could otherwise just *claim*:

### 🧾 Review receipt — "prove the review happened"

When the adversarial reviewer finishes, it stamps a **receipt** tied to the exact code it saw (a fingerprint of the diff). At commit time, the gate checks the receipt. **Change one line after the review and the stamp breaks** — you have to re-review. So "I reviewed it" stops being a claim and becomes a fact the binary can verify. Like a tamper-evident seal on the code.

### 🤥 Claim checker — "don't say it, show it"

Before the model ends its turn, the inspector reads the final message. If it says *"it's secure"* but **no reviewer ran**, or *"tests pass"* but **no test command ran**, the turn is **blocked** until the evidence exists or the claim is dropped. The proof is read from what actually executed — writing the words "go test" in a sentence doesn't count.

### 🎭 Test-theatre detector — "break the code on purpose"

`claudiao mutate` doesn't read your tests; it **sabotages your code** — flips a `>` to `>=`, an `&&` to `||` — in a throwaway copy, then runs your suite. If **no test complains**, that test is decoration: it can't catch the bug you just planted. It's like testing a smoke alarm by lighting a match — if it doesn't go off, the alarm is fake. A surviving sabotage is concrete proof of a missing test.

### 🛡️ Immune system — "the repo learns from its own scars"

`claudiao immune` reads your project's history of `fix:` commits — every fix is you admitting something was wrong. It finds the mistakes that **keep coming back** (the same dangerous call removed in two or more fixes) and turns each into an **antibody**: a permanent check that blocks that exact mistake from returning. Like the body after an illness — once you've been bitten, you're immune. The clever part: the expensive thinking happens **once**, mined from history; the antibodies then run for free forever, so an **older repo gets safer and cheaper to guard, not slower**.

### 📏 Auto-tiering — "notice when it got serious"

As the model edits, the inspector watches. The moment a change crosses ~50 lines, touches a second file, or edits something sensitive (auth, crypto, DB…), it **injects the obligation** to write a plan and call the reviewer. The model can't "forget" the change got big — the binary noticed for it.

That's the whole idea: **every rule that can be mechanized has a matching check, so the model can't just say it's done — it has to be true.**

## What it installs

Pick what you want in a small TUI. Everything lands in `~/.claude/`:

| Module | What it is |
|---|---|
| **Core** | `CLAUDE.md` — the short workflow + index of rules |
| **Rules** | Behavior rules (testing, security, performance, code-quality, git…). A few load every turn; the rest are read on demand. |
| **Agents** | `sdd-reviewer` (the adversarial reviewer), `test-fortifier` (writes the tests that kill surviving sabotages), `immunologist` (writes a regression test for a recurring scar) |
| **Skills** | `/ship` — runs the whole ritual in one command (self-critique → checks → review+receipt → mutate → commit) |
| **Output Style** | The orchestrator persona — compact, no ceremony |
| **Hooks** | The enforcement runtime: wires the binary in as Claude Code hooks (this is what makes the checks above actually fire) |

It **never** touches your `memory/`, `projects/`, `credentials.json`, `settings.local.json`, etc. A full backup of `~/.claude` is taken **before any write**.

## The commands

The binary is also useful by hand:

```bash
claudiao            # the installer (TUI)
claudiao check      # run the antipattern checks against your current diff
claudiao mutate     # the test-theatre detector (add --json to feed test-fortifier)
claudiao immune     # learn antibodies from the repo's fix history (scan/verify/status)
claudiao init       # drop project-local checks for your stack (Go/TS/Python/Rust)
claudiao learn      # show where bugs cluster, from stats + fix history
claudiao stats      # is the agreement actually being followed? (blocks, receipts, claims)
claudiao receipt    # record/verify a review receipt
```

`claudiao check` automatically loads both the global rules and your project's
local `.claude/rules` (including the immune antibodies), so anything you teach
it shows up in the commit gate for free.

## The workflow

No phases, no numbered spec files. Conversation + diff are the memory.

```
ask only if unclear → short plan → implement → self-critique →
auto-review if risky → fix findings → commit
```

The rules behind it are strict and opinionated: ask via `AskUserQuestion` only on real ambiguity; list 3 ways the code could be wrong before declaring done; the adversarial reviewer is mandatory (and automatic) on sensitive areas; never claim "secure" without evidence; no mocks of your own code; commits carry no AI-attribution trailers. See `internal/assets/files/rules/` for the full set.

## Install

### From source

```bash
git clone https://github.com/Muriel-Gasparini/claudiao
cd claudiao
go build -o claudiao ./cmd/claudiao
./claudiao
```

Requires Go 1.24+. **Tip:** build/run it from a stable location (or just let the
Hooks module copy it to `~/.local/bin/claudiao`), because the hooks call the
binary by absolute path.

### From releases

Pre-built binaries for Linux, macOS (Intel + Apple Silicon), and Windows live at
[Releases](https://github.com/Muriel-Gasparini/claudiao/releases).

```bash
VERSION=v0.3.0
OS=$(uname | tr '[:upper:]' '[:lower:]'); [ "$OS" = "darwin" ] && OS=macos
ARCH=$(uname -m | sed 's/aarch64/arm64/')
curl -L -o /tmp/claudiao.tar.gz \
  "https://github.com/Muriel-Gasparini/claudiao/releases/download/${VERSION}/claudiao_${VERSION#v}_${OS}_${ARCH}.tar.gz"
tar -xzf /tmp/claudiao.tar.gz -C /tmp && sudo mv /tmp/claudiao /usr/local/bin/claudiao
claudiao version
```

**macOS:** the release binaries aren't notarized yet, so Gatekeeper blocks the
first run. Clear the quarantine attribute once: `xattr -d com.apple.quarantine
/usr/local/bin/claudiao` (or allow it under System Settings → Privacy &
Security). A binary you build yourself with `go build` is unaffected.

## Usage

Run the binary and pick modules in the TUI (`↑/↓` move · `space` toggle · `enter` next). Re-running is safe — unchanged files are skipped, changed ones previewed before overwrite. The merge into `settings.json` keeps any third-party hooks (e.g. `rtk`) intact.

- **Copy mode** (default) — files written into `~/.claude/`; edit them freely.
- **Symlink mode** — files link back to the repo, so pulling upstream applies instantly. For contributors.

When the Hooks module is on, the binary copies itself to a stable path so the hooks keep working from anywhere. Escape hatches, user-approved only: `CLAUDIAO_ENFORCE=off` (disable all hooks) or `CLAUDIAO_SKIP=1` prefixing a single commit (bypass the gate once).

## How the hooks behave (safety)

- They **fail open** on infrastructure errors (no git, broken input) so a bug here can never brick your session — with one deliberate exception: once a diff is confirmed sensitive, a receipt-verification *error* fails **closed**, because silently allowing it would be the bypass.
- Only **blocker**-level antipatterns on production code hard-block a commit; broad/advisory findings (and immune antibodies) surface in `claudiao check` but don't force `CLAUDIAO_SKIP=1`. Doc and test files are exempt from the gate.
- The self-installer writes the binary atomically and refuses a world-writable or someone-else's directory. `settings.json` is never clobbered — if it can't be parsed, the merge refuses.

## Architecture

A TUI + installer around small, single-purpose, well-tested packages — the enforcement logic is pure Go, the hook handlers are thin glue, and everything ships embedded via `go:embed` (no runtime deps beyond `git`).

| Package | Responsibility |
|---|---|
| `internal/tui` · `installer` · `settings` | TUI, install/backup/self-install, idempotent `settings.json` merge |
| `internal/hook` | Hook protocol + handlers (pre-bash, post-edit, stop, session-start) |
| `internal/sensitive` | Deterministic sensitive-area classifier (paths + diff) |
| `internal/receipt` · `claims` | Review receipts; transcript claim-checker |
| `internal/tiering` · `checks` | Effort-threshold reminders; `claudiao-check` parser/runner |
| `internal/mutate` | Mutation testing in an isolated worktree |
| `internal/immune` · `learn` · `projinit` | Antibodies + case-law ledger; fix-history mining; stack-local checks |
| `internal/stats` | Local JSONL telemetry of adherence |

## Contributing

The agreement lives under `internal/assets/files/`. Edit the Markdown, rebuild, and it ships embedded.

```bash
go test ./...   # full suite
go vet ./...
```

Tests must exercise real behavior; mocks of internal code are rejected in review. PRs welcome — new rules, tighter checks, new mutation operators.

## License

MIT — see [LICENSE](./LICENSE).
