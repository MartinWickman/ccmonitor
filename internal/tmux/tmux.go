// Package tmux provides tmux pane operations (info, title refresh, switching).
package tmux

import (
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/martinwickman/ccmonitor/internal/terminal"
)

// Backend implements terminal.Backend for tmux panes.
type Backend struct{}

var _ terminal.Backend = Backend{}

// Name returns "tmux".
func (Backend) Name() string { return "tmux" }

// Available reports whether the current process is running inside tmux.
func (Backend) Available() bool { return os.Getenv("TMUX_PANE") != "" }

// Info reads the current tmux pane ID from $TMUX_PANE and fetches its
// title. Returns empty strings when not running inside tmux.
func (Backend) Info() (paneID, title string) {
	paneID = os.Getenv("TMUX_PANE")
	if paneID == "" {
		return "", ""
	}
	out, err := exec.Command("tmux", "display-message", "-p", "-t", paneID, "#{pane_title}").Output()
	if err != nil {
		return paneID, ""
	}
	title = strings.TrimSpace(string(out))
	title = terminal.StripTitlePrefix(title)
	return paneID, title
}

// Title refreshes the title for a known tmux pane ID.
// Returns empty string on error.
func (Backend) Title(paneID string) string {
	if paneID == "" {
		return ""
	}
	out, err := exec.Command("tmux", "display-message", "-p", "-t", paneID, "#{pane_title}").Output()
	if err != nil {
		return ""
	}
	title := strings.TrimSpace(string(out))
	return terminal.StripTitlePrefix(title)
}

// Select switches focus to the given tmux pane.
// On Windows, tmux is accessed via WSL.
func (Backend) Select(paneID string) error {
	if runtime.GOOS == "windows" {
		return exec.Command("wsl", "tmux", "select-pane", "-t", paneID).Run()
	}
	return exec.Command("tmux", "select-pane", "-t", paneID).Run()
}
