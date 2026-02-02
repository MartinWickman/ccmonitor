# ccmonitor

A monitoring tool for tracking multiple concurrent Claude Code instances across projects.

## What this project does

ccmonitor uses Claude Code hooks to collect status data from all running instances and displays them in a single live-updating view. See `PRD.md` for the full product description.

## Architecture decisions

- **Data collection**: Claude Code hooks write JSON status files to `~/.ccmonitor/sessions/<session_id>.json`
- **Async hooks**: Tool-related hooks (PreToolUse, PostToolUse, UserPromptSubmit) run with `async: true` to avoid adding latency to Claude Code
- **Atomic writes**: Status files are written via temp file + rename to prevent partial reads
- **Stale detection**: Monitor checks if session PIDs are alive; dead sessions are auto-removed after ~5 minutes
- **Status model**: starting → working ⇄ idle → ended, with "waiting" for when user input is needed

## Tech stack

Not yet decided. Candidates:
- Python + rich (fast to build, good terminal UI)
- Go (single binary, no runtime dependency)
- Node/TypeScript + ink (same ecosystem as Claude Code)

## Project structure

Not yet scaffolded. Will contain:
- A hook handler (reads JSON from stdin, writes status files)
- A monitor CLI (reads status files, displays live table)
- An installer (adds hooks to ~/.claude/settings.json)

## Key files

- `PRD.md` — Full product requirements document
