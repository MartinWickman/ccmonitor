package switcher

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/martinwickman/ccmonitor/internal/session"
)

// Switch focuses the terminal tab/pane for the given session.
// When both WT tab and tmux pane are available (tmux inside WT),
// it switches the WT tab first, then the tmux pane.
func Switch(s session.Session) error {
	if s.RuntimeID != "" && s.TmuxPane != "" {
		if err := switchWT(s.RuntimeID); err != nil {
			return err
		}
		return exec.Command("tmux", "select-pane", "-t", s.TmuxPane).Run()
	}
	if s.RuntimeID != "" {
		return switchWT(s.RuntimeID)
	}
	if s.TmuxPane != "" {
		return exec.Command("tmux", "select-pane", "-t", s.TmuxPane).Run()
	}
	return fmt.Errorf("no switching info available")
}

// switchWT switches to a Windows Terminal tab identified by its RuntimeId.
func switchWT(runtimeID string) error {
	script := fmt.Sprintf(`
Add-Type -AssemblyName UIAutomationClient
Add-Type -AssemblyName UIAutomationTypes
$root = [System.Windows.Automation.AutomationElement]::RootElement
$wtCond = New-Object System.Windows.Automation.PropertyCondition([System.Windows.Automation.AutomationElement]::ClassNameProperty, 'CASCADIA_HOSTING_WINDOW_CLASS')
$wtWindows = $root.FindAll([System.Windows.Automation.TreeScope]::Children, $wtCond)
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
exit 1
`, runtimeID)

	cmd := exec.Command("powershell.exe", "-NoProfile", "-Command", script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("switching WT tab: %s: %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}
