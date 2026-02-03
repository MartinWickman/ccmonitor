package monitor

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/lipgloss"
	"github.com/martinwickman/ccmonitor/internal/session"
)

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

// RenderOnce produces a single snapshot of the current sessions for non-interactive output.
func RenderOnce(sessions []session.Session, width int) string {
	sp := spinner.New()
	sp.Spinner = spinner.MiniDot
	return renderView(sessions, sp, width, nil, "", false)
}

func render(sessions []session.Session, sp spinner.Model, width int, flashUntil map[string]time.Time, statusMsg string) string {
	return renderView(sessions, sp, width, flashUntil, statusMsg, true)
}

func renderView(sessions []session.Session, sp spinner.Model, width int, flashUntil map[string]time.Time, statusMsg string, interactive bool) string {
	if width == 0 {
		width = 80
	}

	if len(sessions) == 0 {
		s := titleStyle.Render("ccmonitor") + "\n\n" +
			idleStyle.Render("No active sessions.")
		if interactive {
			s += "\n" + helpStyle.Render("Press q to quit. Click a session to switch tmux pane.")
		}
		return s
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

	if interactive {
		if statusMsg != "" {
			b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("6")).Render(statusMsg) + "\n")
		}
		b.WriteString(helpStyle.Render("Press q to quit. Click a session to switch tmux pane."))
	}

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
