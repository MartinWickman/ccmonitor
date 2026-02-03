package switcher

import "testing"

func TestSwitch(t *testing.T) {
	t.Run("empty pane string should return an error", func(t *testing.T) {
		err := Switch("")
		if err == nil {
			t.Error("expected error for empty pane string, got nil")
		}
	})
}
