package hook

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
	"unicode"

	ps "github.com/mitchellh/go-ps"

	"github.com/martinwickman/ccmonitor/internal/session"
	"github.com/martinwickman/ccmonitor/internal/wt"
)

// Hook event name constants.
const (
	EventSessionStart     = "SessionStart"
	EventSessionEnd       = "SessionEnd"
	EventUserPromptSubmit = "UserPromptSubmit"
	EventPreToolUse       = "PreToolUse"
	EventPostToolUse      = "PostToolUse"
	EventNotification     = "Notification"
	EventStop             = "Stop"
)

// Actionable notification types.
const (
	NotifPermissionPrompt  = "permission_prompt"
	NotifElicitationDialog = "elicitation_dialog"
)

type hookInput struct {
	SessionID        string          `json:"session_id"`
	CWD              string          `json:"cwd"`
	HookEventName    string          `json:"hook_event_name"`
	ToolName         string          `json:"tool_name"`
	ToolInput        json.RawMessage `json:"tool_input"`
	NotificationType string          `json:"notification_type"`
	Prompt           string          `json:"prompt"`
	Message          string          `json:"message"`
	Title            string          `json:"title"`
	Source           string          `json:"source"`
}

func mapEvent(event, toolDetail, notifType, title, message string) (status, detail string) {
	switch event {
	case EventSessionStart:
		return session.StatusStarting, "Session started"
	case EventUserPromptSubmit:
		return session.StatusWorking, "Processing prompt..."
	case EventPreToolUse:
		return session.StatusWorking, toolDetail
	case EventPostToolUse:
		return session.StatusWorking, toolDetail
	case EventNotification:
		return session.StatusWaiting, notificationDetail(notifType, title, message)
	case EventStop:
		return session.StatusIdle, "Finished responding"
	default:
		return "", ""
	}
}

func buildToolDetail(event, toolName string, toolInput json.RawMessage) string {
	if toolName == "" {
		return ""
	}

	if event == EventPostToolUse {
		return fmt.Sprintf("Finished %s, continuing...", toolName)
	}

	var input map[string]any
	if len(toolInput) > 0 {
		json.Unmarshal(toolInput, &input) // best-effort
	}

	getString := func(key string) string {
		if input == nil {
			return ""
		}
		v, ok := input[key]
		if !ok {
			return ""
		}
		s, ok := v.(string)
		if !ok {
			return ""
		}
		return s
	}

	switch toolName {
	case "Bash":
		cmd := getString("command")
		if len(cmd) > 80 {
			cmd = cmd[:80]
		}
		if cmd != "" {
			return "Bash: " + cmd
		}
		return "Bash"
	case "Edit", "Write", "Read":
		fp := getString("file_path")
		if fp != "" {
			return toolName + " " + filepath.Base(fp)
		}
		return toolName
	case "Glob":
		pattern := getString("pattern")
		if pattern != "" {
			return "Glob " + pattern
		}
		return "Glob"
	case "Grep":
		pattern := getString("pattern")
		if pattern != "" {
			return "Grep " + pattern
		}
		return "Grep"
	case "Task":
		desc := getString("description")
		if desc != "" {
			return "Task: " + desc
		}
		return "Task"
	default:
		return toolName
	}
}

func notificationDetail(notifType, title, message string) string {
	if title != "" {
		return title
	}
	if message != "" {
		return message
	}
	return "Awaiting response"
}

// stripTitlePrefix removes leading non-alphanumeric characters from a tab/pane
// title. Claude Code prefixes titles with status indicators like "✳ " but the
// exact character varies by platform and encoding.
func stripTitlePrefix(title string) string {
	i := strings.IndexFunc(title, func(r rune) bool {
		return unicode.IsLetter(r) || unicode.IsDigit(r)
	})
	if i > 0 {
		return title[i:]
	}
	return title
}

// termInfo holds terminal environment info collected once per hook invocation.
type termInfo struct {
	tmuxPane  string
	summary   string
	runtimeID string
}

func tmuxInfo() (pane, title string) {
	pane = os.Getenv("TMUX_PANE")
	if pane == "" {
		return "", ""
	}
	out, err := exec.Command("tmux", "display-message", "-p", "-t", pane, "#{pane_title}").Output()
	if err != nil {
		return pane, ""
	}
	title = strings.TrimSpace(string(out))
	title = stripTitlePrefix(title)
	return pane, title
}

// defaultTermInfo returns terminal info based on the current environment.
// Captures both tmux and WT info when both are available (tmux inside WT).
// WT is checked first, then tmux — when both are present, tmux title is
// preferred since it's more specific (inner pane vs outer tab).
func defaultTermInfo(hookEvent, sessionID, existingRuntimeID string) termInfo {
	var ti termInfo
	if os.Getenv("WT_SESSION") != "" {
		if hookEvent == EventSessionStart || existingRuntimeID == "" {
			ti.runtimeID, ti.summary = wt.TabInfo()
		} else {
			ti.summary = wt.TabTitle(existingRuntimeID)
		}
		ti.summary = stripTitlePrefix(ti.summary)
	}
	if os.Getenv("TMUX_PANE") != "" {
		ti.tmuxPane, ti.summary = tmuxInfo()
	}
	return ti
}

// loadExistingSession reads the existing session file and returns it.
// Returns a zero-value Session if the file doesn't exist or is corrupt.
func loadExistingSession(path string) session.Session {
	s, err := session.LoadFile(path)
	if err != nil {
		return session.Session{}
	}
	return *s
}

func writeSessionFile(path string, s session.Session) error {
	data, err := json.MarshalIndent(s, "", "    ")
	if err != nil {
		return fmt.Errorf("marshaling session: %w", err)
	}
	return os.WriteFile(path, data, 0644)
}

