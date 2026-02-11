package main

import (
	"flag"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/martinwickman/ccmonitor/internal/hook"
	"github.com/martinwickman/ccmonitor/internal/monitor"
	"github.com/martinwickman/ccmonitor/internal/session"
	"golang.org/x/term"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "hook" {
		if err := hook.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "ccmonitor hook: %v\n", err)
			os.Exit(1)
		}
		return
	}

	once := flag.Bool("once", false, "print current state and exit")
	clean := flag.Bool("clean", false, "remove all session files and exit")
	debug := flag.Bool("debug", false, "show session IDs and PIDs")
	flag.Parse()

	dir := session.Dir()

	if *clean {
		removed, err := session.CleanAll(dir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Removed %d session file(s) from %s\n", removed, dir)
		return
	}

	if *once {
		sessions, err := session.LoadAll(dir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		monitor.CheckPIDLiveness(sessions)
		width := 80
		if w, _, err := term.GetSize(int(os.Stdout.Fd())); err == nil && w > 0 {
			width = w
		}
		fmt.Println(monitor.RenderOnce(sessions, width, *debug))
		return
	}

	p := tea.NewProgram(monitor.New(dir, *debug), tea.WithAltScreen(), tea.WithMouseAllMotion())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
