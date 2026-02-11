# ccmonitor

**A single dashboard for all your Claude Code instances.**

Running multiple Claude Code sessions across different projects and terminal tabs? ccmonitor shows you what every instance is doing — no tab-switching required.

- See all sessions at a glance, grouped by project
- Know which instances are working, waiting for input, or idle
- See what each one is doing: which tool, what file, what command
- Spot stuck or forgotten sessions with elapsed time indicators
- Automatically detect crashed instances via PID-based liveness checks
- Click to jump to the right tmux pane or Windows Terminal tab

```
ccmonitor  2 projects, 4 sessions

╭──────────────────────────────────────────────────────────────╮
│ myproject/ /home/user/myproject                              │
│ ├─ Refactor the auth module to use JWT tokens                │
│ │  ● Working  Edit src/main.py                        2m ago │
│ └─ Delete all temp files and rebuild the project             │
│    ◆ Waiting  permission                              4m ago │
╰──────────────────────────────────────────────────────────────╯

╭──────────────────────────────────────────────────────────────╮
│ webapp/ /home/user/webapp                                    │
│ ├─ Run the test suite and fix any failures                   │
│ │  ● Working  Bash: npm test                          2m ago │
│ └─ Add dark mode support to the settings page                │
│    ○ Idle     Finished responding                     7m ago │
╰──────────────────────────────────────────────────────────────╯
```

## Install

Build from source (requires [Go](https://go.dev/) 1.24+):

```sh
go install github.com/martinwickman/ccmonitor/cmd/ccmonitor@latest
```

Then register the hooks as a Claude Code plugin:

```
/plugin marketplace add ./plugin
/plugin install ccmonitor
```

This registers hooks for all 7 Claude Code lifecycle events. No manual `settings.json` editing needed.

## Usage

Launch the live monitor:

```sh
ccmonitor
```

Print a one-time snapshot and exit:

```sh
ccmonitor --once
```

Show session IDs and PIDs for debugging:

```sh
ccmonitor --debug
```

- Press `q` or `Ctrl+C` to quit the live monitor.
- Click a session to switch to its tmux pane or Windows Terminal tab.

## How it works

ccmonitor has two components that communicate through JSON files on disk:

1. **Hook handler** (`ccmonitor hook`) — Claude Code fires hooks on lifecycle events (session start, tool use, prompts, stop, etc.). The hook handler reads the event JSON from stdin and writes a status file per session to `~/.ccmonitor/sessions/`.

2. **Monitor** (`ccmonitor`) — Reads the session files and renders a live-updating terminal display using [Bubble Tea](https://github.com/charmbracelet/bubbletea). Refreshes every second. The monitor is read-only — it never writes or deletes session files.

### Status tracking

| Hook Event         | Status      | What it means                        |
|--------------------|-------------|--------------------------------------|
| `SessionStart`     | `starting`  | Session just began                   |
| `UserPromptSubmit` | `working`   | User sent a prompt                   |
| `PreToolUse`       | `working`   | Model is calling a tool              |
| `PostToolUse`      | `working`   | Tool call completed                  |
| `Notification`     | `waiting`   | Needs user attention (permission)    |
| `Stop`             | `idle`      | Finished responding                  |
| `SessionEnd`       | `ended`     | Session terminated                   |

Sessions whose PID has died without a clean `SessionEnd` are shown as `exited`.

### Stale session cleanup

The hook handler keeps the sessions directory clean automatically:

- `SessionEnd` deletes its own session file
- `SessionStart` and `SessionEnd` scan for files with dead PIDs and remove them
- Every hook event removes other files sharing the same PID (since a Claude Code process only has one active session at a time)

No daemon, no cron, no manual cleanup needed.

## Platform support

Works on Windows, Linux, and macOS. The plugin hooks call `ccmonitor hook` directly — no shell wrapper scripts, no bash dependency.

| Feature               | Linux/macOS | Windows |
|-----------------------|-------------|---------|
| Status monitoring     | Yes         | Yes     |
| Click-to-switch tmux  | Yes         | —       |
| Click-to-switch WT tab| —           | Yes     |

## Uninstall

Remove the hooks:

```
/plugin uninstall ccmonitor
```