// findParentPID walks up from our parent to find the grandparent PID (Claude Code).
// Hooks are spawned as: Claude Code → shell (/bin/sh -c) → ccmonitor hook.
// Returns 0 if the process tree cannot be walked.
func findParentPID() int {
	ppid := os.Getppid()
	if ppid <= 0 {
		return 0
	}
	parent, err := ps.FindProcess(ppid)
	if err != nil || parent == nil {
		return 0
	}
	gpid := parent.PPid()
	if gpid <= 0 {
		return 0
	}
	return gpid
}

// cleanupSamePID removes session files that share a PID with the current session
// but have a different session ID. This handles the case where Claude Code starts
// a new session (e.g. via /clear) without firing SessionEnd for the old one.
// Only removes sessions from the same OS, since PIDs are only meaningful within
// the same OS (a Linux PID 1234 is unrelated to Windows PID 1234).
func cleanupSamePID(dir, currentSessionID string, currentPID int) {
	if currentPID <= 0 {
		return
	}
	session.ForEachSessionFile(dir, func(path string, s *session.Session) {
		if s.SessionID != currentSessionID && s.PID == currentPID &&
			(s.OS == "" || s.OS == runtime.GOOS) {
			os.Remove(path) // best-effort
		}
	})
}

// cleanupDead removes session files whose PID is no longer alive.
// Files with PID 0 (legacy or unknown) and corrupt files are skipped.
// Only checks sessions from the same OS, since go-ps can only see native PIDs
// (a WSL hook can't check Windows PIDs and vice versa).
func cleanupDead(dir string) error {
	return session.ForEachSessionFile(dir, func(path string, s *session.Session) {
		if s.PID <= 0 {
			return // no PID recorded, can't check
		}
		if s.OS != "" && s.OS != runtime.GOOS {
			return // different OS, can't check from here
		}
		proc, err := ps.FindProcess(s.PID)
		if err != nil {
			return // can't check, leave it
		}
		if proc == nil {
			os.Remove(path) // best-effort
		}
	})
}

// Run is the entry point called from main.go. It reads hook input from stdin.
func Run() error {
	return run(os.Stdin, defaultTermInfo, findParentPID)
}

func run(stdin io.Reader, termInfoFn func(string, string, string) termInfo, pidFn func() int) error {
	dir := session.Dir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating sessions dir: %w", err)
	}

	data, err := io.ReadAll(stdin)
	if err != nil {
		return fmt.Errorf("reading stdin: %w", err)
	}

	var input hookInput
	if err := json.Unmarshal(data, &input); err != nil {
		return fmt.Errorf("parsing hook input: %w", err)
	}

	sessionFile := filepath.Join(dir, input.SessionID+".json")

	// SessionEnd: cleanup dead sessions, delete own file, return
	if input.HookEventName == EventSessionEnd {
		cleanupDead(dir)
		os.Remove(sessionFile)
		return nil
	}

	// SessionStart: cleanup dead sessions
	if input.HookEventName == EventSessionStart {
		cleanupDead(dir)
	}

	// Skip non-actionable notifications (e.g. idle_prompt after ~60s inactivity).
	// The session file already has status "idle" from the prior Stop event.
	if input.HookEventName == EventNotification &&
		input.NotificationType != NotifPermissionPrompt &&
		input.NotificationType != NotifElicitationDialog {
		return nil
	}

	toolDetail := buildToolDetail(input.HookEventName, input.ToolName, input.ToolInput)
	status, detail := mapEvent(input.HookEventName, toolDetail, input.NotificationType, input.Title, input.Message)
	if status == "" {
		return nil // unknown event, no-op
	}

	// Read existing session for preserved fields (last_prompt, runtime_id)
	existing := loadExistingSession(sessionFile)

	// Resolve last_prompt
	var lastPrompt string
	if input.HookEventName == EventUserPromptSubmit {
		lastPrompt = input.Prompt
	} else {
		lastPrompt = existing.LastPrompt
	}

	// Get terminal info (tmux pane, WT runtime ID, and/or tab title)
	ti := termInfoFn(input.HookEventName, input.SessionID, existing.RuntimeID)

	// Preserve RuntimeID and summary from existing session on non-SessionStart events
	runtimeID := ti.runtimeID
	if runtimeID == "" && input.HookEventName != EventSessionStart {
		runtimeID = existing.RuntimeID
	}
	summary := ti.summary
	if summary == "" && input.HookEventName != EventSessionStart {
		summary = existing.Summary
	}

	// Build notification type pointer
	var notifType *string
	if input.NotificationType != "" {
		notifType = &input.NotificationType
	}

	// Capture PID: use pidFn on SessionStart, preserve from existing otherwise
	pid := pidFn()
	if pid == 0 && input.HookEventName != EventSessionStart {
		pid = existing.PID
	}

	s := session.Session{
		SessionID:        input.SessionID,
		Project:          input.CWD,
		Status:           status,
		Detail:           detail,
		LastPrompt:       lastPrompt,
		NotificationType: notifType,
		LastActivity:     time.Now().UTC().Format(time.RFC3339),
		TmuxPane:         ti.tmuxPane,
		Summary:          summary,
		RuntimeID:        runtimeID,
		PID:              pid,
		OS:               runtime.GOOS,
	}

	// Remove stale session files from the same PID (handles --continue/--resume
	// where SessionStart fires with a new ID but events continue under the old ID)
	cleanupSamePID(dir, input.SessionID, pid)

	return writeSessionFile(sessionFile, s)
}
