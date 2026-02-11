package switcher

import (
	"fmt"
	"os/exec"

	"github.com/martinwickman/ccmonitor/internal/session"
	"github.com/martinwickman/ccmonitor/internal/wt"
)

// Switch focuses the terminal tab/pane for the given session.
// When both WT tab and tmux pane are available (tmux inside WT),
// it switches the WT tab first, then the tmux pane.
func Switch(s session.Session) error {
	if s.RuntimeID != "" && s.TmuxPane != "" {
		if err := wt.SelectTab(s.RuntimeID); err != nil {
			return err
		}
		return exec.Command("tmux", "select-pane", "-t", s.TmuxPane).Run()
	}
	if s.RuntimeID != "" {
		return wt.SelectTab(s.RuntimeID)
	}
	if s.TmuxPane != "" {
		return exec.Command("tmux", "select-pane", "-t", s.TmuxPane).Run()
	}
	return fmt.Errorf("no switching info available")
}
