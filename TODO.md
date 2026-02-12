# Implementation Tasks

Update this as you progress

## Task order

Tasks 2 and 3 can be done in parallel after 1. Task 4 depends on 3, task 5 depends on 2, and task 6 depends on everything else.

## Tasks

- [x] **1. Scaffold the Go project** — Initialize Go module (`github.com/martinwickman/ccmonitor`), create directory structure (`cmd/ccmonitor/`, `internal/session/`, `internal/monitor/`), set up basic main entry point and session reader. Compiles and runs against real session files.

- [x] **2. Implement the hook handler** — Rewritten from bash to Go (`internal/hook/`). Invoked as `ccmonitor hook` subcommand. Reads JSON from stdin, maps hook events to statuses, captures last_prompt from UserPromptSubmit, writes session JSON file. Deletes own session file on SessionEnd. Cleans up dead-PID session files on SessionStart/SessionEnd via `cleanupDead`. Captures Claude Code PID by walking process tree (grandparent of hook process). Captures tmux pane/title. Non-actionable notifications (`idle_prompt`, etc.) are skipped — only `permission_prompt` and `elicitation_dialog` set status to "waiting". Notification `title` and `message` fields are captured for detail text. Original bash script (`hooks/ccmonitor-hook.sh`) kept for reference.

- [x] **3. Implement the session file reader in Go** — Reader done: loads all JSON files, parses into Session structs, skips corrupt files, supports CCMONITOR_SESSIONS_DIR. Grouping by project done (`GroupByProject`, `TimeSince`). PID-based liveness: monitor checks PIDs via `go-ps` and marks dead sessions as "exited".

- [x] **4. Implement the monitor CLI display** — Live-updating Bubble Tea TUI. Sessions grouped by project in rounded border boxes. Color coding via Lip Gloss (green=working, yellow=waiting, dim=idle, cyan=starting, red=exited). Tree-style connectors, truncated detail/prompt, italic last_prompt, relative timestamps. Auto-refresh every 1 second. Clean exit on q/Ctrl+C. Handles empty state. Uses alt-screen mode.

- [x] **5. Click-to-switch tmux pane** — Capture `$TMUX_PANE` in hook handler and write to session file. Add `tmux_pane` field to Go Session struct. Enable mouse support in Bubble Tea. Map click Y-coordinates to sessions. On click, run `tmux select-pane -t <pane>` via the new `internal/switcher` package. Show status feedback in the monitor.

- [x] **6. Create Claude Code plugin** — Plugin at `plugin/` with manifest (`plugin/.claude-plugin/plugin.json`) and hook registrations (`plugin/hooks/hooks.json`) for all 7 events. Hooks call `ccmonitor hook` directly — no wrapper script, no bash dependency. Works cross-platform (Windows, Linux, Mac) as long as the ccmonitor binary is on PATH. Install via `/plugin install ./plugin`, uninstall via `/plugin uninstall ccmonitor`.

- [x] **7. Create test fixtures and end-to-end testing** — Integration test script (`test-integration.sh`) pipes hook events into `ccmonitor hook` and verifies `ccmonitor --once` output. Covers 6 scenarios: PreToolUse creates working session, UserPromptSubmit captures prompt, Stop sets idle, idle_prompt is ignored, permission_prompt sets waiting, SessionEnd removes session. Run via `make integration`. Fake session files in `test-sessions/` for manual monitor testing.

- [x] **8. Capture Windows Terminal tab titles** — The `summary` field is now populated from WT tab names in addition to tmux pane titles. On `SessionStart`, `wtTabInfo()` captures both RuntimeID and tab name in a single PowerShell call. On subsequent events, `wtTabTitle()` looks up the tab by its stored RuntimeID and reads the current name. When both tmux and WT are present, tmux title is preferred (more specific). The `termInfoFn` signature was expanded to receive the existing RuntimeID for lookups.

- [x] **9. Swap session display lines — prompt first, status second** — Each session now renders as two lines: line 1 shows the summary/prompt with session ID in parentheses (e.g. `├─ Swap session display lines (bf623347)`), line 2 shows the status indicator, detail, and elapsed time. When no prompt/summary exists, line 1 shows just the session ID. Removed `id` from `columnWidths` since it's now inline on line 1. Updated all monitor tests.

- [x] **10. Fix stale session files on --continue/--resume** — Moved `cleanupSamePID()` from SessionStart-only to every event that writes a session file. Since a Claude Code process only has one active session at a time, removing other session files with the same PID before every write is always safe and catches stale files regardless of how they were created (`/clear` without `SessionEnd`, `--continue`/`--resume`, etc.). Added `Source` field to `hookInput` struct for future use.

- [x] **11. Add --debug flag to show/hide session IDs and PIDs** — Session IDs and PIDs are now hidden by default for a cleaner display. Pass `--debug` to show them. When debug is off and no prompt/summary exists, shows "(no description)" as a faint placeholder. The `debug` bool threads from `main.go` through `monitor.New()` / `monitor.RenderOnce()` down to `sessionRow.render()`.

- [x] **12. Easy deployment and installation** — Added `marketplace.json` at `.claude-plugin/marketplace.json` so users can install hooks from GitHub without cloning: `/plugin marketplace add martinwickman/ccmonitor` then `/plugin install ccmonitor`. Added `install.sh` for downloading the binary from GitHub Releases with platform detection, checksum verification, and smart install directory defaults. Updated README with three install methods: quick install script, from source, and manual download.
