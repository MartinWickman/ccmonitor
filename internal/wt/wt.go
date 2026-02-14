// Package wt provides Windows Terminal tab operations via UI Automation.
package wt

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/martinwickman/ccmonitor/internal/terminal"
)

// Backend implements terminal.Backend for Windows Terminal tabs.
type Backend struct{}

var _ terminal.Backend = Backend{}

// Name returns "wt".
func (Backend) Name() string { return "wt" }

// Available reports whether the current process is running inside Windows Terminal.
func (Backend) Available() bool { return os.Getenv("WT_SESSION") != "" }

// preamble loads UI Automation assemblies and finds all Windows Terminal windows.
const preamble = `
Add-Type -AssemblyName UIAutomationClient
Add-Type -AssemblyName UIAutomationTypes
$root = [System.Windows.Automation.AutomationElement]::RootElement
$wtCond = New-Object System.Windows.Automation.PropertyCondition([System.Windows.Automation.AutomationElement]::ClassNameProperty, 'CASCADIA_HOSTING_WINDOW_CLASS')
$wtWindows = $root.FindAll([System.Windows.Automation.TreeScope]::Children, $wtCond)
`

func runPowerShell(script string) (string, error) {
	out, err := exec.Command("powershell.exe", "-NoProfile", "-Command", script).Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// Info finds the currently selected tab in the foreground Windows Terminal
// window. Returns both the RuntimeId and the tab name (with title prefix
// stripped). Only meaningful during SessionStart, when the active tab
// is the one where Claude Code just started.
func (Backend) Info() (runtimeID, title string) {
	script := preamble + `
Add-Type -TypeDefinition @"
using System;
using System.Runtime.InteropServices;
public class WinAPI {
    [DllImport("user32.dll")]
    public static extern IntPtr GetForegroundWindow();
}
"@
$fgHwnd = [WinAPI]::GetForegroundWindow()
foreach ($w in $wtWindows) {
    if ($w.Current.NativeWindowHandle -ne [int]$fgHwnd) { continue }
    $tabCond = New-Object System.Windows.Automation.PropertyCondition([System.Windows.Automation.AutomationElement]::ControlTypeProperty, [System.Windows.Automation.ControlType]::TabItem)
    $tabs = $w.FindAll([System.Windows.Automation.TreeScope]::Descendants, $tabCond)
    foreach ($tab in $tabs) {
        try {
            $sel = $tab.GetCurrentPattern([System.Windows.Automation.SelectionItemPattern]::Pattern)
            if ($sel.Current.IsSelected) {
                $rid = $tab.GetRuntimeId()
                ($rid -join ',')
                $tab.Current.Name
                exit
            }
        } catch {}
    }
}`

	out, err := runPowerShell(script)
	if err != nil {
		return "", ""
	}
	lines := strings.SplitN(out, "\n", 2)
	if len(lines) == 0 {
		return "", ""
	}
	runtimeID = strings.TrimSpace(lines[0])
	if len(lines) > 1 {
		title = strings.TrimSpace(lines[1])
	}
	title = terminal.StripTitlePrefix(title)
	return runtimeID, title
}

// Title looks up the current tab name for a Windows Terminal tab identified
// by its RuntimeId. Returns the tab name with title prefix stripped.
// Returns empty string on error.
func (Backend) Title(runtimeID string) string {
	script := preamble + fmt.Sprintf(`
$targetRid = @(%s)
foreach ($w in $wtWindows) {
    $tabCond = New-Object System.Windows.Automation.PropertyCondition([System.Windows.Automation.AutomationElement]::ControlTypeProperty, [System.Windows.Automation.ControlType]::TabItem)
    $tabs = $w.FindAll([System.Windows.Automation.TreeScope]::Descendants, $tabCond)
    foreach ($tab in $tabs) {
        $rid = $tab.GetRuntimeId()
        if (($rid -join ',') -eq ($targetRid -join ',')) {
            $tab.Current.Name
            exit
        }
    }
}`, runtimeID)

	out, err := runPowerShell(script)
	if err != nil {
		return ""
	}
	return terminal.StripTitlePrefix(out)
}

// Select switches to a Windows Terminal tab identified by its RuntimeId.
func (Backend) Select(runtimeID string) error {
	script := preamble + fmt.Sprintf(`
$targetRid = @(%s)
foreach ($w in $wtWindows) {
    $tabCond = New-Object System.Windows.Automation.PropertyCondition([System.Windows.Automation.AutomationElement]::ControlTypeProperty, [System.Windows.Automation.ControlType]::TabItem)
    $tabs = $w.FindAll([System.Windows.Automation.TreeScope]::Descendants, $tabCond)
    foreach ($tab in $tabs) {
        $rid = $tab.GetRuntimeId()
        if (($rid -join ',') -eq ($targetRid -join ',')) {
            $sel = $tab.GetCurrentPattern([System.Windows.Automation.SelectionItemPattern]::Pattern)
            $sel.Select()
            $w.SetFocus()
            exit
        }
    }
}
Write-Error 'Tab not found'
exit 1`, runtimeID)

	cmd := exec.Command("powershell.exe", "-NoProfile", "-Command", script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("switching WT tab: %s: %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}
