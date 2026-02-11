# Claude Code Monitor

**A terminal dashboard for Claude Code instances.**

`ccmonitor` shows you what every CC instance is doing *right now* or if any of them are waiting for your response.
So hopefully less hunting for correct terminal tab...

- See all sessions at a glance, grouped by project
- Which Claude sessions are working, waiting for input, or just idling
- It also shows your latest prompt (or summary) for each session
- Click to jump to the right tmux pane or Windows Terminal tab

![](recording.gif)

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

This registers hooks for all 7 Claude Code lifecycle events.

Note: `ccmonitor` should be placed in your PATH or the hooks wont work.

## Usage

Start the monitor in a terminal

```sh
ccmonitor
```

- Press `q` or `Ctrl+C` to quit
- Click a session to switch to its tmux pane or Windows Terminal tab.
- 'p' to toggle between prompt or summary display

Print a one-time snapshot and exit:

```sh
ccmonitor --once
```

## How it works

It installs a few hooks into Claude Code. These hooks reports to the ccmonitor instances by keeping state in your home directory (`~/.ccmonitor/`).

### Quirks

The summary display may lag or be wonky from time to time, again because of how Claude Code hooks work and the limited info we get from Claude.

ccmonitor tries to clean up the stale/dead sessions automatically, but the way
Claude Code hooks works can make this a bit shaky, so if you end up with duplicate sessions in the list,
run `ccmonitor --clean` to remove all stale sessions.

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

## Future work

* Should support more terminals (e.g. Iterm2 etc)
* Running the display in a browser
* Possibly respond to Claude Code via the monitor. 