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

	t.Run("render produces prompt line then status line", func(t *testing.T) {
		s := session.Session{
			SessionID:    "abcd1234-full-session-id",
			Project:      "/home/user/project",
			Status:       "idle",
			Detail:       "Finished responding",
			LastPrompt:   "Fix the bug",
			LastActivity: time.Now().Add(-2 * time.Minute).Format(time.RFC3339),
		}
		row := newSessionRow(s, true, sp, nil, true, true)
		w := columnWidths{conn: 4, status: 12, contentWidth: 80}
		output := row.render(w)

		lines := strings.Split(strings.TrimSuffix(output, "\n"), "\n")
		if len(lines) != 2 {
			t.Errorf("expected 2 lines, got %d", len(lines))
		}
		// Line 1 should have prompt and shortID in parens
		if !strings.Contains(lines[0], "Fix the bug") {
			t.Error("line 1 should contain prompt text")
		}
		if !strings.Contains(lines[0], "(abcd1234)") {
			t.Error("line 1 should contain session ID in parens")
		}
		// Line 2 should have status and detail
		if !strings.Contains(lines[1], "Idle") {
			t.Error("line 2 should contain status label")
		}
		if !strings.Contains(lines[1], "Finished responding") {
			t.Error("line 2 should contain detail text")
		}
		if !strings.HasSuffix(output, "\n") {
			t.Error("output should end with newline")
		}
	})

	t.Run("render without prompt shows ID on line 1 and status on line 2", func(t *testing.T) {
		s := session.Session{
			SessionID:    "abcd1234-full-session-id",
			Project:      "/home/user/project",
			Status:       "working",
			Detail:       "Edit main.go",
			LastActivity: time.Now().Format(time.RFC3339),
		}
		row := newSessionRow(s, false, sp, nil, true, true)
		w := columnWidths{conn: 4, status: 12, contentWidth: 80}
		output := row.render(w)

		lines := strings.Split(strings.TrimSuffix(output, "\n"), "\n")
		if len(lines) != 2 {
			t.Errorf("expected 2 lines, got %d", len(lines))
		}
		if !strings.Contains(lines[0], "abcd1234") {
			t.Error("line 1 should contain truncated session ID")
		}
		if !strings.Contains(lines[1], "Edit main.go") {
			t.Error("line 2 should contain detail text")
		}
	})
}
