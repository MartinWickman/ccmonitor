package switcher

import (
	"fmt"

	"github.com/martinwickman/ccmonitor/internal/session"
	"github.com/martinwickman/ccmonitor/internal/terminal"
	"github.com/martinwickman/ccmonitor/internal/tmux"
	"github.com/martinwickman/ccmonitor/internal/wt"
)

var backends = map[string]terminal.Backend{
	"wt":   wt.Backend{},
	"tmux": tmux.Backend{},
}

// Switch focuses the terminal tab/pane for the given session.
// Iterates over s.Terminals in order — the hook adds WT first, tmux second,
// so the outer tab is switched before the inner pane.
func Switch(s session.Session) error {
	if len(s.Terminals) == 0 {
		return fmt.Errorf("no switching info available")
	}
	for _, t := range s.Terminals {
		b, ok := backends[t.Backend]
		if !ok {
			continue
		}
		if err := b.Select(t.ID); err != nil {
			return err
		}
	}
	return nil
}
