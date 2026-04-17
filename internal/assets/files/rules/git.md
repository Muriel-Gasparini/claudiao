# Rules — Git: no AI attribution in commits

## Policy

- **Forbidden**: inserting AI authorship trailers in commit messages, e.g.:
  - `Co-Authored-By: ...`
  - `Generated-by: ...`
  - `Signed-off-by: Claude` (or similar)

## Recommended practice

- Use focused, conventional messages (prefer **Conventional Commits**):
  - `feat: ...`
  - `fix: ...`
  - `refactor: ...`
  - `test: ...`
- The commit body is optional; use it when you need to explain *why*.

## Config

The project configures `.claude/settings.json` with an empty `attribution.commit`
to prevent automatic trailers.
