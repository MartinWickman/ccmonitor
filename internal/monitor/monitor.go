package monitor

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/martinwickman/ccmonitor/internal/session"
	"github.com/martinwickman/ccmonitor/internal/switcher"
)

// Styles
var (
	titleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("15"))
	countStyle = lipgloss.NewStyle().Faint(true)

	projectStyle     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("15"))
	projectPathStyle = lipgloss.NewStyle().Faint(true)

	workingStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("2")) // green
	waitingStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("3")) // yellow
	idleStyle     = lipgloss.NewStyle().Faint(true)
	startingStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("6")) // cyan
	exitedStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("1")) // red

	promptStyle = lipgloss.NewStyle().Faint(true).Italic(true)

	helpStyle = lipgloss.NewStyle().Faint(true).MarginTop(1)

	projectBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("8")).
			Padding(0, 1).
			MarginTop(1)

	summaryBarStyle = lipgloss.NewStyle().Faint(true).MarginTop(1)
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
}

// New creates a new monitor model that reads from the given directory.
func New(sessionsDir string) Model {
	sessions, _ := session.LoadAll(sessionsDir)

	s := spinner.New()
	s.Spinner = spinner.MiniDot
	s.Style = workingStyle

	return Model{
		sessionsDir:  sessionsDir,
		sessions:     sessions,
		spinner:      s,
		lastState: map[string]string{},
		flashUntil:   map[string]time.Time{},
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
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		return m, nil
	case tea.MouseMsg:
		if msg.Action == tea.MouseActionRelease && msg.Button == tea.MouseButtonLeft {
			if sid, ok := m.clickMap[msg.Y]; ok {
				// Find the session to get tmux pane
				for _, s := range m.sessions {
					if s.SessionID == sid {
						shortID := sid
						if len(shortID) > 8 {
							shortID = shortID[:8]
						}
						if err := switcher.Switch(s.TmuxPane); err != nil {
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
		view := render(m.sessions, m.spinner, m.width, m.flashUntil, "")
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
	return render(m.sessions, m.spinner, m.width, m.flashUntil, status)
}

func render(sessions []session.Session, sp spinner.Model, width int, flashUntil map[string]time.Time, statusMsg string) string {
	if width == 0 {
		width = 80
	}

	if len(sessions) == 0 {
		return titleStyle.Render("ccmonitor") + "\n\n" +
			idleStyle.Render("No active sessions.") + "\n" +
			helpStyle.Render("Press q to quit. Click a session to switch tmux pane.")
	}

	groups := session.GroupByProject(sessions)

	// Box width accounts for border (2) and padding (2)
	boxWidth := width - 4

	var b strings.Builder

	// Header
	header := titleStyle.Render("ccmonitor") + "  " +
		countStyle.Render(fmt.Sprintf("%d projects, %d sessions", len(groups), len(sessions)))
	b.WriteString(header + "\n")

	// Summary bar
	b.WriteString(summaryBarStyle.Render(renderSummary(sessions)))
	b.WriteString("\n")

	// Build rows for all groups and compute global column widths
	groupRows := make([][]sessionRow, len(groups))
	var allRows []sessionRow
	for i, g := range groups {
		rows := buildRows(g.Sessions, sp, flashUntil)
		groupRows[i] = rows
		allRows = append(allRows, rows...)
	}
	w := computeWidths(allRows)

	boxStyle := projectBoxStyle.Width(boxWidth)

	for i, g := range groups {
		box := renderProjectGroup(g, groupRows[i], w)
		b.WriteString(boxStyle.Render(box) + "\n")
	}

	if statusMsg != "" {
		b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("6")).Render(statusMsg) + "\n")
	}

	b.WriteString(helpStyle.Render("Press q to quit. Click a session to switch tmux pane."))

	return b.String()
}

func renderSummary(sessions []session.Session) string {
	counts := map[string]int{}
	for _, s := range sessions {
		counts[s.Status]++
	}

	var parts []string
	if n := counts["working"]; n > 0 {
		parts = append(parts, workingStyle.Render(fmt.Sprintf("● %d working", n)))
	}
	if n := counts["waiting"]; n > 0 {
		parts = append(parts, waitingStyle.Render(fmt.Sprintf("◆ %d waiting", n)))
	}
	if n := counts["idle"]; n > 0 {
		parts = append(parts, idleStyle.Render(fmt.Sprintf("○ %d idle", n)))
	}
	if n := counts["starting"]; n > 0 {
		parts = append(parts, startingStyle.Render(fmt.Sprintf("◌ %d starting", n)))
	}
	if n := counts["exited"]; n > 0 {
		parts = append(parts, exitedStyle.Render(fmt.Sprintf("✕ %d exited", n)))
	}

	return strings.Join(parts, "  ")
}

// sessionRow holds the data for one session table row plus its prompt.
type sessionRow struct {
	connector       string
	shortID         string
	status          string
	detail          string
	elapsed         string
	rawLastActivity string
	prompt          string
	isLast          bool
	flashPhase      int // 0=none, 1=brightest ... 10=dimmest
}

// columnWidths holds the computed widths for each column.
type columnWidths struct {
	conn, id, status, detail int
}

// buildRows converts sessions into styled row data.
func buildRows(sessions []session.Session, sp spinner.Model, flashUntil map[string]time.Time) []sessionRow {
	now := time.Now()
	var rows []sessionRow
	for i, s := range sessions {
		isLast := i == len(sessions)-1
		connector := "├─"
		if isLast {
			connector = "└─"
		}

		shortID := s.SessionID
		if len(shortID) > 8 {
			shortID = shortID[:8]
		}

		indicator, style, label := statusDisplay(s.Status, sp)
		elapsed := session.TimeSince(s.LastActivity)

		detail := s.Detail
		if len(detail) > 40 {
			detail = detail[:37] + "..."
		}

		prompt := s.LastPrompt
		if len(prompt) > 70 {
			prompt = prompt[:67] + "..."
		}

		phase := flashPhase(now, flashUntil[s.SessionID])

		rows = append(rows, sessionRow{
			connector:       lipgloss.NewStyle().Faint(true).Render(connector),
			shortID:         lipgloss.NewStyle().Faint(true).Render(shortID),
			status:          style.Render(indicator + " " + label),
			detail:          detail,
			elapsed:         lipgloss.NewStyle().Faint(true).Render(elapsed),
			rawLastActivity: s.LastActivity,
			prompt:          prompt,
			isLast:          isLast,
			flashPhase:      phase,
		})
	}
	return rows
}

// computeWidths calculates column widths across all rows globally.
func computeWidths(allRows []sessionRow) columnWidths {
	w := columnWidths{status: 12} // fixed minimum to prevent spinner jitter
	for _, r := range allRows {
		w.conn = max(w.conn, lipgloss.Width(r.connector))
		w.id = max(w.id, lipgloss.Width(r.shortID))
		w.status = max(w.status, lipgloss.Width(r.status))
		w.detail = max(w.detail, lipgloss.Width(r.detail))
	}
	return w
}

func renderProjectGroup(g session.ProjectGroup, rows []sessionRow, w columnWidths) string {
	var b strings.Builder

	dirName := filepath.Base(g.Project)
	title := projectStyle.Render(dirName+"/") + " " + projectPathStyle.Render(g.Project)
	b.WriteString(title + "\n")
	b.WriteString(lipgloss.NewStyle().Faint(true).Render("│") + "\n")

	for _, r := range rows {
		elapsed := r.elapsed
		if r.flashPhase == 1 {
			elapsed = lipgloss.NewStyle().
				Foreground(lipgloss.Color("9")). // bright red
				Bold(true).
				Render(session.TimeSince(r.rawLastActivity))
		} else if r.flashPhase == 2 {
			elapsed = lipgloss.NewStyle().Faint(true).
				Render(session.TimeSince(r.rawLastActivity))
		}

		line := padRight(r.connector, w.conn) + " " +
			padRight(r.shortID, w.id) + "  " +
			padRight(r.status, w.status) + "  " +
			padRight(r.detail, w.detail) + "  " +
			elapsed
		b.WriteString(line + "\n")

		if r.prompt != "" {
			indent := lipgloss.NewStyle().Faint(true).Render("│") + "  "
			if r.isLast {
				indent = "   "
			}
			b.WriteString(indent + promptStyle.Render(r.prompt) + "\n")
		}
	}

	return b.String()
}

// padRight pads a string (which may contain ANSI codes) to the given visible width.
func padRight(s string, width int) string {
	visible := lipgloss.Width(s)
	if visible >= width {
		return s
	}
	return s + strings.Repeat(" ", width-visible)
}

// flashPhase returns whether the flash is currently "on" (visible) or "off".
// Returns 0=no flash, 1=on, 2=off (blinking cycle).
func flashPhase(now time.Time, until time.Time) int {
	if until.IsZero() || !now.Before(until) {
		return 0
	}
	// Toggle every 150ms
	elapsed := flashDuration - until.Sub(now)
	cycle := int(elapsed.Milliseconds() / 150)
	if cycle%2 == 0 {
		return 1 // on
	}
	return 2 // off
}

// buildClickMap scans the rendered view for truncated session IDs and maps
// their Y line numbers to full session IDs. This approach is immune to
// layout changes (margins, borders, padding) since it matches actual content.
func buildClickMap(sessions []session.Session, view string) map[int]string {
	clickMap := make(map[int]string)
	if len(sessions) == 0 {
		return clickMap
	}

	// Build a lookup from truncated ID → full session ID.
	shortToFull := make(map[string]string, len(sessions))
	for _, s := range sessions {
		short := s.SessionID
		if len(short) > 8 {
			short = short[:8]
		}
		shortToFull[short] = s.SessionID
	}

	lines := strings.Split(view, "\n")
	for y, line := range lines {
		for short, full := range shortToFull {
			if strings.Contains(line, short) {
				clickMap[y] = full
				// Also map the prompt line directly below, if it exists and
				// isn't already mapped to a different session.
				if y+1 < len(lines) {
					if _, taken := clickMap[y+1]; !taken {
						// Only map if the next line doesn't contain any session ID
						// (which would mean it's another session row, not a prompt).
						isSessionRow := false
						for s := range shortToFull {
							if strings.Contains(lines[y+1], s) {
								isSessionRow = true
								break
							}
						}
						if !isSessionRow {
							clickMap[y+1] = full
						}
					}
				}
				break
			}
		}
	}

	return clickMap
}

func statusDisplay(status string, sp spinner.Model) (indicator string, style lipgloss.Style, label string) {
	switch status {
	case "working":
		return sp.View(), workingStyle, "Working"
	case "waiting":
		return "◆", waitingStyle, "Waiting"
	case "idle":
		return "○", idleStyle, "Idle"
	case "starting":
		return "◌", startingStyle, "Started"
	case "exited":
		return "✕", exitedStyle, "Exited"
	case "ended":
		return "─", idleStyle, "Ended"
	default:
		return "?", idleStyle, status
	}
}
