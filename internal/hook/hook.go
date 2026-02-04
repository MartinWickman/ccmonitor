package hook

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/martinwickman/ccmonitor/internal/session"
)

type hookInput struct {
	SessionID        string          `json:"session_id"`
	CWD              string          `json:"cwd"`
	HookEventName    string          `json:"hook_event_name"`
	ToolName         string          `json:"tool_name"`
	ToolInput        json.RawMessage `json:"tool_input"`
	NotificationType string          `json:"notification_type"`
	Prompt           string          `json:"prompt"`
}

func mapEvent(event, toolDetail, notifType string) (status, detail string) {
	switch event {
	case "SessionStart":
		return "starting", "Session started"
	case "UserPromptSubmit":
		return "working", "Processing prompt..."
	case "PreToolUse":
		return "working", toolDetail
	case "PostToolUse":
		return "working", toolDetail
	case "Notification":
		return "waiting", notificationDetail(notifType)
	case "Stop":
		return "idle", "Finished responding"
	default:
		return "", ""
	}
}

func buildToolDetail(event, toolName string, toolInput json.RawMessage) string {
	if toolName == "" {
		return ""
	}

	if event == "PostToolUse" {
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

func notificationDetail(notifType string) string {
	switch notifType {
	case "idle_prompt":
		return "Waiting for input"
	case "permission_prompt":
		return "Awaiting response"
	default:
		return notifType
	}
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
	title = strings.TrimPrefix(title, "✳ ")
	return pane, title
}

func readExistingPrompt(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	var s session.Session
	if err := json.Unmarshal(data, &s); err != nil {
		return ""
	}
	return s.LastPrompt
}

func writeSessionFile(path string, s session.Session) error {
	data, err := json.MarshalIndent(s, "", "    ")
	if err != nil {
		return fmt.Errorf("marshaling session: %w", err)
	}
	return os.WriteFile(path, data, 0644)
}

// Run is the entry point called from main.go. It reads hook input from stdin.
func Run() error {
	return run(os.Stdin, tmuxInfo)
}

func run(stdin io.Reader, tmuxFn func() (string, string)) error {
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

	// SessionEnd: cleanup stale files, delete own file, return
	if input.HookEventName == "SessionEnd" {
		session.CleanupStale(dir)
		os.Remove(sessionFile)
		return nil
	}

	// SessionStart: cleanup stale files
	if input.HookEventName == "SessionStart" {
		session.CleanupStale(dir)
	}

	toolDetail := buildToolDetail(input.HookEventName, input.ToolName, input.ToolInput)
	status, detail := mapEvent(input.HookEventName, toolDetail, input.NotificationType)
	if status == "" {
		return nil // unknown event, no-op
	}

	// Resolve last_prompt
	var lastPrompt string
	if input.HookEventName == "UserPromptSubmit" {
		lastPrompt = input.Prompt
	} else {
		lastPrompt = readExistingPrompt(sessionFile)
	}

	// Get tmux info
	pane, title := tmuxFn()

	// Build notification type pointer
	var notifType *string
	if input.NotificationType != "" {
		notifType = &input.NotificationType
	}

	s := session.Session{
		SessionID:        input.SessionID,
		Project:          input.CWD,
		Status:           status,
		Detail:           detail,
		LastPrompt:       lastPrompt,
		NotificationType: notifType,
		LastActivity:     time.Now().UTC().Format(time.RFC3339),
		TmuxPane:         pane,
		Summary:          title,
	}

	return writeSessionFile(sessionFile, s)
}
