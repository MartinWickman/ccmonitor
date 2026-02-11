package monitor

import (
	"os"
	"testing"
	"time"

	"github.com/martinwickman/ccmonitor/internal/session"
)

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
		got := buildClickMap(nil, "some view\ncontent\n", false)
		if len(got) != 0 {
			t.Errorf("got %d entries, want 0", len(got))
		}
	})

	t.Run("connector line should be mapped to session", func(t *testing.T) {
		sessions := []session.Session{
			{SessionID: "abcd1234-full-id", Project: "/p"},
		}
		view := "header\nsummary\n├─ Fix the bug\n   Working  Edit main.go\n"
		got := buildClickMap(sessions, view, false)
		if got[2] != "abcd1234-full-id" {
			t.Errorf("line 2: got %q, want %q", got[2], "abcd1234-full-id")
		}
		// Status line below should also be mapped
		if got[3] != "abcd1234-full-id" {
			t.Errorf("line 3: got %q, want %q", got[3], "abcd1234-full-id")
		}
	})

	t.Run("last connector maps correctly", func(t *testing.T) {
		sessions := []session.Session{
			{SessionID: "abcd1234-full-id", Project: "/p"},
		}
		view := "header\n└─ Fix the bug\n   Working  Edit main.go\nfooter\n"
		got := buildClickMap(sessions, view, false)
		if got[1] != "abcd1234-full-id" {
			t.Errorf("line 1: got %q, want %q", got[1], "abcd1234-full-id")
		}
		if got[2] != "abcd1234-full-id" {
			t.Errorf("line 2: got %q, want %q", got[2], "abcd1234-full-id")
		}
	})

	t.Run("consecutive session rows should not bleed into each other", func(t *testing.T) {
		sessions := []session.Session{
			{SessionID: "aaaaaaaa-1111", Project: "/p"},
			{SessionID: "bbbbbbbb-2222", Project: "/p"},
		}
		view := "header\n├─ First task\n│  Working\n└─ Second task\n   Idle\nfooter\n"
		got := buildClickMap(sessions, view, false)
		if got[1] != "aaaaaaaa-1111" {
			t.Errorf("line 1: got %q, want %q", got[1], "aaaaaaaa-1111")
		}
		if got[2] != "aaaaaaaa-1111" {
			t.Errorf("line 2: got %q, want %q", got[2], "aaaaaaaa-1111")
		}
		if got[3] != "bbbbbbbb-2222" {
			t.Errorf("line 3: got %q, want %q", got[3], "bbbbbbbb-2222")
		}
		if got[4] != "bbbbbbbb-2222" {
			t.Errorf("line 4: got %q, want %q", got[4], "bbbbbbbb-2222")
		}
	})

	t.Run("lines without connectors should not be mapped", func(t *testing.T) {
		sessions := []session.Session{
			{SessionID: "abcd1234-full-id", Project: "/p"},
		}
		view := "header line\nproject title\n├─ Fix the bug\n   Working\n"
		got := buildClickMap(sessions, view, false)
		if _, ok := got[0]; ok {
			t.Errorf("header line should not be mapped")
		}
		if _, ok := got[1]; ok {
			t.Errorf("project title should not be mapped")
		}
	})
}

func TestCheckPIDLiveness(t *testing.T) {
	t.Run("dead PID sets status to exited", func(t *testing.T) {
		sessions := []session.Session{
			{SessionID: "s1", Status: "working", PID: 99999999},
		}
		CheckPIDLiveness(sessions)
		if sessions[0].Status != "exited" {
			t.Errorf("status = %q, want %q", sessions[0].Status, "exited")
		}
		if sessions[0].Detail != "Process ended" {
			t.Errorf("detail = %q, want %q", sessions[0].Detail, "Process ended")
		}
	})

	t.Run("alive PID keeps original status", func(t *testing.T) {
		sessions := []session.Session{
			{SessionID: "s2", Status: "working", PID: os.Getpid()},
		}
		CheckPIDLiveness(sessions)
		if sessions[0].Status != "working" {
			t.Errorf("status = %q, want %q", sessions[0].Status, "working")
		}
	})

	t.Run("zero PID is left as-is", func(t *testing.T) {
		sessions := []session.Session{
			{SessionID: "s3", Status: "idle", PID: 0},
		}
		CheckPIDLiveness(sessions)
		if sessions[0].Status != "idle" {
			t.Errorf("status = %q, want %q", sessions[0].Status, "idle")
		}
	})
}
