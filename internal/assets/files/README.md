# claudiao — bundled assets

Files in this tree are embedded into the `claudiao` binary at build time
and installed into `~/.claude/` by the TUI.

## Layout

- `CLAUDE.md` — top-level SDD flow + rules index (installs at `~/.claude/CLAUDE.md`)
- `rules/` — global behavior rules (referenced by `CLAUDE.md`)
- `agents/` — SDD sub-agent definitions
- `output-styles/` — orchestrator and per-phase personas

Contributors: edit files here, rebuild (`go build ./cmd/claudiao`),
and the new versions ship with the binary.
