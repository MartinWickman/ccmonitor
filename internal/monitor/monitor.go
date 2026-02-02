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

// tickMsg is sent on every refresh interval.
type tickMsg time.Time

func tickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

// Model holds the state for the Bubble Tea program.
type Model struct {
	sessionsDir string
	sessions    []session.Session
	spinner     spinner.Model
	width       int
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
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(tickCmd(), m.spinner.Tick)
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
	case tickMsg:
		m.sessions, _ = session.LoadAll(m.sessionsDir)
		return m, tickCmd()
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m Model) View() string {
	return render(m.sessions, m.spinner, m.width)
}

func render(sessions []session.Session, sp spinner.Model, width int) string {
	if width == 0 {
		width = 80
	}

	if len(sessions) == 0 {
		return titleStyle.Render("ccmonitor") + "\n\n" +
			idleStyle.Render("No active sessions.") + "\n" +
			helpStyle.Render("Press q to quit.")
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
		rows := buildRows(g.Sessions, sp)
		groupRows[i] = rows
		allRows = append(allRows, rows...)
	}
	w := computeWidths(allRows)

	boxStyle := projectBoxStyle.Width(boxWidth)

	for i, g := range groups {
		box := renderProjectGroup(g, groupRows[i], w)
		b.WriteString(boxStyle.Render(box) + "\n")
	}

	b.WriteString(helpStyle.Render("Press q to quit."))

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
	connector string
	shortID   string
	status    string
	detail    string
	elapsed   string
	prompt    string
	isLast    bool
}

// columnWidths holds the computed widths for each column.
type columnWidths struct {
	conn, id, status, detail int
}

// buildRows converts sessions into styled row data.
func buildRows(sessions []session.Session, sp spinner.Model) []sessionRow {
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

		rows = append(rows, sessionRow{
			connector: lipgloss.NewStyle().Faint(true).Render(connector),
			shortID:   lipgloss.NewStyle().Faint(true).Render(shortID),
			status:    style.Render(indicator + " " + label),
			detail:    detail,
			elapsed:   lipgloss.NewStyle().Faint(true).Render(elapsed),
			prompt:    prompt,
			isLast:    isLast,
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
		line := padRight(r.connector, w.conn) + " " +
			padRight(r.shortID, w.id) + "  " +
			padRight(r.status, w.status) + "  " +
			padRight(r.detail, w.detail) + "  " +
			r.elapsed
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

func statusDisplay(status string, sp spinner.Model) (indicator string, style lipgloss.Style, label string) {
	switch status {
	case "working":
		return sp.View(), workingStyle, "Working"
	case "waiting":
		return "◆", waitingStyle, "Waiting"
	case "idle":
		return "○", idleStyle, "Idle"
	case "starting":
		return sp.View(), startingStyle, "Starting"
	case "exited":
		return "✕", exitedStyle, "Exited"
	case "ended":
		return "─", idleStyle, "Ended"
	default:
		return "?", idleStyle, status
	}
}
