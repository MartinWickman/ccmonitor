# Implementation Tasks

Update this as you progress

## Task order

Tasks 2 and 3 can be done in parallel after 1. Task 4 depends on 3, task 5 depends on 2, and task 6 depends on everything else.

## Tasks

- [x] **1. Scaffold the Go project** — Initialize Go module (`github.com/martinwickman/ccmonitor`), create directory structure (`cmd/ccmonitor/`, `internal/session/`, `internal/monitor/`), set up basic main entry point and session reader. Compiles and runs against real session files.

- [x] **2. Implement the hook handler** — Rewritten from bash to Go (`internal/hook/`). Invoked as `ccmonitor hook` subcommand. Reads JSON from stdin, maps hook events to statuses, captures last_prompt from UserPromptSubmit, writes session JSON file. Deletes own session file on SessionEnd. Cleans up stale files (>1 hour) on SessionStart/SessionEnd via `session.CleanupStale`. Captures tmux pane/title. Non-actionable notifications (`idle_prompt`, etc.) are skipped — only `permission_prompt` and `elicitation_dialog` set status to "waiting". Notification `title` and `message` fields are captured for detail text. Original bash script (`hooks/ccmonitor-hook.sh`) kept for reference.

- [x] **3. Implement the session file reader in Go** — Reader done: loads all JSON files, parses into Session structs, skips corrupt files, supports CCMONITOR_SESSIONS_DIR. Grouping by project done (`GroupByProject`, `TimeSince`). Stale session filtering: sessions inactive for 1+ hour are excluded from `LoadAll`. PID field removed — staleness is time-based instead.

- [x] **4. Implement the monitor CLI display** — Live-updating Bubble Tea TUI. Sessions grouped by project in rounded border boxes. Color coding via Lip Gloss (green=working, yellow=waiting, dim=idle, cyan=starting, red=exited). Tree-style connectors, truncated detail/prompt, italic last_prompt, relative timestamps. Auto-refresh every 1 second. Clean exit on q/Ctrl+C. Handles empty state. Uses alt-screen mode.

- [x] **5. Click-to-switch tmux pane** — Capture `$TMUX_PANE` in hook handler and write to session file. Add `tmux_pane` field to Go Session struct. Enable mouse support in Bubble Tea. Map click Y-coordinates to sessions. On click, run `tmux select-pane -t <pane>` via the new `internal/switcher` package. Show status feedback in the monitor.

- [ ] **6. Build the installer command** — Add hook configuration to Claude Code settings file, merging with existing hooks. Create sessions directory. For dev: target .claude/settings.local.json. For production: target ~/.claude/settings.json. Include uninstall option.

- [x] **7. Create test fixtures and end-to-end testing** — Integration test script (`test-integration.sh`) pipes hook events into `ccmonitor hook` and verifies `ccmonitor --once` output. Covers 6 scenarios: PreToolUse creates working session, UserPromptSubmit captures prompt, Stop sets idle, idle_prompt is ignored, permission_prompt sets waiting, SessionEnd removes session. Run via `make integration`. Fake session files in `test-sessions/` for manual monitor testing.
