package session

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// Status constants for session state.
const (
	StatusStarting = "starting"
	StatusWorking  = "working"
	StatusIdle     = "idle"
	StatusWaiting  = "waiting"
	StatusEnded    = "ended"
	StatusExited   = "exited"
)

// Session represents the state of a single Claude Code instance.
type Session struct {
	SessionID        string  `json:"session_id"`
	Project          string  `json:"project"`
	Status           string  `json:"status"`
	Detail           string  `json:"detail"`
	LastPrompt       string  `json:"last_prompt"`
	NotificationType *string `json:"notification_type"`
	LastActivity     string  `json:"last_activity"`
	TmuxPane         string  `json:"tmux_pane"`
	Summary          string  `json:"summary"`
	RuntimeID        string  `json:"wt_tab_id,omitempty"`
	PID              int     `json:"pid,omitempty"`
}

// ProjectGroup holds sessions belonging to the same project directory.
type ProjectGroup struct {
	Project  string
	Sessions []Session
}

// Dir returns the sessions directory, respecting CCMONITOR_SESSIONS_DIR.
func Dir() string {
	if dir := os.Getenv("CCMONITOR_SESSIONS_DIR"); dir != "" {
		return dir
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".ccmonitor", "sessions")
}

// ForEachSessionFile iterates over all valid session files in dir, calling fn
// with the file path and parsed session for each. Corrupt files are skipped.
// Returns nil (not an error) if the directory does not exist.
func ForEachSessionFile(dir string, fn func(path string, s *Session)) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		path := filepath.Join(dir, e.Name())
		s, err := LoadFile(path)
		if err != nil {
			continue // skip corrupt files
		}
		fn(path, s)
	}
	return nil
}

// LoadAll reads all session JSON files from dir and returns the parsed sessions.
// Corrupt or unreadable files are skipped silently. PID liveness checking is the
// caller's responsibility (see monitor package).
func LoadAll(dir string) ([]Session, error) {
	var sessions []Session
	err := ForEachSessionFile(dir, func(_ string, s *Session) {
		sessions = append(sessions, *s)
	})
	return sessions, err
}

// GroupByProject groups sessions by their project directory, sorted by project name.
// Sessions within each group are sorted by session ID (stable order).
func GroupByProject(sessions []Session) []ProjectGroup {
	grouped := make(map[string][]Session)
	for _, s := range sessions {
		grouped[s.Project] = append(grouped[s.Project], s)
	}

	var groups []ProjectGroup
	for project, sess := range grouped {
		sort.Slice(sess, func(i, j int) bool {
			return sess[i].SessionID < sess[j].SessionID
		})
		groups = append(groups, ProjectGroup{Project: project, Sessions: sess})
	}

	sort.Slice(groups, func(i, j int) bool {
		return groups[i].Project < groups[j].Project
	})

	return groups
}

// TimeSince returns a human-readable duration since the given RFC3339 timestamp.
func TimeSince(timestamp string) string {
	t, err := time.Parse(time.RFC3339, timestamp)
	if err != nil {
		return "?"
	}

	d := time.Since(t)
	switch {
	case d < time.Second:
		return "now"
	case d < time.Minute:
		return fmt.Sprintf("%ds ago", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	}
}

// CleanAll removes all .json session files from dir and returns the count removed.
func CleanAll(dir string) (int, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}

	removed := 0
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		if err := os.Remove(filepath.Join(dir, e.Name())); err == nil {
			removed++
		}
	}
	return removed, nil
}

// LoadFile reads and parses a single session file.
func LoadFile(path string) (*Session, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var s Session
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, err
	}

	return &s, nil
}
