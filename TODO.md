# Implementation Tasks

## Task order

Tasks 2 and 3 can be done in parallel after 1. Task 4 depends on 3, task 5 depends on 2, and task 6 depends on everything else.

## Tasks

- [ ] **1. Scaffold the Go project** — Initialize Go module, create directory structure, set up basic project layout with main entry points and shared constants (session dir path, CCMONITOR_SESSIONS_DIR env var override).

- [ ] **2. Implement the hook handler bash script** — Read JSON from stdin, map hook events to statuses, capture last_prompt from UserPromptSubmit, write session JSON file. Include stale session cleanup on SessionStart and SessionEnd (scan for dead PIDs, remove their files). Delete own session file on SessionEnd. Support CCMONITOR_SESSIONS_DIR env var.

- [ ] **3. Implement the session file reader in Go** — Read all session JSON files from the sessions directory, parse them, check PID liveness, return a structured list of sessions grouped by project. Handle JSON parse errors gracefully (skip corrupt files). Support CCMONITOR_SESSIONS_DIR env var.

- [ ] **4. Implement the monitor CLI display** — Live-updating terminal view of all sessions grouped by project. Color coding: green=working, yellow=waiting, dim=idle, red=exited. Show tool detail, last_prompt, time since last activity. Auto-refresh every ~1 second. Handle empty state. Clean exit on Ctrl+C.

- [ ] **5. Build the installer command** — Add hook configuration to Claude Code settings file, merging with existing hooks. Create sessions directory. For dev: target .claude/settings.local.json. For production: target ~/.claude/settings.json. Include uninstall option.

- [ ] **6. Create test fixtures and end-to-end testing** — Script to generate fake session files for monitor testing. Pipe sample JSON to hook handler to test it. Verify full flow: install hooks → start Claude Code session → confirm status updates → confirm cleanup on session end.
