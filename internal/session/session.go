package session

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// staleThreshold is the duration after which inactive sessions are considered stale
// and excluded from the monitor display.
const staleThreshold = 1 * time.Hour

// startingStaleThreshold is a shorter threshold for "starting" sessions, which are
// likely orphaned from a resumed session that never received further hook events.
const startingStaleThreshold = 2 * time.Minute

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

// LoadAll reads all session JSON files from dir and returns the parsed sessions.
// Corrupt or unreadable files are skipped silently.
func LoadAll(dir string) ([]Session, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var sessions []Session
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}

		s, err := loadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue // skip corrupt files
		}

		// Skip stale sessions (inactive for longer than staleThreshold)
		if t, err := time.Parse(time.RFC3339, s.LastActivity); err == nil {
			age := time.Since(t)
			if age > staleThreshold {
				continue
			}
			// "starting" sessions that haven't progressed are likely orphaned
			if s.Status == "starting" && age > startingStaleThreshold {
				continue
			}
		}

		sessions = append(sessions, *s)
	}

	return sessions, nil
}

// GroupByProject groups sessions by their project directory, sorted by project name.
// When sortByLatest is true, sessions within each group are sorted by last activity
// (most recent first). Otherwise, sessions are sorted by session ID (stable order).
func GroupByProject(sessions []Session, sortByLatest bool) []ProjectGroup {
	grouped := make(map[string][]Session)
	for _, s := range sessions {
		grouped[s.Project] = append(grouped[s.Project], s)
	}

	var groups []ProjectGroup
	for project, sess := range grouped {
		if sortByLatest {
			sort.Slice(sess, func(i, j int) bool {
				return sess[i].LastActivity > sess[j].LastActivity
			})
		} else {
			sort.Slice(sess, func(i, j int) bool {
				return sess[i].SessionID < sess[j].SessionID
			})
		}
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

// CleanupStale removes session files with last_activity older than staleThreshold.
// Returns nil if the directory does not exist.
func CleanupStale(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("reading sessions dir: %w", err)
	}

	now := time.Now()
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		path := filepath.Join(dir, e.Name())
		s, err := loadFile(path)
		if err != nil {
			continue // skip corrupt files
		}
		t, err := time.Parse(time.RFC3339, s.LastActivity)
		if err != nil {
			continue // skip unparseable timestamps
		}
		if now.Sub(t) > staleThreshold {
			os.Remove(path)
		}
	}
	return nil
}

func loadFile(path string) (*Session, error) {
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
