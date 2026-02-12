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
	sessionID       string
	connector       string
	shortID         string
	pid             int
	status          string
	detail          string
	elapsed         string
	rawLastActivity string
	prompt          string
	isQuoted        bool // true if prompt should be wrapped in quotes
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
		detail = detail[:38] + " …"
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
	isQuoted := isPrompt && prompt != ""

	phase := flashPhase(now, flashUntil[s.SessionID])

	return sessionRow{
		sessionID:       s.SessionID,
		connector:       connector,
		shortID:         lipgloss.NewStyle().Faint(true).Render(shortID),
		pid:             s.PID,
		status:          style.Render(indicator + " " + label),
		detail:          detail,
		elapsed:         lipgloss.NewStyle().Faint(true).Render(elapsed),
		rawLastActivity: s.LastActivity,
		prompt:          prompt,
		isQuoted:        isQuoted,
		isLast:          isLast,
		flashPhase:      phase,
		debug:           debug,
	}
}

// render produces the full output for this row: line 1 is the prompt/summary
// with session ID, line 2 is the status/detail/elapsed.
func (r sessionRow) render(w columnWidths, hovered bool) string {
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

	// Style connector: bold when hovered, faint otherwise
	var styledConn string
	if hovered {
		styledConn = lipgloss.NewStyle().Bold(true).Render(r.connector)
	} else {
		styledConn = lipgloss.NewStyle().Faint(true).Render(r.connector)
	}

	// Line 1: connector + prompt/summary, with optional (shortID:PID) in debug mode
	textStyle := promptStyle
	faintStyle := lipgloss.NewStyle().Faint(true)
	if hovered {
		textStyle = lipgloss.NewStyle().Bold(true)
		faintStyle = lipgloss.NewStyle().Bold(true)
	}

	// Compute available width for prompt text, then truncate to fit
	prompt := r.prompt
	if w.contentWidth > 0 && prompt != "" {
		available := w.contentWidth - w.conn - 1 - 8 // connector + space + right margin
		if r.isQuoted {
			available -= 2 // surrounding quotes
		}
		if r.debug {
			// Account for " (shortID)" or " (shortID:PID)"
			suffixLen := 2 + lipgloss.Width(r.shortID) + 1 // space + ( + shortID + )
			if r.pid > 0 {
				suffixLen += 1 + len(fmt.Sprintf("%d", r.pid)) // : + PID digits
			}
			available -= suffixLen
		}
		if available < 0 {
			available = 0
		}
		if len(prompt) > available {
			if available > 2 {
				prompt = prompt[:available-2] + "…"
			} else {
				prompt = prompt[:available]
			}
		}
	}
	if r.isQuoted && prompt != "" {
		prompt = "\"" + prompt + "\""
	}

	var line1 string
	if r.debug {
		idPart := r.shortID
		if r.pid > 0 {
			idPart += ":" + fmt.Sprintf("%d", r.pid)
		}
		if prompt != "" {
			line1 = padRight(styledConn, w.conn) + " " +
				textStyle.Render(prompt) + " " +
				faintStyle.Render("("+idPart+")")
		} else {
			line1 = padRight(styledConn, w.conn) + " " +
				faintStyle.Render(idPart)
		}
	} else {
		if prompt != "" {
			line1 = padRight(styledConn, w.conn) + " " +
				textStyle.Render(prompt)
		} else {
			line1 = padRight(styledConn, w.conn) + " " +
				faintStyle.Render("…")
		}
	}

	// Line 2: indent + status + detail ... elapsed (right-aligned)
	indent := lipgloss.NewStyle().Faint(true).Render("│") + "  "
	if r.isLast {
		indent = "   "
	}
	leftPart := indent +
		padRight(r.status, w.status) + "  " +
		r.detail

	elapsedWidth := lipgloss.Width(elapsed)
	leftWidth := lipgloss.Width(leftPart)
	// Right-align elapsed to contentWidth, with at least 2 spaces gap
	targetWidth := w.contentWidth - elapsedWidth
	if targetWidth > leftWidth+2 {
		leftPart = leftPart + strings.Repeat(" ", targetWidth-leftWidth)
	} else {
		leftPart = leftPart + "  "
	}
	line2 := leftPart + elapsed

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
	}
}

// statusDisplay returns the indicator character, style, and label for a status.
func statusDisplay(status string, sp spinner.Model) (indicator string, style lipgloss.Style, label string) {
	switch status {
	case session.StatusWorking:
		return sp.View(), workingStyle, "Working"
	case session.StatusWaiting:
		return "◆", waitingStyle, "Waiting"
	case session.StatusIdle:
		return "○", idleStyle, "Idle"
	case session.StatusStarting:
		return "◌", startingStyle, "Started"
	case session.StatusExited:
		return "✕", exitedStyle, "Exited"
	case session.StatusEnded:
		return "─", idleStyle, "Ended"
	default:
		return "?", idleStyle, status
	}
}
