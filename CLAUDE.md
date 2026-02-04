# ccmonitor

A monitoring tool for tracking multiple concurrent Claude Code instances across projects.

ccmonitor uses Claude Code hooks to collect status data from all running instances and displays them in a single live-updating view.

## Development guidelines

- **Never modify `~/.claude/settings.json`** during development. Use `.claude/settings.local.json` (project-scoped, gitignored) for testing hooks.
- Both the hook handler and monitor must support a `CCMONITOR_SESSIONS_DIR` environment variable that overrides the default `~/.ccmonitor/sessions/` path. Use this to point at a local test directory during development.
- Use fake session files to test the monitor UI without needing live Claude Code sessions.
- Don't git commit unless told to!
- To parse and see the output, you can run the command with `ccmonitor --once` to just have it run, print the output and then exit.
- Run unit tests
- Update the @TODO file to keep track.

## Key files

- @PRD.md — Product requirements
- @ARCHITECTURE.md — Architecture decisions, data flow, session file schema, tech stack
- @TODO.md - Progress and what to do next. Update this as you progress
- GO_BEST_PRACTICES.md - READ THIS WHEN WRITING GO PROGRAMMING LANGUAGE CODE
