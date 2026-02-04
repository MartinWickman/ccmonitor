package monitor

import (
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/martinwickman/ccmonitor/internal/session"
	"github.com/martinwickman/ccmonitor/internal/switcher"
)

// tickMsg is sent on every refresh interval (session reload).
type tickMsg time.Time

// flashTickMsg is sent on a faster interval for smooth flash animation.
type flashTickMsg time.Time

func tickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func flashTickCmd() tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(t time.Time) tea.Msg {
		return flashTickMsg(t)
	})
}

const flashDuration = 2 * time.Second

// Model holds the state for the Bubble Tea program.
type Model struct {
	sessionsDir string
	sessions    []session.Session
	spinner     spinner.Model
	width       int
	// lastState tracks the last known status+detail per session ID for change detection.
	lastState map[string]string
	// flashUntil tracks when the flash expires per session ID.
	flashUntil map[string]time.Time
	// clickMap maps Y line number to session ID for mouse click handling.
	clickMap map[int]string
	// statusMsg is feedback text shown after a click action.
	statusMsg string
	// statusUntil is when to clear the status message.
	statusUntil time.Time
	// sortByLatest toggles session sort order: false=by session ID, true=by latest activity.
	sortByLatest bool
	// showSummary toggles subtitle display: true=prefer summary, false=prefer prompt.
	showSummary bool
}

// New creates a new monitor model that reads from the given directory.
func New(sessionsDir string) Model {
	sessions, _ := session.LoadAll(sessionsDir)

	s := spinner.New()
	s.Spinner = spinner.MiniDot
	s.Style = workingStyle

	return Model{
		sessionsDir: sessionsDir,
		sessions:    sessions,
		spinner:     s,
		lastState:   map[string]string{},
		flashUntil:  map[string]time.Time{},
		showSummary: false,
		sortByLatest: false,
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(tickCmd(), flashTickCmd(), m.spinner.Tick)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "s":
			m.sortByLatest = !m.sortByLatest
			return m, nil
		case "p":
			m.showSummary = !m.showSummary
			return m, nil
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		return m, nil
	case tea.MouseMsg:
		if msg.Action == tea.MouseActionRelease && msg.Button == tea.MouseButtonLeft {
			if sid, ok := m.clickMap[msg.Y]; ok {
				// Find the session to switch to
				for _, s := range m.sessions {
					if s.SessionID == sid {
						shortID := sid
						if len(shortID) > 8 {
							shortID = shortID[:8]
						}
						if err := switcher.Switch(s); err != nil {
							m.statusMsg = fmt.Sprintf("Error: %v", err)
						} else {
							m.statusMsg = fmt.Sprintf("Switched to %s", shortID)
						}
						m.statusUntil = time.Now().Add(3 * time.Second)
						break
					}
				}
			}
		}
		return m, nil
	case tickMsg:
		m.sessions, _ = session.LoadAll(m.sessionsDir)
		// Build click map by scanning the actual rendered view for session IDs.
		view := render(m.sessions, m.spinner, m.width, m.flashUntil, "", m.sortByLatest, m.showSummary)
		m.clickMap = buildClickMap(m.sessions, view)
		now := time.Now()
		newFlash := false
		for _, s := range m.sessions {
			state := s.Status + "|" + s.Detail
			prev, known := m.lastState[s.SessionID]
			if known && prev != state {
				m.flashUntil[s.SessionID] = now.Add(flashDuration)
				newFlash = true
			}
			m.lastState[s.SessionID] = state
		}
		cmds := []tea.Cmd{tickCmd()}
		if newFlash {
			cmds = append(cmds, flashTickCmd())
		}
		return m, tea.Batch(cmds...)
	case flashTickMsg:
		// Re-render to update flash animation; only keep ticking if flashes are active
		hasFlash := false
		now := time.Now()
		for _, until := range m.flashUntil {
			if now.Before(until) {
				hasFlash = true
				break
			}
		}
		if hasFlash {
			return m, flashTickCmd()
		}
		return m, nil
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m Model) View() string {
	var status string
	if m.statusMsg != "" && time.Now().Before(m.statusUntil) {
		status = m.statusMsg
	}
	return render(m.sessions, m.spinner, m.width, m.flashUntil, status, m.sortByLatest, m.showSummary)
}
