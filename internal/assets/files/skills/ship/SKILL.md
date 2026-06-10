---
name: ship
description: Run the full claudiao agreement ritual before a commit — self-critique, machine checks, adversarial review with receipt on sensitive areas, mutation check, then a Conventional Commit. Use when the user says "ship", "ship it", "commit this properly", or asks to finish and commit a change the right way.
---

# /ship — the agreement ritual, in one command

This skill executes the default flow from the working agreement end to end,
using the `claudiao` binary for the mechanical parts. Do not skip steps; each
one is a gate. If a gate fails, stop and fix before moving on.

If `claudiao` is not on PATH, say so and fall back to the manual flow
(self-critique → reviewer if sensitive → tests → commit); the steps still
apply, only the automation is missing.

## Step 1 — Self-critique (always)

List **3 concrete ways this change could be wrong** (logic error, edge case,
wrong assumption, missing validation, race, off-by-one). For each, check the
actual code and report present/absent with the line. If you cannot find 3,
you have not looked hard enough.

## Step 2 — Machine antipattern check

Run from the repo root:

```
claudiao check
```

This runs the `claudiao-check` blocks embedded in the installed rules against
the added lines of `git diff HEAD`. Resolve every **blocker** and **major**
before continuing. Minors may ship with a noted follow-up.

## Step 3 — Adversarial review + receipt (only if the diff is sensitive)

Determine whether the diff touches a sensitive area (auth, crypto, input
validation, DB queries, public endpoints, authz, secrets, schema, new deps,
path/SSRF, templates). If it does:

1. Invoke the reviewer yourself — do not ask the user:
   `Agent(subagent_type: "sdd-reviewer", …)` passing the scope, not the
   full diff: changed-file list from `git diff HEAD --stat` + sensitive
   areas touched + acceptance criteria (the reviewer scopes hunks itself
   via git).
2. Resolve every Blocker and Major. Re-run the reviewer if a fix touched
   non-trivial logic — the reviewer records a fresh receipt after a clean
   pass.
3. Confirm the receipt is valid for the current tree (the reviewer is the
   one who creates it; any edit after the review stales it):
   ```
   claudiao receipt verify
   ```
   If it is missing or stale, the review did not cover this tree —
   re-review; do not hand-mint a receipt.

The commit gate (if the Hooks module is installed) **blocks** a sensitive
commit without a valid receipt — so this step is mandatory, not optional,
when the diff is sensitive.

## Step 4 — Mutation check (when tests changed or were added)

Prove the tests around the change can actually fail:

```
claudiao mutate
```

If mutants survive, the tests are decoration. Either strengthen them
yourself, or hand the survivors to the fortifier:

```
claudiao mutate --json   # feed the survivors to Agent(subagent_type: "test-fortifier")
```

Re-run until all mutants are killed (or an equivalent mutant is explained).

## Step 5 — Commit

Write a **Conventional Commit** (`feat:`, `fix:`, `refactor:`, `test:`,
`docs:`, `chore:`, `perf:`). Subject ≤ 72 chars, imperative, no period. Body
explains *why*. **No AI-attribution trailers** — the commit gate rejects them.

If the user has not authorized committing yet, present the proposed message
and the gate results, and ask before running `git commit`.

## Done

Report, in two lines max: what was checked (check/reviewer/mutate results)
and the commit subject. Do not claim "secure" or "tests pass" unless the
evidence above was actually produced — the Stop hook will block an
unbacked claim anyway.
