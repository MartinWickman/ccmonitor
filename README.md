# ccmonitor

A terminal dashboard for monitoring multiple concurrent Claude Code instances across projects.

## How it works

ccmonitor has two components:

1. **Hook handler** (`ccmonitor hook`) — Invoked by Claude Code hooks on lifecycle events. Reads JSON from stdin and writes a status file per session to `~/.ccmonitor/sessions/`.

2. **Monitor CLI** (`ccmonitor`) — Reads the session files and renders a live-updating terminal display using Bubble Tea. Sessions are grouped by project, color-coded by status, and clickable for tmux pane / Windows Terminal tab switching.

## Prerequisites

- [Go](https://go.dev/) 1.24+ (for building from source)

## Install

Build and install the binary:

```sh
go install github.com/martinwickman/ccmonitor/cmd/ccmonitor@latest
```

Register the hooks by installing the Claude Code plugin:

```
/plugin marketplace add ./plugin
/plugin install ccmonitor
```

This registers hooks for all 7 Claude Code lifecycle events. No manual `settings.json` editing needed.

To uninstall the plugin:

```
/plugin uninstall ccmonitor
```

## Usage

Launch the live monitor:

```sh
ccmonitor
```

Print a one-time snapshot and exit:

```sh
ccmonitor --once
```

- Press `q` or `Ctrl+C` to quit the live monitor.
- Click a session to switch to its tmux pane or Windows Terminal tab.

## Platform support

ccmonitor works on Windows, Linux, and macOS. The plugin hooks call `ccmonitor hook` directly with no shell dependency.
