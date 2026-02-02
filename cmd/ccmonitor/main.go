package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/martinwickman/ccmonitor/internal/monitor"
	"github.com/martinwickman/ccmonitor/internal/session"
)

func main() {
	dir := session.Dir()

	p := tea.NewProgram(monitor.New(dir), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
