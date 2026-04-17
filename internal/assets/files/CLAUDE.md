# Project — Spec Driven Development (SDD)

## Required flow

This project uses **Spec Driven Development**. Every feature goes through:

**Discover → Design → Task → Implement → Review → Ship**

Specs live under `specs/<feature_slug>/` with numbered files (00 to 06).

## Match effort to size — read this FIRST

Not every change is a feature. Before anything else, classify the tier
(Trivial / Small / Medium / Large) per `rules/effort-tiering.md` and
follow the lightweight flow for smaller tiers.

Full ceremony (Discover → Design → Tasks → …) only applies to **Medium**
and **Large**. Trivial goes straight to commit. Small collapses into a
one-page spec.

## Core rule (Medium+)

Do NOT write code without filled-out specs:

1. `01-discover.md` with MVP, acceptance criteria, and edge cases
2. `02-design.md` with architecture, contracts, and NFRs
3. `03-tasks.md` with small, verifiable tasks

If the user asks for direct implementation on a Medium+ change, complete
the earlier phases first. Trivial/Small have their own lighter gates.

## Specs are memory

Each spec is the **handoff document** for the next agent. Every phase runs in
isolated context — the previous spec is the ONLY bridge. Therefore:

- Specs must be **complete and self-contained** (the reader never needs to ask more)
- Record **decisions AND the reasoning** behind them
- Record **alternatives discarded** and why
- Use **concrete examples** with realistic data
- Never leave "…" in a finished spec
- End every spec with a **Handoff** summarizing what the next phase needs

## Agents ask — via AskUserQuestion

Before writing any spec, the agent INTERVIEWS the user in rounds — **scaled
to the tier**:

- **Trivial / Small**: 0-1 round. Ask only if something is genuinely unknown.
- **Medium / Large**: 2-4 rounds (domain / scope / edge cases / confirmation).

Stop asking when the answers become "obvious from context" — that is the
signal the tier is too heavy, not that you must push through.

### Interaction rule: ALWAYS use AskUserQuestion

**EVERY** question to the user MUST use the `AskUserQuestion` tool.
**NEVER** ask as plain text, numbered lists, or bullets.

API constraints:
- 1-4 questions per call (maxItems: 4)
- 2-4 options per question (minItems: 2, maxItems: 4)
- **Do NOT include an "Other" option** — the system adds one automatically
- `header` is required, max 12 chars
- `description` is required on every option
- `multiSelect` is required (boolean)

If `AskUserQuestion` is not present in the runtime, emit
`[PENDING_USER_QUESTIONS]` (see `rules/clarifications.md`) and stop.

## Built-in Claude Code tools

Beyond AskUserQuestion, use these native tools throughout SDD:

- **TodoWrite**: visible task progress during implementation
- **Grep**: code search — NEVER run `grep`/`rg` via Bash, use the native tool
- **Task**: launch sub-agents (Explore / general-purpose) for deeper codebase analysis

## Act, don't analyze forever

Reading without producing output is not progress. Limits:

- **Every Grep/Read/Task(Explore) call must produce a written takeaway** in the next response. If you cannot state what you learned, the call was wasted.
- **More than 3 Read/Grep calls in a row without a code change, spec paragraph, or user question** → stop and report "stuck on X, need Y".
- **If the same question bounces back and forth in your head for 2+ tool calls without a concrete next step** → surface it to the user.

Analysis paralysis costs the user tokens, time, and trust. Default to the
smallest concrete next step, then iterate.

## Transitions are explicit

The user decides when to move forward. No agent auto-advances to the next phase.
At the end of each phase the agent:

- Declares **Ready: yes/no** with a reason (see `rules/quality-gates.md` for the evidence contract)
- Summarizes **what was decided** and **what remains open**
- Suggests the next command, but **asks** whether the user wants to advance

## Commands

- `/sdd-new <description>` — start a feature (creates specs + suggests discover)
- `/sdd <feature_slug>` — dashboard of state + next step
- `/sdd-discover <slug>` — Discover (fork, deep interview)
- `/sdd-design <slug>` — Design (fork, technical decisions)
- `/sdd-task <slug>` — Tasks (fork, decomposition)
- `/sdd-implement <slug>` — Implement (fork, execution)
- `/sdd-review <slug>` — Review (fork, analysis)
- `/sdd-ship <slug>` — Ship (fork, release)

## Rules — always loaded

@.claude/rules/effort-tiering.md
@.claude/rules/concision.md
@.claude/rules/clarifications.md
@.claude/rules/quality-gates.md
@.claude/rules/git.md

## Rules — load when the surface is touched

Do NOT pre-load these. They contain significant context and should only
be Read when a change actually touches the area.

- `@.claude/rules/testing.md` — writing or changing tests; always for Medium+
- `@.claude/rules/code-quality.md` — non-trivial code changes; always for Medium+
- `@.claude/rules/security.md` — auth, secrets, input validation, external calls, data access, crypto, sessions
- `@.claude/rules/performance.md` — hot paths, DB queries, caching, frontend perf budgets, background jobs
- `@.claude/rules/ui-ux.md` — any user-visible surface (web, mobile, CLI UX)

When you decide a rule applies, `Read` the file at the start of the phase
it applies to — not at the end.
