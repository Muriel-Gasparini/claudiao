# Rules — Quality gates (compact)

The model has a positive bias toward its own output. Self-approval is
not evidence. This rule defines two independent checks: a cheap
self-critique by the writer, and an expensive adversarial review by an
independent subagent.

## Self-critique by the writer (cheap, always)

Before declaring "done" on any non-trivial change, the writer MUST:

1. List **3 ways the code could be wrong** (logic error, edge case,
   wrong assumption, missing validation, race, off-by-one, etc.).
2. For each, **check the actual code** and report whether the failure
   mode is present.
3. If the code is fine, say so with the verification (e.g. *"checked
   line X, the bound is correct because Y"*).

Skipping this step is forbidden on changes > 20 lines or any change
that touches business logic.

## Adversarial review by `sdd-reviewer` (mandatory on sensitive areas)

When the diff touches a sensitive area (list in `~/.claude/CLAUDE.md`),
the writer MUST call:

```
Agent({
  subagent_type: "sdd-reviewer",
  description: "Adversarial review",
  prompt: "Diff: <paths/summary>. Sensitive areas touched: <auth/crypto/...>.
           Posture: 'wrong until proven otherwise'. Find failures.
           Use Grep/Read on the changed files. If nothing found after a
           systematic pass, report 'no obvious failures'."
})
```

The writer **does not ask the user** whether to review. The review runs.

## Reviewer output format

For each finding:

```
### [Severity: blocker|major|minor] <short title>

**Where**: <file:line>
**Problem**: <what is wrong, concrete>
**Impact**: <why it matters; possible exploit/failure>
**Fix**: <patch or direction>
**Validation**: <how to confirm the fix works>
```

Final table, even when empty:

```
| Severity | Count | Items |
|----------|-------|-------|
| Blocker  | 0     | —     |
| Major    | 0     | —     |
| Minor    | 0     | —     |
```

## Severity definitions

- **Blocker** — exploitable vulnerability, broken contract, data loss,
  authz bypass, wrong data persisted. Fix before commit.
- **Major** — edge case that will explode in production, NFR not met,
  weak/fake test. Fix before commit.
- **Minor** — naming, formatting, prose nit. May ship; record as a TODO
  with link/issue if not fixed now.

When in doubt, rank up. False positives cost less than missed bugs.

## Resolution

- Blocker + Major: fix before commit, then re-run the reviewer if the
  fix touched non-trivial logic.
- Minor: fix now, or open an issue and reference it in the commit body.

## Anti-patterns — rejected

- Writer self-reviewing and declaring "secure" without invoking
  `sdd-reviewer` on sensitive areas.
- Skipping the reviewer because "this is a small change" — small
  changes in sensitive areas are exactly where bugs slip through.
- Downgrading a Major to Minor to ship faster.
- Hiding findings in prose instead of the severity table.
- Findings without `Where:` (file:line).
