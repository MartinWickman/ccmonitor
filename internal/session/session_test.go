package session

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func writeSessionFile(t *testing.T, dir string, s Session) {
	t.Helper()
	data, err := json.Marshal(s)
	if err != nil {
		t.Fatalf("marshal session: %v", err)
	}
	path := filepath.Join(dir, s.SessionID+".json")
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("write session file: %v", err)
	}
}

func TestLoadAll(t *testing.T) {
	t.Run("empty directory should return no sessions", func(t *testing.T) {
		dir := t.TempDir()
		sessions, err := LoadAll(dir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(sessions) != 0 {
			t.Errorf("got %d sessions, want 0", len(sessions))
		}
	})

	t.Run("nonexistent directory should return nil without error", func(t *testing.T) {
		sessions, err := LoadAll("/nonexistent/path")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if sessions != nil {
			t.Errorf("got %v, want nil", sessions)
		}
	})

	t.Run("valid session file should parse all fields correctly", func(t *testing.T) {
		dir := t.TempDir()
		writeSessionFile(t, dir, Session{
			SessionID: "abc123",
			Project:   "/home/user/project",
			Status:    "working",
			Detail:    "Edit main.go",
		})

		sessions, err := LoadAll(dir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(sessions) != 1 {
			t.Fatalf("got %d sessions, want 1", len(sessions))
		}
		if sessions[0].SessionID != "abc123" {
			t.Errorf("got session ID %q, want %q", sessions[0].SessionID, "abc123")
		}
		if sessions[0].Status != "working" {
			t.Errorf("got status %q, want %q", sessions[0].Status, "working")
		}
	})

	t.Run("file with invalid JSON should be skipped without error", func(t *testing.T) {
		dir := t.TempDir()
		writeSessionFile(t, dir, Session{
			SessionID: "valid1",
			Project:   "/home/user/project",
			Status:    "idle",
		})
		if err := os.WriteFile(filepath.Join(dir, "corrupt.json"), []byte("{invalid json"), 0644); err != nil {
			t.Fatalf("write corrupt file: %v", err)
		}

		sessions, err := LoadAll(dir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(sessions) != 1 {
			t.Errorf("got %d sessions, want 1 (corrupt file should be skipped)", len(sessions))
		}
	})

	t.Run("non-json files should be ignored", func(t *testing.T) {
		dir := t.TempDir()
		writeSessionFile(t, dir, Session{
			SessionID: "valid1",
			Project:   "/home/user/project",
			Status:    "idle",
		})
		if err := os.WriteFile(filepath.Join(dir, "readme.txt"), []byte("hello"), 0644); err != nil {
			t.Fatalf("write txt file: %v", err)
		}

		sessions, err := LoadAll(dir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(sessions) != 1 {
			t.Errorf("got %d sessions, want 1", len(sessions))
		}
	})

	t.Run("multiple session files should all be loaded", func(t *testing.T) {
		dir := t.TempDir()
		writeSessionFile(t, dir, Session{SessionID: "s1", Project: "/p1", Status: "working"})
		writeSessionFile(t, dir, Session{SessionID: "s2", Project: "/p2", Status: "idle"})
		writeSessionFile(t, dir, Session{SessionID: "s3", Project: "/p1", Status: "waiting"})

		sessions, err := LoadAll(dir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(sessions) != 3 {
			t.Errorf("got %d sessions, want 3", len(sessions))
		}
	})

	t.Run("missing terminals field should default to nil", func(t *testing.T) {
		dir := t.TempDir()
		data := []byte(`{"session_id":"old1","project":"/p","status":"idle"}`)
		if err := os.WriteFile(filepath.Join(dir, "old1.json"), data, 0644); err != nil {
			t.Fatalf("write file: %v", err)
		}

		sessions, err := LoadAll(dir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(sessions) != 1 {
			t.Fatalf("got %d sessions, want 1", len(sessions))
		}
		if sessions[0].Terminals != nil {
			t.Errorf("got terminals %v, want nil", sessions[0].Terminals)
		}
	})

	t.Run("session with PID field should parse correctly", func(t *testing.T) {
		dir := t.TempDir()
		writeSessionFile(t, dir, Session{
			SessionID: "pid1",
			Project:   "/p",
			Status:    "working",
			PID:       12345,
		})

		sessions, err := LoadAll(dir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(sessions) != 1 {
			t.Fatalf("got %d sessions, want 1", len(sessions))
		}
		if sessions[0].PID != 12345 {
			t.Errorf("got PID %d, want 12345", sessions[0].PID)
		}
	})
}

func TestGroupByProject(t *testing.T) {
	t.Run("empty input should return no groups", func(t *testing.T) {
		groups := GroupByProject(nil)
		if len(groups) != 0 {
			t.Errorf("got %d groups, want 0", len(groups))
		}
	})

	t.Run("sessions from different projects should be grouped and sorted alphabetically", func(t *testing.T) {
		sessions := []Session{
			{SessionID: "s1", Project: "/b-project", LastActivity: "2026-01-01T00:00:00Z"},
			{SessionID: "s2", Project: "/a-project", LastActivity: "2026-01-01T00:00:00Z"},
			{SessionID: "s3", Project: "/b-project", LastActivity: "2026-01-02T00:00:00Z"},
		}

		groups := GroupByProject(sessions)
		if len(groups) != 2 {
			t.Fatalf("got %d groups, want 2", len(groups))
		}
		if groups[0].Project != "/a-project" {
			t.Errorf("first group is %q, want %q", groups[0].Project, "/a-project")
		}
		if groups[1].Project != "/b-project" {
			t.Errorf("second group is %q, want %q", groups[1].Project, "/b-project")
		}
	})

	t.Run("sessions within a group should be sorted by session ID", func(t *testing.T) {
		sessions := []Session{
			{SessionID: "bbb", Project: "/proj", LastActivity: "2026-01-02T00:00:00Z"},
			{SessionID: "aaa", Project: "/proj", LastActivity: "2026-01-01T00:00:00Z"},
		}

		groups := GroupByProject(sessions)
		if len(groups) != 1 {
			t.Fatalf("got %d groups, want 1", len(groups))
		}
		if groups[0].Sessions[0].SessionID != "aaa" {
			t.Errorf("first session is %q, want %q (sorted by ID)", groups[0].Sessions[0].SessionID, "aaa")
		}
	})

}

func TestTimeSince(t *testing.T) {
	t.Run("unparseable timestamp should return ?", func(t *testing.T) {
		if got := TimeSince("not-a-timestamp"); got != "?" {
			t.Errorf("got %q, want %q", got, "?")
		}
	})

	t.Run("timestamp less than a second ago should format as now", func(t *testing.T) {
		ts := time.Now().Format(time.RFC3339)
		if got := TimeSince(ts); got != "now" {
			t.Errorf("got %q, want %q", got, "now")
		}
	})

	t.Run("timestamp under a minute should format as seconds", func(t *testing.T) {
		ts := time.Now().Add(-30 * time.Second).Format(time.RFC3339)
		got := TimeSince(ts)
		if got != "30s ago" && got != "29s ago" && got != "31s ago" {
			t.Errorf("got %q, want approximately 30s ago", got)
		}
	})

	t.Run("timestamp under an hour should format as minutes", func(t *testing.T) {
		ts := time.Now().Add(-5 * time.Minute).Format(time.RFC3339)
		got := TimeSince(ts)
		if got != "5m ago" {
			t.Errorf("got %q, want %q", got, "5m ago")
		}
	})

	t.Run("timestamp under a day should format as hours", func(t *testing.T) {
		ts := time.Now().Add(-3 * time.Hour).Format(time.RFC3339)
		got := TimeSince(ts)
		if got != "3h ago" {
			t.Errorf("got %q, want %q", got, "3h ago")
		}
	})

	t.Run("timestamp over 24 hours should format as days", func(t *testing.T) {
		ts := time.Now().Add(-48 * time.Hour).Format(time.RFC3339)
		got := TimeSince(ts)
		if got != "2d ago" {
			t.Errorf("got %q, want %q", got, "2d ago")
		}
	})
}
