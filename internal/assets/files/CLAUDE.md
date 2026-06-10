# Working agreement (compact)

ALWAYS READ @~/.claude/RTK.md

This file replaces the old phase-based SDD. There are no spec files, no
six-phase workflow, no Ready/Evidence ceremony. The system is built around
three habits: **ask only when ambiguous**, **review adversarially when it
matters**, **never auto-declare safety without evidence**.

## Default flow

1. **Ask only if there is real ambiguity.** If the request is clear, go.
   When you ask, use `AskUserQuestion` (1-3 grouped questions in a single
   call). No mandatory rounds.
2. **Short plan in the conversation** (`TodoWrite` or Plan tool) before
   touching more than one file or more than ~50 lines.
3. **Implement.**
4. **Self-critique before declaring done.** List 3 ways the code could be
   wrong and verify each one. If you cannot list 3, you have not looked
   enough.
5. **Adversarial reviewer is mandatory** when the diff touches a sensitive
   area (list below). You call `Agent({ subagent_type: "sdd-reviewer", … })`
   yourself — do not ask the user whether to review. Pass the scope in the
   prompt (changed-file list + sensitive areas + acceptance criteria), not
   the full diff. Resolve every Blocker and Major before commit.
6. Commit with a Conventional Commit message. No AI trailers (see
   `rules/git.md`).

## Sensitive areas — auto-trigger the reviewer

- Authentication, sessions, cookies, JWT
- Cryptography, hashing, RNG, secret comparison
- Input validation, sanitization, parsers of untrusted bytes
- Database queries (SQL/NoSQL); filters built from user input
- Public endpoints (HTTP, RPC, WebSocket)
- Authorization / permissions / multi-tenant isolation
- Secrets, environment variables, `.env` files
- Schema migrations
- Adding or upgrading external dependencies
- File paths built from user input (path traversal)
- Fetching URLs supplied by the user (SSRF)
- Templates rendering user content (XSS, SSTI)

If the diff touches any of these, the reviewer runs. Period.

## Mechanized enforcement (when the Hooks module is installed)

The `claudiao` binary enforces parts of this agreement via hooks — they
are not suggestions, they will block:

- **Commit gate** (`PreToolUse` on Bash): commits with AI attribution
  trailers are rejected; commits whose diff touches a sensitive area
  require a valid review receipt (`claudiao receipt create`, recorded by
  sdd-reviewer after the review). If the code changed after the review,
  the receipt is stale — re-review.
- **Auto effort-tiering** (`PostToolUse` on Edit/Write): crossing >50
  lines / >2 files or touching a sensitive path injects the
  corresponding obligation once. Treat the injected reminder as part of
  this agreement.
- **Claim checker** (`Stop`): a final message claiming safety without
  the reviewer having run, or claiming tests pass without a test command
  having run, blocks the stop. Produce the evidence or retract the claim.

Useful commands: `claudiao check` (rule-embedded antipattern checks on
the diff), `claudiao mutate` (proves the tests around the diff can
fail), `claudiao stats` (adherence report). Escape hatch, user-approved
only: `CLAUDIAO_ENFORCE=off` or `CLAUDIAO_SKIP=1` prefix on the commit.

## Comments policy (code)

Default: **no comments**. The order of preference for clarity is:

1. clear code → 2. good names → 3. small functions → 4. comment **only**
where context is missing.

**Comment when** the reader genuinely cannot infer the *why*:

- a weird business rule the code alone does not explain
- a workaround for a lib/API/browser bug (link the issue + affected version)
- a decision that looks wrong but is intentional
- a "trap" someone would simplify and silently break
- code touching security, money, concurrency, cache, idempotency, timezone,
  or fragile parsing

**Do not comment** when:

- it just restates the function/variable name (`// increments counter`)
- it explains basic language syntax
- it compensates for confusing code that should be **refactored** instead
- it rots fast: references to current task/PR/caller, "for now", "added in
  Sprint 12", "used by X" — that belongs in the PR description, not the code

If a comment would only repeat what a well-named identifier already says,
**delete it and improve the name instead.**

## Hard prohibitions

- Never claim "secure / safe / protected / hardened" without concrete
  evidence (reviewer ran, security linter ran, or a documented
  antipattern grep ran). Otherwise say: *"implemented; security not
  verified"*.
- Never create numbered spec files (`01-discover.md`, `02-design.md`,
  `03-tasks.md`, `04-implementation.md`, `05-review.md`, `06-ship.md`).
  Conversation + diff are the memory.
- Never use `Co-Authored-By:` or other AI attribution trailers.
- Never declare "done" while a `[PENDING_USER_QUESTIONS]` block from a
  subagent is unanswered — surface it to the user first.

## Built-in tools used here

- `AskUserQuestion` for clarification (1-4 questions, 2-4 options, no "Other").
- `TodoWrite` to show the short plan and progress.
- `Grep` for code search (never `grep`/`rg` via Bash).
- `Agent` (subagent_type: `sdd-reviewer`) for the mandatory adversarial review.

## Rules — pre-loaded

@.claude/rules/concision.md
@.claude/rules/clarifications.md
@.claude/rules/git.md

## Rules — load on demand (Read when the surface is touched)

DO NOT prefix these with `@`. The `@` prefix auto-loads the file. Use
`Read` on the path below at the start of the work that touches the area.

- rules/testing.md — writing or changing tests
- rules/code-quality.md — non-trivial code changes
- rules/security.md — required reading before invoking the reviewer
- rules/performance.md — hot paths, queries, caching, frontend perf
- rules/ui-ux.md — any user-visible surface (web, mobile, CLI UX)
- rules/effort-tiering.md — when the size of the change is unclear
- rules/quality-gates.md — reviewer protocol and finding format
