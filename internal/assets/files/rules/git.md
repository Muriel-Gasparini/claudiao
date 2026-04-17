# Rules — Git: commit hygiene

## No AI attribution trailers

- **Forbidden** in commit messages:
  - `Co-Authored-By: ...`
  - `Generated-by: ...`
  - `Signed-off-by: Claude` (or similar)

## No SDD task IDs in commit messages

Internal task identifiers (`T01`, `T02`, `T-03`, `task-12`) exist inside
`03-tasks.md`. They are meaningless to anyone reading `git log` without
the spec open — and the spec may move, be archived, or get renumbered.

- **Forbidden** in the commit subject:
  - `feat(storage): add X (T02)`
  - `test(api): cover flow T-07`
  - `T01: chore(layers): add z-index`
- **Forbidden** in the commit body unless the body already explains *why*
  and the task ID adds real traceability. Prefer a spec path instead:
  `Refs: specs/floating-contact-bar/03-tasks.md#t02`.
- If traceability is needed, put it in the **PR description**, not in the
  commit subject.

The `git log` stays readable on its own, years after the spec is gone.

## Recommended practice

- Use focused, conventional messages (prefer **Conventional Commits**):
  - `feat: ...`
  - `fix: ...`
  - `refactor: ...`
  - `test: ...`
  - `perf: ...`
  - `docs: ...`
  - `chore: ...`
- The scope (`feat(storage): ...`) names a module or area of the codebase,
  not a task or ticket. `feat(storage)` is good; `feat(t02)` is not.
- The subject is ≤ 72 chars, imperative mood, no period.
- The body (optional) explains *why*, wraps at 72 chars, separated from
  the subject by a blank line.
- Reference external issue trackers (GitHub, Linear, Jira) in the **body**
  or a trailer, not the subject — e.g. `Closes: #123`.

## Forbidden

- AI-attribution trailers (listed above).
- SDD task IDs in commit subjects.
- Internal ticket numbers in the subject (use the body or a trailer).
- `WIP`, `fixup`, `squash me`, or `.` as a commit subject on shared branches.
- Squash-less histories of 40+ "tweak" / "fix typo" commits into a PR.
  Rebase/squash before merge when the history has no narrative value.
- Force-push to `main` / protected branches.
- Commits that mix unrelated changes ("refactor X + fix bug Y + bump dep Z").
  One logical change per commit.

## Config

The project configures `.claude/settings.json` with an empty
`attribution.commit` to prevent automatic trailers.
