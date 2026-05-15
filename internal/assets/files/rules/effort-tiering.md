# Rules — Effort tiering (compact)

No nominal tiers. Three binary questions per change. Answer them, then
act.

## 1. Does the diff touch a sensitive area?

The list lives in `~/.claude/CLAUDE.md` (auth, crypto, input validation,
DB queries, public endpoints, authz, secrets, schema, new deps,
path/SSRF/XSS).

- **Yes** → adversarial reviewer (`sdd-reviewer`) is mandatory.
- **No** → reviewer is optional. Use it when the change exceeds ~50
  lines or touches a critical code path.

## 2. Does the change span > 1 file or > ~50 lines?

- **Yes** → write a short plan in the conversation (`TodoWrite` or
  Plan tool) before implementing.
- **No** → go directly.

## 3. Is there real ambiguity in the request?

- **Yes** → `AskUserQuestion` with 1-3 grouped questions in a single
  call.
- **No** → 0 questions.

## Heuristic for surprise scope

If during implementation you discover scope the request did not
anticipate (new files, new endpoint, new dep): **stop**. Report to the
user. Ask whether to expand or open a follow-up. Do not silently grow
the change.

## Forbidden

- Creating numbered spec files (`01-discover.md`, …, `06-ship.md`).
- Skipping the reviewer when a sensitive area was touched.
- Asking 2+ rounds of questions when the request is clear.
- Expanding scope silently.
