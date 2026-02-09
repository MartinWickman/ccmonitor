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

	ps "github.com/mitchellh/go-ps"

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
	Message          string          `json:"message"`
	Title            string          `json:"title"`
}

func mapEvent(event, toolDetail, notifType, title, message string) (status, detail string) {
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
		return "waiting", notificationDetail(notifType, title, message)
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

func notificationDetail(notifType, title, message string) string {
	if title != "" {
		return title
	}
	if message != "" {
		return message
	}
	return "Awaiting response"
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
	title = strings.TrimPrefix(title, "✳ ")
	return pane, title
}

// wtRuntimeID uses PowerShell and UI Automation to find the currently selected
// tab in the foreground Windows Terminal window. Only called on SessionStart,
// so the active tab is the one where Claude Code just started.
func wtRuntimeID() string {
	script := `
Add-Type -AssemblyName UIAutomationClient
Add-Type -AssemblyName UIAutomationTypes
Add-Type -TypeDefinition @"
using System;
using System.Runtime.InteropServices;
public class WinAPI {
    [DllImport("user32.dll")]
    public static extern IntPtr GetForegroundWindow();
}
"@
$fgHwnd = [WinAPI]::GetForegroundWindow()
$root = [System.Windows.Automation.AutomationElement]::RootElement
$wtCond = New-Object System.Windows.Automation.PropertyCondition([System.Windows.Automation.AutomationElement]::ClassNameProperty, 'CASCADIA_HOSTING_WINDOW_CLASS')
$wtWindows = $root.FindAll([System.Windows.Automation.TreeScope]::Children, $wtCond)
foreach ($w in $wtWindows) {
    if ($w.Current.NativeWindowHandle -ne [int]$fgHwnd) { continue }
    $tabCond = New-Object System.Windows.Automation.PropertyCondition([System.Windows.Automation.AutomationElement]::ControlTypeProperty, [System.Windows.Automation.ControlType]::TabItem)
    $tabs = $w.FindAll([System.Windows.Automation.TreeScope]::Descendants, $tabCond)
    foreach ($tab in $tabs) {
        try {
            $sel = $tab.GetCurrentPattern([System.Windows.Automation.SelectionItemPattern]::Pattern)
            if ($sel.Current.IsSelected) {
                $rid = $tab.GetRuntimeId()
                ($rid -join ',')
                exit
            }
        } catch {}
    }
}
`

	out, err := exec.Command("powershell.exe", "-NoProfile", "-Command", script).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// defaultTermInfo returns terminal info based on the current environment.
// Captures both tmux and WT info when both are available (tmux inside WT).
func defaultTermInfo(hookEvent, sessionID string) termInfo {
	var ti termInfo
	if os.Getenv("TMUX_PANE") != "" {
		ti.tmuxPane, ti.summary = tmuxInfo()
	}
	if os.Getenv("WT_SESSION") != "" && hookEvent == "SessionStart" {
		ti.runtimeID = wtRuntimeID()
	}
	return ti
}

// readExistingSession reads the existing session file and returns it.
// Returns a zero-value Session if the file doesn't exist or is corrupt.
func readExistingSession(path string) session.Session {
	data, err := os.ReadFile(path)
	if err != nil {
		return session.Session{}
	}
	var s session.Session
	if err := json.Unmarshal(data, &s); err != nil {
		return session.Session{}
	}
	return s
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

// cleanupDead removes session files whose PID is no longer alive.
// Files with PID 0 (legacy or unknown) and corrupt files are skipped.
func cleanupDead(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("reading sessions dir: %w", err)
	}

	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		path := filepath.Join(dir, e.Name())
		s, err := session.LoadFile(path)
		if err != nil {
			continue // skip corrupt files
		}
		if s.PID <= 0 {
			continue // no PID recorded, can't check
		}
		proc, err := ps.FindProcess(s.PID)
		if err != nil {
			continue // can't check, leave it
		}
		if proc == nil {
			// Process is dead, remove the session file
			os.Remove(path)
		}
	}
	return nil
}

// Run is the entry point called from main.go. It reads hook input from stdin.
func Run() error {
	return run(os.Stdin, defaultTermInfo, findParentPID)
}

func run(stdin io.Reader, termInfoFn func(string, string) termInfo, pidFn func() int) error {
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
	if input.HookEventName == "SessionEnd" {
		cleanupDead(dir)
		os.Remove(sessionFile)
		return nil
	}

	// SessionStart: cleanup dead sessions
	if input.HookEventName == "SessionStart" {
		cleanupDead(dir)
	}

	// Skip non-actionable notifications (e.g. idle_prompt after ~60s inactivity).
	// The session file already has status "idle" from the prior Stop event.
	if input.HookEventName == "Notification" &&
		input.NotificationType != "permission_prompt" &&
		input.NotificationType != "elicitation_dialog" {
		return nil
	}

	toolDetail := buildToolDetail(input.HookEventName, input.ToolName, input.ToolInput)
	status, detail := mapEvent(input.HookEventName, toolDetail, input.NotificationType, input.Title, input.Message)
	if status == "" {
		return nil // unknown event, no-op
	}

	// Read existing session for preserved fields (last_prompt, runtime_id)
	existing := readExistingSession(sessionFile)

	// Resolve last_prompt
	var lastPrompt string
	if input.HookEventName == "UserPromptSubmit" {
		lastPrompt = input.Prompt
	} else {
		lastPrompt = existing.LastPrompt
	}

	// Get terminal info (tmux pane or WT runtime ID)
	ti := termInfoFn(input.HookEventName, input.SessionID)

	// Preserve RuntimeID from existing session on non-SessionStart events
	runtimeID := ti.runtimeID
	if runtimeID == "" && input.HookEventName != "SessionStart" {
		runtimeID = existing.RuntimeID
	}

	// Build notification type pointer
	var notifType *string
	if input.NotificationType != "" {
		notifType = &input.NotificationType
	}

	// Capture PID: use pidFn on SessionStart, preserve from existing otherwise
	pid := pidFn()
	if pid == 0 && input.HookEventName != "SessionStart" {
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
		Summary:          ti.summary,
		RuntimeID:        runtimeID,
		PID:              pid,
	}

	return writeSessionFile(sessionFile, s)
}
