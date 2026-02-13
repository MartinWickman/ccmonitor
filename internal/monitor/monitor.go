package monitor

import (
	"fmt"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	ps "github.com/mitchellh/go-ps"

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
	// showSummary toggles subtitle display: true=prefer summary, false=prefer prompt.
	showSummary bool
	// debug shows session IDs and PIDs in the display.
	debug bool
	// hoverSID is the session ID currently under the mouse cursor.
	hoverSID string
	// lastPIDCheck is when CheckPIDLiveness was last run.
	lastPIDCheck time.Time
}

// CheckPIDLiveness marks sessions with dead PIDs as "exited".
// Sessions record the OS they were created on. When the monitor runs on a
// different OS (e.g. Windows .exe reading WSL sessions), it uses the
// appropriate method to check each PID.
func CheckPIDLiveness(sessions []session.Session) {
	alive := alivePIDs(sessions)
	for i := range sessions {
		if sessions[i].PID <= 0 {
			continue
		}
		if !alive[sessions[i].PID] {
			sessions[i].Status = session.StatusExited
			sessions[i].Detail = "Process ended"
		}
	}
}

// alivePIDs returns the set of PIDs that are still running.
// Sessions record which OS they were created on. When the monitor runs on a
// different OS, cross-platform checks are used:
//   - Windows monitor + Linux session → batch-check via "wsl kill -0"
//   - Linux monitor + Windows session → batch-check via "powershell.exe Get-Process"
//   - Same OS → native go-ps
func alivePIDs(sessions []session.Session) map[int]bool {
	alive := make(map[int]bool)
	var wslPIDs, winPIDs []int

	for i := range sessions {
		if sessions[i].PID <= 0 {
			continue
		}
		switch {
		case runtime.GOOS == "windows" && sessions[i].OS != "windows":
			wslPIDs = append(wslPIDs, sessions[i].PID)
		case runtime.GOOS != "windows" && sessions[i].OS == "windows":
			winPIDs = append(winPIDs, sessions[i].PID)
		default:
			alive[sessions[i].PID] = isNativePIDAlive(sessions[i].PID)
		}
	}

	for pid, ok := range checkWSLPIDs(wslPIDs) {
		alive[pid] = ok
	}
	for pid, ok := range checkWindowsPIDs(winPIDs) {
		alive[pid] = ok
	}

	return alive
}

// isNativePIDAlive checks a PID using the native OS process table (go-ps).
func isNativePIDAlive(pid int) bool {
	proc, err := ps.FindProcess(pid)
	if err != nil {
		return true // assume alive on error
	}
	return proc != nil
}

// checkWSLPIDs checks Linux PIDs from Windows via "wsl kill -0 <pid>".
func checkWSLPIDs(pids []int) map[int]bool {
	alive := make(map[int]bool)
	for _, pid := range pids {
		if exec.Command("wsl", "kill", "-0", strconv.Itoa(pid)).Run() == nil {
			alive[pid] = true
		}
	}
	return alive
}

// checkWindowsPIDs batch-checks Windows PIDs from WSL via powershell.exe.
func checkWindowsPIDs(pids []int) map[int]bool {
	alive := make(map[int]bool)
	if len(pids) == 0 {
		return alive
	}
	var pidList string
	for _, pid := range pids {
		if pidList != "" {
			pidList += ","
		}
		pidList += strconv.Itoa(pid)
	}
	script := pidList + " | ForEach-Object { if (Get-Process -Id $_ -ErrorAction SilentlyContinue) { $_ } }"
	out, err := exec.Command("powershell.exe", "-NoProfile", "-Command", script).Output()
	if err != nil {
		return alive
	}
	return parseAlivePIDs(string(out))
}

// parseAlivePIDs extracts PIDs from newline-separated output.
func parseAlivePIDs(output string) map[int]bool {
	alive := make(map[int]bool)
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		if pid, err := strconv.Atoi(strings.TrimSpace(line)); err == nil {
			alive[pid] = true
		}
	}
	return alive
}

// New creates a new monitor model that reads from the given directory.
func New(sessionsDir string, debug bool) Model {
	sessions, _ := session.LoadAll(sessionsDir)
	CheckPIDLiveness(sessions)

	s := spinner.New()
	s.Spinner = spinner.MiniDot
	s.Style = workingStyle

	return Model{
		sessionsDir:  sessionsDir,
		sessions:     sessions,
		spinner:      s,
		lastState:    map[string]string{},
		flashUntil:   map[string]time.Time{},
		showSummary:  false,
		debug:        debug,
		lastPIDCheck: time.Now(),
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
		case "p":
			m.showSummary = !m.showSummary
			return m, nil
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		return m, nil
	case tea.MouseMsg:
		// Update hover state on any mouse event
		m.hoverSID = m.clickMap[msg.Y]

		if msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft {
			if sid, ok := m.clickMap[msg.Y]; ok {
				// Find the session to switch to
				for _, s := range m.sessions {
					if s.SessionID == sid {
						proj := baseName(s.Project)
						m.statusMsg = fmt.Sprintf("Switching to %s...", proj)
						m.statusUntil = time.Now().Add(3 * time.Second)
						sess := s // capture for goroutine
						go func() {
							switcher.Switch(sess)
						}()
						break
					}
				}
			}
		}
		return m, nil
	case tickMsg:
		m.sessions, _ = session.LoadAll(m.sessionsDir)
		if time.Since(m.lastPIDCheck) >= 10*time.Second {
			CheckPIDLiveness(m.sessions)
			m.lastPIDCheck = time.Now()
		}
		// Build click map by scanning the actual rendered view for session IDs.
		view := render(m.sessions, m.spinner, m.width, m.flashUntil, "", m.showSummary, m.debug, "")
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
	return render(m.sessions, m.spinner, m.width, m.flashUntil, status, m.showSummary, m.debug, m.hoverSID)
}
