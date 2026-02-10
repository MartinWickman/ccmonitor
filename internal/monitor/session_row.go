package monitor

import (
	"fmt"
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
	pid             int
	status          string
	detail          string
	elapsed         string
	rawLastActivity string
	prompt          string
	isLast          bool
	flashPhase      int // 0=none, 1=brightest ... 10=dimmest
	debug           bool
}

// newSessionRow builds a sessionRow from a session, applying truncation, styling,
// and flash state. isLast indicates whether this is the last session in its group.
func newSessionRow(s session.Session, isLast bool, sp spinner.Model, flashUntil map[string]time.Time, showSummary bool, debug bool) sessionRow {
	now := time.Now()

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

	// Treat default "Claude Code" tab title as empty — it's not useful.
	summary := s.Summary
	if summary == "Claude Code" {
		summary = ""
	}

	var prompt string
	isPrompt := false
	if showSummary {
		prompt = summary
		if prompt == "" {
			prompt = s.LastPrompt
			isPrompt = true
		}
	} else {
		prompt = s.LastPrompt
		isPrompt = true
		if prompt == "" {
			prompt = summary
			isPrompt = false
		}
	}
	if len(prompt) > 70 {
		prompt = prompt[:67] + "..."
	}
	if isPrompt && prompt != "" {
		prompt = "\"" + prompt + "\""
	}

	phase := flashPhase(now, flashUntil[s.SessionID])

	return sessionRow{
		connector:       lipgloss.NewStyle().Faint(true).Render(connector),
		shortID:         lipgloss.NewStyle().Faint(true).Render(shortID),
		pid:             s.PID,
		status:          style.Render(indicator + " " + label),
		detail:          detail,
		elapsed:         lipgloss.NewStyle().Faint(true).Render(elapsed),
		rawLastActivity: s.LastActivity,
		prompt:          prompt,
		isLast:          isLast,
		flashPhase:      phase,
		debug:           debug,
	}
}

// render produces the full output for this row: line 1 is the prompt/summary
// with session ID, line 2 is the status/detail/elapsed.
func (r sessionRow) render(w columnWidths) string {
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

	// Line 1: connector + prompt/summary, with optional (shortID:PID) in debug mode
	var line1 string
	if r.debug {
		idPart := r.shortID
		if r.pid > 0 {
			idPart += ":" + fmt.Sprintf("%d", r.pid)
		}
		if r.prompt != "" {
			line1 = padRight(r.connector, w.conn) + " " +
				promptStyle.Render(r.prompt) + " " +
				lipgloss.NewStyle().Faint(true).Render("("+idPart+")")
		} else {
			line1 = padRight(r.connector, w.conn) + " " +
				lipgloss.NewStyle().Faint(true).Render(idPart)
		}
	} else {
		if r.prompt != "" {
			line1 = padRight(r.connector, w.conn) + " " +
				promptStyle.Render(r.prompt)
		} else {
			line1 = padRight(r.connector, w.conn) + " " +
				lipgloss.NewStyle().Faint(true).Render("(no description)")
		}
	}

	// Line 2: indent + status + detail + elapsed
	indent := lipgloss.NewStyle().Faint(true).Render("│") + "  "
	if r.isLast {
		indent = "   "
	}
	line2 := indent +
		padRight(r.status, w.status) + "  " +
		padRight(r.detail, w.detail) + "  " +
		elapsed

	return line1 + "\n" + line2 + "\n"
}

// padRight pads a string (which may contain ANSI codes) to the given visible width.
func padRight(s string, width int) string {
	visible := lipgloss.Width(s)
	if visible >= width {
		return s
	}
	return s + strings.Repeat(" ", width-visible)
}

// widths returns the visible column widths for this row.
func (r sessionRow) widths() columnWidths {
	return columnWidths{
		conn:   lipgloss.Width(r.connector),
		status: lipgloss.Width(r.status),
		detail: lipgloss.Width(r.detail),
	}
}

// statusDisplay returns the indicator character, style, and label for a status.
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
