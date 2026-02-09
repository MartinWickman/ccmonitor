package hook

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/martinwickman/ccmonitor/internal/session"
)

func TestMapEvent(t *testing.T) {
	tests := []struct {
		name       string
		event      string
		toolDetail string
		notifType  string
		title      string
		message    string
		wantStatus string
		wantDetail string
	}{
		{"SessionStart", "SessionStart", "", "", "", "", "starting", "Session started"},
		{"UserPromptSubmit", "UserPromptSubmit", "", "", "", "", "working", "Processing prompt..."},
		{"PreToolUse", "PreToolUse", "Edit main.go", "", "", "", "working", "Edit main.go"},
		{"PostToolUse", "PostToolUse", "Finished Bash, continuing...", "", "", "", "working", "Finished Bash, continuing..."},
		{"Notification with title", "Notification", "", "permission_prompt", "Allow Edit?", "", "waiting", "Allow Edit?"},
		{"Notification with message only", "Notification", "", "permission_prompt", "", "Claude wants to edit", "waiting", "Claude wants to edit"},
		{"Notification no title or message", "Notification", "", "permission_prompt", "", "", "waiting", "Awaiting response"},
		{"Notification elicitation_dialog", "Notification", "", "elicitation_dialog", "Pick an option", "", "waiting", "Pick an option"},
		{"Stop", "Stop", "", "", "", "", "idle", "Finished responding"},
		{"UnknownEvent", "UnknownEvent", "", "", "", "", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, detail := mapEvent(tt.event, tt.toolDetail, tt.notifType, tt.title, tt.message)
			if status != tt.wantStatus {
				t.Errorf("status = %q, want %q", status, tt.wantStatus)
			}
			if detail != tt.wantDetail {
				t.Errorf("detail = %q, want %q", detail, tt.wantDetail)
			}
		})
	}
}

