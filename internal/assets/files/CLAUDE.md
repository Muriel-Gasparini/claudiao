# Project — Spec Driven Development (SDD)

## Required flow

This project uses **Spec Driven Development**. Every feature goes through:

**Discover → Design → Task → Implement → Review → Ship**

Specs live under `specs/<feature_slug>/` with numbered files (00 to 06).

## Core rule

Do NOT write code without filled-out specs:

1. `01-discover.md` with MVP, acceptance criteria, and edge cases
2. `02-design.md` with architecture, contracts, and NFRs
3. `03-tasks.md` with small, verifiable tasks

If the user asks for direct implementation, complete the earlier phases first.

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

Before writing any spec, the agent INTERVIEWS the user in rounds:

1. **Domain context** — how it works today, who uses it, what the pain is
2. **Scope and limits** — what is in, what is out, constraints
3. **Edge cases** — extreme scenarios, failures, concurrency
4. **Confirmation** — summarize what was understood, validate with the user

The agent writes the spec only after at least 2 rounds of questions.

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

## Built-in Claude Code tools

Beyond AskUserQuestion, use these native tools throughout SDD:

- **TodoWrite**: visible task progress during implementation (status: pending/in_progress/completed)
- **Grep**: code search — NEVER run `grep`/`rg` via Bash, use the native tool
- **Task**: launch sub-agents (Explore or general-purpose) for deeper codebase analysis

## Transitions are explicit

The user decides when to move forward. No agent auto-advances to the next phase.
At the end of each phase the agent:

- Declares **Ready: yes/no** with a reason
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

## Rules

@.claude/rules/clarifications.md
@.claude/rules/testing.md
@.claude/rules/git.md
@.claude/rules/quality-gates.md
@.claude/rules/concision.md
@.claude/rules/security.md
@.claude/rules/performance.md
@.claude/rules/code-quality.md
