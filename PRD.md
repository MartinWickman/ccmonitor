# ccmonitor — Claude Code Instance Monitor

## Problem

When running multiple Claude Code instances simultaneously — across different projects, in different terminal tabs — it's difficult to know what each instance is doing. You have to manually switch between tabs to check if an instance is:

- Actively working (thinking, calling tools)
- Waiting for input (permission prompt, idle)
- Finished and idle
- Crashed or exited

This becomes increasingly painful as the number of concurrent instances grows.

## Requirements

- See all Claude Code instances in a single view — no tab-switching
- Instances grouped by project directory
- Each instance shows its current status: working, waiting for input, idle
- See what each instance is doing: which tool, what file/command
- Show time since last activity to spot stuck or forgotten sessions
- Automatically detect and flag dead/crashed instances
- Live-updating display, no manual refresh
- Zero noticeable performance impact on Claude Code

### Future (out of scope for v1)

- Send prompts to waiting instances directly from the monitor
- Desktop notifications when an instance needs attention
- Graphical dashboard beyond the terminal

## Solution

ccmonitor is a monitoring tool that provides a single view of all running Claude Code instances, grouped by project. It consists of two parts:

1. **Hook-based data collection** — Claude Code hooks fire on lifecycle events (tool use, prompts, notifications, stop). A hook handler script writes the current status of each session to a shared directory.
2. **Monitor display** — A CLI program reads the status files and renders a live-updating view showing all instances and their current state.

## How It Works

### Data Collection via Hooks

Claude Code supports user-defined hooks that execute at specific lifecycle points. The following hooks are used to track instance state:

| Hook Event         | Triggers When                          | Status Set     |
|--------------------|----------------------------------------|----------------|
| `SessionStart`     | A Claude Code session begins           | `starting`     |
| `UserPromptSubmit` | User sends a prompt                    | `working`      |
| `PreToolUse`       | Model is about to call a tool          | `working`      |
| `PostToolUse`      | A tool call completed                  | `working`      |
| `Notification`     | Claude needs user attention            | `waiting`      |
| `Stop`             | Model finished responding              | `idle`         |
| `SessionEnd`       | Session terminates                     | `ended`        |

Each hook invocation receives JSON on stdin containing `session_id`, `cwd` (project directory), `hook_event_name`, and event-specific fields like `tool_name` and `tool_input`.

The hook handler writes a status file per session to `~/.ccmonitor/sessions/<session_id>.json`:

```json
{
  "session_id": "abc123",
  "project": "/home/user/myproject",
  "status": "working",
  "detail": "Edit src/main.py",
  "last_prompt": "Write a book about Go programming",
  "notification_type": null,
  "last_activity": "2026-02-02T14:30:00Z",
  "pid": 12345
}
```

The `last_prompt` field is captured from the `UserPromptSubmit` hook and gives a rough indication of what the session is working on. It persists across tool calls until the user sends a new prompt.

### Stale Session Detection

The monitor checks whether each session's PID is still alive. If a process has died (e.g. terminal was closed), the session is shown as "exited" in the display.

Cleanup of stale session files is handled by the hook handler itself: the `SessionEnd` hook deletes its own session file and scans for other files with dead PIDs. The `SessionStart` hook does the same scan. This means crashed sessions get cleaned up the next time any Claude Code session starts or ends, with no daemon or manual intervention needed.

### Status File Integrity

The hook handler writes directly to the session file. The monitor must handle JSON parse errors gracefully (skip and retry on the next cycle) in the unlikely event of a read during a write.

## Status Model

An instance moves through these states:

```
starting → working ⇄ idle
               ↓
           waiting (needs input/permission)
               ↓
           working (after user responds)
               ↓
             ended
```

- **starting** — Session just began, no activity yet
- **working** — Model is thinking, calling tools, or processing results
- **idle** — Model finished responding, waiting for user's next prompt
- **waiting** — Model needs user attention (permission dialog, idle prompt, etc.)
- **ended** — Session terminated normally
- **exited** — Process died without a clean SessionEnd (detected by PID check)

## Monitor Display

The CLI monitor shows a live-updating terminal view using Bubble Tea and Lip Gloss, refreshing every ~1 second. Each project is displayed in a rounded border box with tree-style session listing:

```
ccmonitor  2 projects, 4 sessions

╭──────────────────────────────────────────────────────────────╮
│ myproject/ /home/user/myproject                              │
│ ├─ abc12345  ● Working  Edit src/main.py              2m ago │
│ │  Refactor the authentication module to use JWT tokens      │
│ └─ def67890  ◆ Waiting  permission                    4m ago │
│    Delete all temp files and rebuild the project             │
╰──────────────────────────────────────────────────────────────╯

╭──────────────────────────────────────────────────────────────╮
│ webapp/ /home/user/webapp                                    │
│ ├─ ghi11111  ● Working  Bash: npm test                2m ago │
│ │  Run the test suite and fix any failures                   │
│ └─ jkl22222  ○ Idle     Finished responding            7m ago │
│    Add dark mode support to the settings page                │
╰──────────────────────────────────────────────────────────────╯

Press q to quit.
```

Color coding (via Lip Gloss):
- Green: working
- Yellow: waiting for input
- Dim/gray: idle
- Cyan: starting
- Red: exited/dead

Additional display features:
- Last user prompt shown in italic below each session
- Session IDs truncated to 8 characters
- Detail text truncated to 40 characters
- Relative timestamps (e.g. "2m ago", "now")
- Alt-screen mode (restores terminal on exit)
- Clean exit on q or Ctrl+C

## Distribution

The project will be open source on GitHub, packaged for easy installation by any developer. Installation should:

1. Install the hook handler and monitor as CLI commands
2. Add hook configuration to `~/.claude/settings.json` (merging with existing hooks, not overwriting)
3. Create the `~/.ccmonitor/sessions/` directory

An uninstall command should cleanly remove the hooks from settings and optionally remove the sessions directory.

The packaging approach depends on the chosen tech stack:
- **Python**: publish to PyPI, install via `pip install ccmonitor` or `pipx install ccmonitor`
- **Go**: single binary, distribute via GitHub releases and `go install`
- **Node/TypeScript**: publish to npm, install via `npm install -g ccmonitor`

## Future Directions

These are not in scope for the initial version but inform the architecture:

### Interactive Control
- Send prompts to waiting instances directly from the monitor
- Approve/deny permission requests from the monitor
- Requires interfacing with Claude Code's stdin/IPC (significant complexity)

### GUI Version
- Web dashboard or Electron app reading the same status files
- Richer visualization: timeline of activity, resource usage
- The file-based status approach makes this straightforward — any program can read the JSON files

### Task Summarization
- Replace the raw `last_prompt` with an AI-generated summary of the current task using a `type: "prompt"` hook on `UserPromptSubmit` or `Stop`
- A fast model (e.g. Haiku) reads the conversation context and produces a ~10 word summary like "Writing chapter 4 of Go book"
- Updates as the session progresses through subtasks, not just when the user sends a new prompt

### Richer Status Data
- Track token usage per session
- Track cumulative tool calls
- Show the model's last text output summary
- Session duration and cost estimation

### Notifications
- Desktop notifications when any instance transitions to "waiting"
- Configurable alert rules (e.g. notify if any instance has been idle for >5 minutes)