func TestBuildToolDetail(t *testing.T) {
	tests := []struct {
		name     string
		event    string
		toolName string
		input    any
		want     string
	}{
		{
			name: "empty tool name",
			event: "PreToolUse", toolName: "", input: nil,
			want: "",
		},
		{
			name: "PostToolUse returns finished message",
			event: "PostToolUse", toolName: "Bash", input: nil,
			want: "Finished Bash, continuing...",
		},
		{
			name: "Bash with command",
			event: "PreToolUse", toolName: "Bash",
			input: map[string]any{"command": "npm test"},
			want:  "Bash: npm test",
		},
		{
			name: "Bash command truncated at 80 chars",
			event: "PreToolUse", toolName: "Bash",
			input: map[string]any{"command": strings.Repeat("x", 100)},
			want:  "Bash: " + strings.Repeat("x", 80),
		},
		{
			name: "Bash without command",
			event: "PreToolUse", toolName: "Bash",
			input: map[string]any{},
			want:  "Bash",
		},
		{
			name: "Edit with file_path",
			event: "PreToolUse", toolName: "Edit",
			input: map[string]any{"file_path": "/home/user/project/src/main.go"},
			want:  "Edit main.go",
		},
		{
			name: "Read with file_path",
			event: "PreToolUse", toolName: "Read",
			input: map[string]any{"file_path": "/tmp/foo.txt"},
			want:  "Read foo.txt",
		},
		{
			name: "Write without file_path",
			event: "PreToolUse", toolName: "Write",
			input: map[string]any{},
			want:  "Write",
		},
		{
			name: "Glob with pattern",
			event: "PreToolUse", toolName: "Glob",
			input: map[string]any{"pattern": "**/*.go"},
			want:  "Glob **/*.go",
		},
		{
			name: "Grep with pattern",
			event: "PreToolUse", toolName: "Grep",
			input: map[string]any{"pattern": "func main"},
			want:  "Grep func main",
		},
		{
			name: "Task with description",
			event: "PreToolUse", toolName: "Task",
			input: map[string]any{"description": "search for errors"},
			want:  "Task: search for errors",
		},
		{
			name: "WebFetch returns tool name only",
			event: "PreToolUse", toolName: "WebFetch",
			input: map[string]any{"url": "https://example.com"},
			want:  "WebFetch",
		},
		{
			name: "unknown tool returns tool name",
			event: "PreToolUse", toolName: "CustomTool",
			input: nil,
			want:  "CustomTool",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var raw json.RawMessage
			if tt.input != nil {
				raw, _ = json.Marshal(tt.input)
			}
			got := buildToolDetail(tt.event, tt.toolName, raw)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNotificationDetail(t *testing.T) {
	tests := []struct {
		name      string
		notifType string
		title     string
		message   string
		want      string
	}{
		{"title takes precedence", "permission_prompt", "Allow Edit?", "some message", "Allow Edit?"},
		{"message used when no title", "permission_prompt", "", "Claude wants to edit", "Claude wants to edit"},
		{"fallback when no title or message", "permission_prompt", "", "", "Awaiting response"},
		{"elicitation with title", "elicitation_dialog", "Pick option", "", "Pick option"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := notificationDetail(tt.notifType, tt.title, tt.message); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestReadExistingSession(t *testing.T) {
	t.Run("existing file with last_prompt and runtime_id", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "test.json")
		s := session.Session{
			SessionID:  "test1",
			LastPrompt: "do the thing",
			RuntimeID:  "42,123,4,5",
		}
		data, _ := json.Marshal(s)
		os.WriteFile(path, data, 0644)

		got := readExistingSession(path)
		if got.LastPrompt != "do the thing" {
			t.Errorf("last_prompt = %q, want %q", got.LastPrompt, "do the thing")
		}
		if got.RuntimeID != "42,123,4,5" {
			t.Errorf("runtime_id = %q, want %q", got.RuntimeID, "42,123,4,5")
		}
	})

	t.Run("missing file returns zero session", func(t *testing.T) {
		got := readExistingSession("/nonexistent/file.json")
		if got.LastPrompt != "" {
			t.Errorf("last_prompt = %q, want empty", got.LastPrompt)
		}
		if got.RuntimeID != "" {
			t.Errorf("runtime_id = %q, want empty", got.RuntimeID)
		}
	})

	t.Run("corrupt JSON returns zero session", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "bad.json")
		os.WriteFile(path, []byte("{bad"), 0644)

		got := readExistingSession(path)
		if got.LastPrompt != "" {
			t.Errorf("last_prompt = %q, want empty", got.LastPrompt)
		}
	})
}

func TestRun(t *testing.T) {
	stubTermInfo := func(string, string) termInfo { return termInfo{} }
	stubPidFn := func() int { return 42 }

	t.Run("PreToolUse writes session file", func(t *testing.T) {
		dir := t.TempDir()
		t.Setenv("CCMONITOR_SESSIONS_DIR", dir)

		input := `{"session_id":"s1","cwd":"/tmp/proj","hook_event_name":"PreToolUse","tool_name":"Bash","tool_input":{"command":"ls -la"}}`
		err := run(strings.NewReader(input), stubTermInfo, stubPidFn)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		data, err := os.ReadFile(filepath.Join(dir, "s1.json"))
		if err != nil {
			t.Fatalf("reading session file: %v", err)
		}
		var s session.Session
		if err := json.Unmarshal(data, &s); err != nil {
			t.Fatalf("parsing session file: %v", err)
		}
		if s.Status != "working" {
			t.Errorf("status = %q, want %q", s.Status, "working")
		}
		if s.Detail != "Bash: ls -la" {
			t.Errorf("detail = %q, want %q", s.Detail, "Bash: ls -la")
		}
		if s.Project != "/tmp/proj" {
			t.Errorf("project = %q, want %q", s.Project, "/tmp/proj")
		}
		if s.NotificationType != nil {
			t.Errorf("notification_type = %v, want nil", s.NotificationType)
		}
	})

	t.Run("UserPromptSubmit captures prompt", func(t *testing.T) {
		dir := t.TempDir()
		t.Setenv("CCMONITOR_SESSIONS_DIR", dir)

		input := `{"session_id":"s2","cwd":"/tmp","hook_event_name":"UserPromptSubmit","prompt":"fix the bug"}`
		err := run(strings.NewReader(input), stubTermInfo, stubPidFn)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		data, _ := os.ReadFile(filepath.Join(dir, "s2.json"))
		var s session.Session
		json.Unmarshal(data, &s)
		if s.LastPrompt != "fix the bug" {
			t.Errorf("last_prompt = %q, want %q", s.LastPrompt, "fix the bug")
		}
		if s.Status != "working" {
			t.Errorf("status = %q, want %q", s.Status, "working")
		}
	})

	t.Run("subsequent event preserves last_prompt from existing file", func(t *testing.T) {
		dir := t.TempDir()
		t.Setenv("CCMONITOR_SESSIONS_DIR", dir)

		// First: write a session file with a prompt
		existing := session.Session{
			SessionID:  "s3",
			LastPrompt: "original prompt",
		}
		data, _ := json.Marshal(existing)
		os.WriteFile(filepath.Join(dir, "s3.json"), data, 0644)

		// Then: send a Stop event (should preserve last_prompt)
		input := `{"session_id":"s3","cwd":"/tmp","hook_event_name":"Stop"}`
		err := run(strings.NewReader(input), stubTermInfo, stubPidFn)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		data, _ = os.ReadFile(filepath.Join(dir, "s3.json"))
		var s session.Session
		json.Unmarshal(data, &s)
		if s.LastPrompt != "original prompt" {
			t.Errorf("last_prompt = %q, want %q", s.LastPrompt, "original prompt")
		}
	})

	t.Run("SessionEnd deletes session file", func(t *testing.T) {
		dir := t.TempDir()
		t.Setenv("CCMONITOR_SESSIONS_DIR", dir)

		// Create existing session file
		os.WriteFile(filepath.Join(dir, "s4.json"), []byte(`{"session_id":"s4"}`), 0644)

		input := `{"session_id":"s4","cwd":"/tmp","hook_event_name":"SessionEnd"}`
		err := run(strings.NewReader(input), stubTermInfo, stubPidFn)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if _, err := os.Stat(filepath.Join(dir, "s4.json")); !os.IsNotExist(err) {
			t.Error("session file should have been deleted")
		}
	})

	t.Run("SessionStart cleans up dead PID files", func(t *testing.T) {
		dir := t.TempDir()
		t.Setenv("CCMONITOR_SESSIONS_DIR", dir)

		// Create a session file with a dead PID (99999999 should not exist)
		dead := session.Session{
			SessionID: "dead1",
			Project:   "/p",
			Status:    "idle",
			PID:       99999999,
		}
		data, _ := json.Marshal(dead)
		os.WriteFile(filepath.Join(dir, "dead1.json"), data, 0644)

		input := `{"session_id":"s5","cwd":"/tmp","hook_event_name":"SessionStart"}`
		err := run(strings.NewReader(input), stubTermInfo, stubPidFn)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Dead PID file should be cleaned up
		if _, err := os.Stat(filepath.Join(dir, "dead1.json")); !os.IsNotExist(err) {
			t.Error("dead PID session file should have been deleted")
		}
		// New session file should exist
		if _, err := os.Stat(filepath.Join(dir, "s5.json")); err != nil {
			t.Error("new session file should have been created")
		}
	})

	t.Run("Notification sets notification_type", func(t *testing.T) {
		dir := t.TempDir()
		t.Setenv("CCMONITOR_SESSIONS_DIR", dir)

		input := `{"session_id":"s6","cwd":"/tmp","hook_event_name":"Notification","notification_type":"permission_prompt"}`
		err := run(strings.NewReader(input), stubTermInfo, stubPidFn)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		data, _ := os.ReadFile(filepath.Join(dir, "s6.json"))
		var s session.Session
		json.Unmarshal(data, &s)
		if s.NotificationType == nil {
			t.Fatal("notification_type should not be nil")
		}
		if *s.NotificationType != "permission_prompt" {
			t.Errorf("notification_type = %q, want %q", *s.NotificationType, "permission_prompt")
		}
	})

	t.Run("idle_prompt notification is skipped", func(t *testing.T) {
		dir := t.TempDir()
		t.Setenv("CCMONITOR_SESSIONS_DIR", dir)

		// Create an existing session file with idle status
		existing := session.Session{
			SessionID:    "s-idle",
			Project:      "/tmp",
			Status:       "idle",
			Detail:       "Finished responding",
			LastActivity: time.Now().UTC().Format(time.RFC3339),
		}
		data, _ := json.Marshal(existing)
		os.WriteFile(filepath.Join(dir, "s-idle.json"), data, 0644)

		// Send idle_prompt notification — should be a no-op
		input := `{"session_id":"s-idle","cwd":"/tmp","hook_event_name":"Notification","notification_type":"idle_prompt"}`
		err := run(strings.NewReader(input), stubTermInfo, stubPidFn)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Session file should still have idle status (unchanged)
		data, _ = os.ReadFile(filepath.Join(dir, "s-idle.json"))
		var s session.Session
		json.Unmarshal(data, &s)
		if s.Status != "idle" {
			t.Errorf("status = %q, want %q (idle_prompt should not change status)", s.Status, "idle")
		}
	})

	t.Run("permission_prompt notification captures title", func(t *testing.T) {
		dir := t.TempDir()
		t.Setenv("CCMONITOR_SESSIONS_DIR", dir)

		input := `{"session_id":"s-perm","cwd":"/tmp","hook_event_name":"Notification","notification_type":"permission_prompt","title":"Allow Bash?","message":"Claude wants to run: rm -rf /tmp/foo"}`
		err := run(strings.NewReader(input), stubTermInfo, stubPidFn)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		data, _ := os.ReadFile(filepath.Join(dir, "s-perm.json"))
		var s session.Session
		json.Unmarshal(data, &s)
		if s.Status != "waiting" {
			t.Errorf("status = %q, want %q", s.Status, "waiting")
		}
		if s.Detail != "Allow Bash?" {
			t.Errorf("detail = %q, want %q", s.Detail, "Allow Bash?")
		}
	})

	t.Run("unknown event is a no-op", func(t *testing.T) {
		dir := t.TempDir()
		t.Setenv("CCMONITOR_SESSIONS_DIR", dir)

		input := `{"session_id":"s7","cwd":"/tmp","hook_event_name":"SomeFutureEvent"}`
		err := run(strings.NewReader(input), stubTermInfo, stubPidFn)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if _, err := os.Stat(filepath.Join(dir, "s7.json")); !os.IsNotExist(err) {
			t.Error("no session file should be created for unknown events")
		}
	})

	t.Run("tmux info is captured", func(t *testing.T) {
		dir := t.TempDir()
		t.Setenv("CCMONITOR_SESSIONS_DIR", dir)

		tmuxFn := func(string, string) termInfo {
			return termInfo{tmuxPane: "%5", summary: "my project"}
		}
		input := `{"session_id":"s8","cwd":"/tmp","hook_event_name":"Stop"}`
		err := run(strings.NewReader(input), tmuxFn, stubPidFn)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		data, _ := os.ReadFile(filepath.Join(dir, "s8.json"))
		var s session.Session
		json.Unmarshal(data, &s)
		if s.TmuxPane != "%5" {
			t.Errorf("tmux_pane = %q, want %q", s.TmuxPane, "%5")
		}
		if s.Summary != "my project" {
			t.Errorf("summary = %q, want %q", s.Summary, "my project")
		}
	})

	t.Run("RuntimeID captured on SessionStart", func(t *testing.T) {
		dir := t.TempDir()
		t.Setenv("CCMONITOR_SESSIONS_DIR", dir)

		wtFn := func(event, sid string) termInfo {
			if event == "SessionStart" {
				return termInfo{runtimeID: "42,17436612,4,279"}
			}
			return termInfo{}
		}
		input := `{"session_id":"s-wt","cwd":"/tmp","hook_event_name":"SessionStart"}`
		err := run(strings.NewReader(input), wtFn, stubPidFn)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		data, _ := os.ReadFile(filepath.Join(dir, "s-wt.json"))
		var s session.Session
		json.Unmarshal(data, &s)
		if s.RuntimeID != "42,17436612,4,279" {
			t.Errorf("runtime_id = %q, want %q", s.RuntimeID, "42,17436612,4,279")
		}
		if s.Status != "starting" {
			t.Errorf("status = %q, want %q", s.Status, "starting")
		}
	})

	t.Run("RuntimeID preserved from existing file on subsequent events", func(t *testing.T) {
		dir := t.TempDir()
		t.Setenv("CCMONITOR_SESSIONS_DIR", dir)

		// Write existing session with RuntimeID
		existing := session.Session{
			SessionID:  "s-wt2",
			RuntimeID:  "42,17436612,4,279",
			LastPrompt: "do stuff",
		}
		data, _ := json.Marshal(existing)
		os.WriteFile(filepath.Join(dir, "s-wt2.json"), data, 0644)

		// Send a Stop event — RuntimeID should be preserved
		input := `{"session_id":"s-wt2","cwd":"/tmp","hook_event_name":"Stop"}`
		err := run(strings.NewReader(input), stubTermInfo, stubPidFn)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		data, _ = os.ReadFile(filepath.Join(dir, "s-wt2.json"))
		var s session.Session
		json.Unmarshal(data, &s)
		if s.RuntimeID != "42,17436612,4,279" {
			t.Errorf("runtime_id = %q, want %q", s.RuntimeID, "42,17436612,4,279")
		}
		if s.LastPrompt != "do stuff" {
			t.Errorf("last_prompt = %q, want %q", s.LastPrompt, "do stuff")
		}
	})

	t.Run("PID is captured in session file", func(t *testing.T) {
		dir := t.TempDir()
		t.Setenv("CCMONITOR_SESSIONS_DIR", dir)

		customPidFn := func() int { return 12345 }
		input := `{"session_id":"s-pid","cwd":"/tmp","hook_event_name":"SessionStart"}`
		err := run(strings.NewReader(input), stubTermInfo, customPidFn)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		data, _ := os.ReadFile(filepath.Join(dir, "s-pid.json"))
		var s session.Session
		json.Unmarshal(data, &s)
		if s.PID != 12345 {
			t.Errorf("pid = %d, want 12345", s.PID)
		}
	})

	t.Run("PID is preserved from existing file on subsequent events", func(t *testing.T) {
		dir := t.TempDir()
		t.Setenv("CCMONITOR_SESSIONS_DIR", dir)

		// Write existing session with PID
		existing := session.Session{
			SessionID: "s-pid2",
			PID:       54321,
		}
		data, _ := json.Marshal(existing)
		os.WriteFile(filepath.Join(dir, "s-pid2.json"), data, 0644)

		// pidFn returns 0 (simulating inability to walk process tree)
		zeroPidFn := func() int { return 0 }
		input := `{"session_id":"s-pid2","cwd":"/tmp","hook_event_name":"Stop"}`
		err := run(strings.NewReader(input), stubTermInfo, zeroPidFn)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		data, _ = os.ReadFile(filepath.Join(dir, "s-pid2.json"))
		var s session.Session
		json.Unmarshal(data, &s)
		if s.PID != 54321 {
			t.Errorf("pid = %d, want 54321 (should be preserved)", s.PID)
		}
	})
}

func TestFindParentPID(t *testing.T) {
	pid := findParentPID()
	if pid <= 0 {
		t.Errorf("findParentPID() = %d, want > 0", pid)
	}
}

func TestCleanupDead(t *testing.T) {
	t.Run("removes files with dead PIDs", func(t *testing.T) {
		dir := t.TempDir()
		dead := session.Session{
			SessionID: "dead1",
			Project:   "/p",
			Status:    "working",
			PID:       99999999,
		}
		data, _ := json.Marshal(dead)
		os.WriteFile(filepath.Join(dir, "dead1.json"), data, 0644)

		if err := cleanupDead(dir); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if _, err := os.Stat(filepath.Join(dir, "dead1.json")); !os.IsNotExist(err) {
			t.Error("dead PID session file should have been removed")
		}
	})

	t.Run("keeps files with alive PIDs", func(t *testing.T) {
		dir := t.TempDir()
		alive := session.Session{
			SessionID: "alive1",
			Project:   "/p",
			Status:    "working",
			PID:       os.Getpid(),
		}
		data, _ := json.Marshal(alive)
		os.WriteFile(filepath.Join(dir, "alive1.json"), data, 0644)

		if err := cleanupDead(dir); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if _, err := os.Stat(filepath.Join(dir, "alive1.json")); err != nil {
			t.Error("alive PID session file should have been kept")
		}
	})

	t.Run("keeps files with zero PID", func(t *testing.T) {
		dir := t.TempDir()
		noPid := session.Session{
			SessionID: "nopid1",
			Project:   "/p",
			Status:    "working",
			PID:       0,
		}
		data, _ := json.Marshal(noPid)
		os.WriteFile(filepath.Join(dir, "nopid1.json"), data, 0644)

		if err := cleanupDead(dir); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if _, err := os.Stat(filepath.Join(dir, "nopid1.json")); err != nil {
			t.Error("zero PID session file should have been kept")
		}
	})

	t.Run("skips corrupt files", func(t *testing.T) {
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, "corrupt.json"), []byte("{bad"), 0644)

		if err := cleanupDead(dir); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		entries, _ := os.ReadDir(dir)
		if len(entries) != 1 {
			t.Errorf("got %d files, want 1 (corrupt file should remain)", len(entries))
		}
	})

	t.Run("nonexistent directory returns nil", func(t *testing.T) {
		if err := cleanupDead("/nonexistent/path"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}
