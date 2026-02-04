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

func TestFlashPhase(t *testing.T) {
	t.Run("zero until time should return no flash", func(t *testing.T) {
		got := flashPhase(time.Now(), time.Time{})
		if got != 0 {
			t.Errorf("got %d, want 0", got)
		}
	})

	t.Run("expired flash should return no flash", func(t *testing.T) {
		now := time.Now()
		until := now.Add(-1 * time.Second)
		got := flashPhase(now, until)
		if got != 0 {
			t.Errorf("got %d, want 0", got)
		}
	})

	t.Run("active flash should return 1 or 2", func(t *testing.T) {
		now := time.Now()
		until := now.Add(1 * time.Second)
		got := flashPhase(now, until)
		if got != 1 && got != 2 {
			t.Errorf("got %d, want 1 or 2", got)
		}
	})
}

func TestBuildClickMap(t *testing.T) {
	t.Run("empty sessions should return empty map", func(t *testing.T) {
		got := buildClickMap(nil, "some view\ncontent\n")
		if len(got) != 0 {
			t.Errorf("got %d entries, want 0", len(got))
		}
	})

	t.Run("line containing session ID should be mapped", func(t *testing.T) {
		sessions := []session.Session{
			{SessionID: "abcd1234-full-id", Project: "/p"},
		}
		view := "header\nsummary\n├─ abcd1234  Working\n"
		got := buildClickMap(sessions, view)
		if got[2] != "abcd1234-full-id" {
			t.Errorf("line 2: got %q, want %q", got[2], "abcd1234-full-id")
		}
	})

	t.Run("line after session row should be mapped as prompt line", func(t *testing.T) {
		sessions := []session.Session{
			{SessionID: "abcd1234-full-id", Project: "/p"},
		}
		view := "header\n├─ abcd1234  Working\n   Fix the bug\nfooter\n"
		got := buildClickMap(sessions, view)
		if got[1] != "abcd1234-full-id" {
			t.Errorf("session row line 1: got %q, want %q", got[1], "abcd1234-full-id")
		}
		if got[2] != "abcd1234-full-id" {
			t.Errorf("prompt line 2: got %q, want %q", got[2], "abcd1234-full-id")
		}
	})

	t.Run("consecutive session rows should not bleed into each other", func(t *testing.T) {
		sessions := []session.Session{
			{SessionID: "aaaaaaaa-1111", Project: "/p"},
			{SessionID: "bbbbbbbb-2222", Project: "/p"},
		}
		view := "header\n├─ aaaaaaaa  Working\n└─ bbbbbbbb  Idle\nfooter\n"
		got := buildClickMap(sessions, view)
		if got[1] != "aaaaaaaa-1111" {
			t.Errorf("line 1: got %q, want %q", got[1], "aaaaaaaa-1111")
		}
		if got[2] != "bbbbbbbb-2222" {
			t.Errorf("line 2: got %q, want %q", got[2], "bbbbbbbb-2222")
		}
	})

	t.Run("lines without any session ID should not be mapped", func(t *testing.T) {
		sessions := []session.Session{
			{SessionID: "abcd1234-full-id", Project: "/p"},
		}
		view := "header line\nproject title\n├─ abcd1234  Working\n"
		got := buildClickMap(sessions, view)
		if _, ok := got[0]; ok {
			t.Errorf("header line should not be mapped")
		}
		if _, ok := got[1]; ok {
			t.Errorf("project title should not be mapped")
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
