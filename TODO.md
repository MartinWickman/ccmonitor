# Implementation Tasks

Update this as you progress

## Task order

Tasks 2 and 3 can be done in parallel after 1. Task 4 depends on 3, task 5 depends on 2, and task 6 depends on everything else.

## Tasks

- [x] **1. Scaffold the Go project** — Initialize Go module (`github.com/martinwickman/ccmonitor`), create directory structure (`cmd/ccmonitor/`, `internal/session/`, `internal/monitor/`), set up basic main entry point and session reader. Compiles and runs against real session files.

- [x] **2. Implement the hook handler bash script** — Read JSON from stdin, map hook events to statuses, capture last_prompt from UserPromptSubmit, write session JSON file. Delete own session file on SessionEnd. Support CCMONITOR_SESSIONS_DIR env var. Note: stale session cleanup was removed — PID-based detection was unreliable (PPID captures ephemeral process, not Claude Code). Stale detection to be revisited later.

- [~] **3. Implement the session file reader in Go** — Reader done: loads all JSON files, parses into Session structs, skips corrupt files, supports CCMONITOR_SESSIONS_DIR. Grouping by project done (`GroupByProject`, `TimeSince`). Still needed: PID liveness checks (pending reliable PID strategy).

- [x] **4. Implement the monitor CLI display** — Live-updating Bubble Tea TUI. Sessions grouped by project in rounded border boxes. Color coding via Lip Gloss (green=working, yellow=waiting, dim=idle, cyan=starting, red=exited). Tree-style connectors, truncated detail/prompt, italic last_prompt, relative timestamps. Auto-refresh every 1 second. Clean exit on q/Ctrl+C. Handles empty state. Uses alt-screen mode.

- [ ] **5. Build the installer command** — Add hook configuration to Claude Code settings file, merging with existing hooks. Create sessions directory. For dev: target .claude/settings.local.json. For production: target ~/.claude/settings.json. Include uninstall option.

- [ ] **6. Create test fixtures and end-to-end testing** — Script to generate fake session files for monitor testing. Pipe sample JSON to hook handler to test it. Verify full flow: install hooks → start Claude Code session → confirm status updates → confirm cleanup on session end.
