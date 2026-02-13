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

// columnWidths holds the computed widths for each column.
type columnWidths struct {
	conn, status int
	contentWidth int // total available width inside the box
}

// RenderOnce produces a single snapshot of the current sessions for non-interactive output.
func RenderOnce(sessions []session.Session, width int, debug bool) string {
	sp := spinner.New()
	sp.Spinner = spinner.MiniDot
	return renderView(sessions, sp, width, nil, "", false, true, debug, "")
}

func render(sessions []session.Session, sp spinner.Model, width int, flashUntil map[string]time.Time, statusMsg string, showSummary bool, debug bool, hoverSID string) string {
	return renderView(sessions, sp, width, flashUntil, statusMsg, true, showSummary, debug, hoverSID)
}

func renderView(sessions []session.Session, sp spinner.Model, width int, flashUntil map[string]time.Time, statusMsg string, interactive bool, showSummary bool, debug bool, hoverSID string) string {
	if width == 0 {
		width = 80
	}

	if len(sessions) == 0 {
		s := titleStyle.Render("ccmonitor") + "\n\n" +
			idleStyle.Render("No active sessions.")
		if interactive {
			s += "\n" + renderHelp(showSummary)
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
		rows := buildRows(g.Sessions, sp, flashUntil, showSummary, debug)
		groupRows[i] = rows
		allRows = append(allRows, rows...)
	}
	w := computeWidths(allRows)
	w.contentWidth = boxWidth - 2 // subtract left+right padding (1 each)

	boxStyle := projectBoxStyle.Width(boxWidth)

	for i, g := range groups {
		box := renderProjectGroup(g, groupRows[i], w, hoverSID)
		b.WriteString(boxStyle.Render(box) + "\n")
	}

	if interactive {
		if statusMsg != "" {
			b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("6")).Render(statusMsg) + "\n")
		}
		b.WriteString(renderHelp(showSummary))
	}

	return b.String()
}

func renderHelp(showSummary bool) string {
	faint := lipgloss.NewStyle().Faint(true).Render
	bold := lipgloss.NewStyle().Bold(true).Render

	var toggle string
	if showSummary {
		toggle = faint("p prompt/") + bold("title")
	} else {
		toggle = faint("p ") + bold("prompt") + faint("/title")
	}

	line := faint("q quit · ") + toggle + faint(" · click to switch tab")
	return helpStyle.Render(line)
}

func renderSummary(sessions []session.Session) string {
	counts := map[string]int{}
	for _, s := range sessions {
		counts[s.Status]++
	}

	var parts []string
	if n := counts[session.StatusWorking]; n > 0 {
		parts = append(parts, workingStyle.Render(fmt.Sprintf("● %d working", n)))
	}
	if n := counts[session.StatusWaiting]; n > 0 {
		parts = append(parts, waitingStyle.Render(fmt.Sprintf("◆ %d waiting", n)))
	}
	if n := counts[session.StatusIdle]; n > 0 {
		parts = append(parts, idleStyle.Render(fmt.Sprintf("○ %d idle", n)))
	}
	if n := counts[session.StatusStarting]; n > 0 {
		parts = append(parts, startingStyle.Render(fmt.Sprintf("◌ %d starting", n)))
	}
	if n := counts[session.StatusExited]; n > 0 {
		parts = append(parts, exitedStyle.Render(fmt.Sprintf("✕ %d exited", n)))
	}

	return strings.Join(parts, "  ")
}

// buildRows converts sessions into styled row data.
func buildRows(sessions []session.Session, sp spinner.Model, flashUntil map[string]time.Time, showSummary bool, debug bool) []sessionRow {
	var rows []sessionRow
	for i, s := range sessions {
		isLast := i == len(sessions)-1
		rows = append(rows, newSessionRow(s, isLast, sp, flashUntil, showSummary, debug))
	}
	return rows
}

// computeWidths calculates column widths across all rows globally.
func computeWidths(allRows []sessionRow) columnWidths {
	w := columnWidths{status: 12} // fixed minimum to prevent spinner jitter
	for _, r := range allRows {
		rw := r.widths()
		w.conn = max(w.conn, rw.conn)
		w.status = max(w.status, rw.status)
	}
	return w
}

// baseName extracts the last path component, handling both forward and
// backslash separators so Windows paths work correctly on Linux/WSL.
func baseName(path string) string {
	// Try the native separator first, then the other one.
	name := filepath.Base(path)
	if i := strings.LastIndex(name, "\\"); i >= 0 {
		name = name[i+1:]
	}
	if name == "" {
		return path
	}
	return name
}

func renderProjectGroup(g session.ProjectGroup, rows []sessionRow, w columnWidths, hoverSID string) string {
	var b strings.Builder

	dirName := baseName(g.Project)
	title := projectStyle.Render(dirName) + " " + projectPathStyle.Render(g.Project)
	b.WriteString(title + "\n")
	b.WriteString(lipgloss.NewStyle().Faint(true).Render("│") + "\n")

	for _, r := range rows {
		b.WriteString(r.render(w, r.sessionID == hoverSID))
	}

	return b.String()
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

// buildClickMap scans the rendered view for tree connectors (├─ / └─) and maps
// their Y line numbers to session IDs. Connectors appear in the same order as
// sessions are rendered, so we flatten the groups and match by position.
func buildClickMap(sessions []session.Session, view string) map[int]string {
	clickMap := make(map[int]string)
	if len(sessions) == 0 {
		return clickMap
	}

	// Flatten sessions in render order.
	groups := session.GroupByProject(sessions)
	var ordered []session.Session
	for _, g := range groups {
		ordered = append(ordered, g.Sessions...)
	}

	lines := strings.Split(view, "\n")
	sessionIdx := 0
	for y, line := range lines {
		if sessionIdx >= len(ordered) {
			break
		}
		if strings.Contains(line, "├─") || strings.Contains(line, "└─") {
			sid := ordered[sessionIdx].SessionID
			clickMap[y] = sid
			// Also map the status line directly below.
			if y+1 < len(lines) {
				clickMap[y+1] = sid
			}
			sessionIdx++
		}
	}

	return clickMap
}
