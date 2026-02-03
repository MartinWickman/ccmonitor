# ccmonitor

A terminal dashboard for monitoring multiple concurrent Claude Code instances across projects.

## How it works

ccmonitor has two components:

1. **Hook handler** (`hooks/ccmonitor-hook.sh`) — A Bash script invoked by Claude Code hooks on lifecycle events. It reads JSON from stdin and writes a status file per session to `~/.ccmonitor/sessions/`.

2. **Monitor CLI** (`ccmonitor`) — A Go program that reads the session files and renders a live-updating terminal display using Bubble Tea. Sessions are grouped by project, color-coded by status, and clickable for tmux pane switching.

## Prerequisites

- [Go](https://go.dev/) 1.24+
- [jq](https://jqlang.github.io/jq/)

## Install

Build and install the monitor:

```sh
go install github.com/martinwickman/ccmonitor/cmd/ccmonitor@latest
```

Create the sessions directory:

```sh
mkdir -p ~/.ccmonitor/sessions
```

Add hooks to `~/.claude/settings.json` (merge with any existing hooks):

```json
{
  "hooks": {
    "SessionStart": [{ "type": "command", "command": "/path/to/ccmonitor-hook.sh" }],
    "UserPromptSubmit": [{ "type": "command", "command": "/path/to/ccmonitor-hook.sh" }],
    "PreToolUse": [{ "type": "command", "command": "/path/to/ccmonitor-hook.sh" }],
    "PostToolUse": [{ "type": "command", "command": "/path/to/ccmonitor-hook.sh" }],
    "Notification": [{ "type": "command", "command": "/path/to/ccmonitor-hook.sh" }],
    "Stop": [{ "type": "command", "command": "/path/to/ccmonitor-hook.sh" }],
    "SessionEnd": [{ "type": "command", "command": "/path/to/ccmonitor-hook.sh" }]
  }
}
```

Replace `/path/to/ccmonitor-hook.sh` with the absolute path to the hook script.

## Usage

Launch the live monitor:

```sh
ccmonitor
```

Print a one-time snapshot and exit:

```sh
ccmonitor -once
```

- Press `q` or `Ctrl+C` to quit the live monitor.
- Click a session to switch to its tmux pane (when running inside tmux).
