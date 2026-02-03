package switcher

import (
	"fmt"
	"os/exec"
)

// Switch focuses the given tmux pane using tmux select-pane.
func Switch(tmuxPane string) error {
	if tmuxPane == "" {
		return fmt.Errorf("no tmux pane info (not running in tmux?)")
	}
	return exec.Command("tmux", "select-pane", "-t", tmuxPane).Run()
}
