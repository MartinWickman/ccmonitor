package monitor

import "github.com/charmbracelet/lipgloss"

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
