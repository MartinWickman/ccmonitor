package monitor

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/martinwickman/ccmonitor/internal/session"
)

// Styles
var (
	titleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("15"))
	countStyle = lipgloss.NewStyle().Faint(true)

	projectStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("15"))
	projectPathStyle = lipgloss.NewStyle().Faint(true)

	workingStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("2")) // green
	waitingStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("3")) // yellow
	idleStyle     = lipgloss.NewStyle().Faint(true)
	startingStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("6")) // cyan
	exitedStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("1")) // red

	sessionIDStyle = lipgloss.NewStyle().Faint(true)
	detailStyle    = lipgloss.NewStyle()
	promptStyle    = lipgloss.NewStyle().Faint(true).Italic(true)
	timeStyle      = lipgloss.NewStyle().Faint(true)
	connectorStyle = lipgloss.NewStyle().Faint(true)

	helpStyle = lipgloss.NewStyle().Faint(true).MarginTop(1)

	projectBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("8")).
			Padding(0, 1).
			MarginTop(1)
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
}

// New creates a new monitor model that reads from the given directory.
func New(sessionsDir string) Model {
	sessions, _ := session.LoadAll(sessionsDir)
	return Model{
		sessionsDir: sessionsDir,
		sessions:    sessions,
	}
}

func (m Model) Init() tea.Cmd {
	return tickCmd()
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		}
	case tickMsg:
		m.sessions, _ = session.LoadAll(m.sessionsDir)
		return m, tickCmd()
	}
	return m, nil
}

func (m Model) View() string {
	return render(m.sessions)
}

func render(sessions []session.Session) string {
	if len(sessions) == 0 {
		return titleStyle.Render("ccmonitor") + "\n\n" +
			idleStyle.Render("No active sessions.") + "\n" +
			helpStyle.Render("Press q to quit.")
	}

	groups := session.GroupByProject(sessions)

	var b strings.Builder

	// Header
	header := titleStyle.Render("ccmonitor") + "  " +
		countStyle.Render(fmt.Sprintf("%d projects, %d sessions", len(groups), len(sessions)))
	b.WriteString(header + "\n")

	for _, g := range groups {
		box := renderProjectGroup(g)
		b.WriteString(projectBoxStyle.Render(box) + "\n")
	}

	b.WriteString(helpStyle.Render("Press q to quit."))

	return b.String()
}

func renderProjectGroup(g session.ProjectGroup) string {
	var b strings.Builder

	dirName := filepath.Base(g.Project)
	title := projectStyle.Render(dirName+"/") + " " + projectPathStyle.Render(g.Project)
	b.WriteString(title + "\n")

	for i, s := range g.Sessions {
		isLast := i == len(g.Sessions)-1
		connector := "├─"
		if isLast {
			connector = "└─"
		}

		shortID := s.SessionID
		if len(shortID) > 8 {
			shortID = shortID[:8]
		}

		indicator, style, label := statusDisplay(s.Status)
		elapsed := session.TimeSince(s.LastActivity)

		detail := s.Detail
		if len(detail) > 40 {
			detail = detail[:37] + "..."
		}

		line := fmt.Sprintf("%s %s  %s  %-40s %s",
			connectorStyle.Render(connector),
			sessionIDStyle.Render(shortID),
			style.Render(indicator+" "+label),
			detailStyle.Render(detail),
			timeStyle.Render(elapsed),
		)
		b.WriteString(line + "\n")

		if s.LastPrompt != "" {
			indent := connectorStyle.Render("│") + "  "
			if isLast {
				indent = "   "
			}
			prompt := s.LastPrompt
			if len(prompt) > 60 {
				prompt = prompt[:57] + "..."
			}
			b.WriteString(indent + promptStyle.Render(prompt) + "\n")
		}
	}

	return b.String()
}

func statusDisplay(status string) (indicator string, style lipgloss.Style, label string) {
	switch status {
	case "working":
		return "●", workingStyle, "Working"
	case "waiting":
		return "◆", waitingStyle, "Waiting"
	case "idle":
		return "○", idleStyle, "Idle"
	case "starting":
		return "◌", startingStyle, "Starting"
	case "exited":
		return "✕", exitedStyle, "Exited"
	case "ended":
		return "─", idleStyle, "Ended"
	default:
		return "?", idleStyle, status
	}
}
