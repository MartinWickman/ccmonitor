package monitor

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/martinwickman/ccmonitor/internal/session"
)

func TestPadRight(t *testing.T) {
	t.Run("string shorter than width should be padded with spaces", func(t *testing.T) {
		got := padRight("hi", 5)
		if got != "hi   " {
			t.Errorf("got %q, want %q", got, "hi   ")
		}
	})

	t.Run("string equal to width should not be padded", func(t *testing.T) {
		got := padRight("hello", 5)
		if got != "hello" {
			t.Errorf("got %q, want %q", got, "hello")
		}
	})

	t.Run("string longer than width should be returned unchanged", func(t *testing.T) {
		got := padRight("hello world", 5)
		if got != "hello world" {
			t.Errorf("got %q, want %q", got, "hello world")
		}
	})
}

func TestSessionRowRender(t *testing.T) {
	sp := spinner.New()
	sp.Spinner = spinner.MiniDot

	t.Run("render produces status line and prompt line", func(t *testing.T) {
		s := session.Session{
			SessionID:    "abcd1234-full-session-id",
			Project:      "/home/user/project",
			Status:       "idle",
			Detail:       "Finished responding",
			LastPrompt:   "Fix the bug",
			LastActivity: time.Now().Add(-2 * time.Minute).Format(time.RFC3339),
		}
		row := NewSessionRow(s, true, sp, nil)
		w := columnWidths{conn: 4, id: 10, status: 12, detail: 20}
		output := row.Render(w)

		if !strings.Contains(output, "abcd1234") {
			t.Error("output should contain truncated session ID")
		}
		if !strings.Contains(output, "Idle") {
			t.Error("output should contain status label")
		}
		if !strings.Contains(output, "Finished responding") {
			t.Error("output should contain detail text")
		}
		if !strings.Contains(output, "Fix the bug") {
			t.Error("output should contain last prompt")
		}
		// Should end with newline
		if !strings.HasSuffix(output, "\n") {
			t.Error("output should end with newline")
		}
	})

	t.Run("render without prompt produces single line", func(t *testing.T) {
		s := session.Session{
			SessionID:    "abcd1234-full-session-id",
			Project:      "/home/user/project",
			Status:       "working",
			Detail:       "Edit main.go",
			LastActivity: time.Now().Format(time.RFC3339),
		}
		row := NewSessionRow(s, false, sp, nil)
		w := columnWidths{conn: 4, id: 10, status: 12, detail: 20}
		output := row.Render(w)

		lines := strings.Split(strings.TrimSuffix(output, "\n"), "\n")
		if len(lines) != 1 {
			t.Errorf("expected 1 line, got %d", len(lines))
		}
	})
}
