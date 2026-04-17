# claudiao — bundled assets

Files in this tree are embedded into the `claudiao` binary at build time
and installed into `~/.claude/` by the TUI.

## Layout

- `rules/` — global behavior rules (referenced by `CLAUDE.md`)
- `commands/` — slash commands (`/sdd-*`)
- `agents/` — sub-agent definitions
- `output-styles/` — orchestrator personas
- `templates/` — spec templates for each SDD phase

Contributors: edit files here, rebuild (`go build ./cmd/claudiao`),
and the new versions ship with the binary.
