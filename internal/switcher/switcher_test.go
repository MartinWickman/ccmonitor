package switcher

import (
	"testing"

	"github.com/martinwickman/ccmonitor/internal/session"
)

func TestSwitch(t *testing.T) {
	t.Run("empty session should return an error", func(t *testing.T) {
		err := Switch(session.Session{})
		if err == nil {
			t.Error("expected error for empty session, got nil")
		}
	})
}
