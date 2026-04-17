# Rules — Match effort to feature size

Not every feature needs the full SDD. The process exists to lower downside
on risky/complex work. Running the full ceremony on a trivial change wastes
tokens, time, and attention — and makes the model look smart while shipping
bad code because it spent its budget satisfying checklists.

## Tiers

### Trivial
- Change: < 20 lines, single file, no public API, no UI, no data change.
- Examples: typo, rename private var, tighten a regex, fix a log message, tweak a constant.
- Process: **just do it**. No spec, no interview. Single Conventional Commit. Tests only if the area was already tested.

### Small
- Change: < 200 lines, 1-3 files, no schema change, no new external surface.
- Examples: add a field to an existing endpoint, fix a bug with a regression test, small UI tweak inside an existing component.
- Process: one-page `spec.md` with intent + test plan. Skip Discover/Design phases. Implement. Test. Ship.

### Medium
- Change: a feature added or changed meaningfully. Multi-file, possibly touches contracts or UI composition.
- Process: collapsed SDD — merge Discover + Design into a single `01-02.md` (1-2 pages). Tasks + Implement + Review + Ship as usual.

### Large
- Change: new feature with multiple surfaces, schema changes, or new external contracts. Cross-team impact likely.
- Process: full SDD as defined in `CLAUDE.md`.

## Heuristic

- If the interview keeps returning "obvious from context" answers, the tier is too heavy. **Move down a tier.**
- If the implementation keeps hitting scope the spec didn't anticipate, the tier is too light. **Move up a tier.**
- When in doubt between two tiers, pick the **lower** one; escalate when you hit a real unknown.

## Which rules apply per tier

| Rule                  | Trivial | Small | Medium | Large |
|-----------------------|:-------:|:-----:|:------:|:-----:|
| `concision.md`        | ✓       | ✓     | ✓      | ✓     |
| `git.md`              | ✓       | ✓     | ✓      | ✓     |
| `clarifications.md`   | on ambiguity | ✓ | ✓ | ✓ |
| `quality-gates.md`    | —       | light | full   | full  |
| `testing.md`          | —       | happy + edge | full | full |
| `security.md`         | only if touching auth / input / secrets | same | full | full |
| `performance.md`      | only if touching hot path | same | full | full |
| `code-quality.md`     | smells + typing | full | full | full |
| `ui-ux.md`            | only if UI | full | full | full |

"—" = the rule does not trigger. "only if …" = load it only when the
change actually touches that surface.

## Orchestrator responsibility

Before starting a feature, classify the tier in **one sentence** and state
the process that follows. Examples:

- "Tier: Trivial — rename private helper. Going straight to commit."
- "Tier: Small — add `emailVerified` boolean to existing `User`. Single spec page, one test."
- "Tier: Medium — introduce webhook signing. Merging Discover+Design, tasks next."
- "Tier: Large — rebuild the billing pipeline. Full SDD."

The user can override the tier.

## Forbidden

- Running the full interview (3–4 rounds) on a Trivial or Small change.
- Loading `security.md` / `performance.md` / `code-quality.md` / `ui-ux.md`
  when the change does not touch those surfaces.
- Writing a 10-page Design spec for a 2-file change.
- Skipping a tier down to ship faster on something that actually is Medium.
