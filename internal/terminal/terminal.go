// Package terminal defines a common interface for terminal backends (tmux, Windows Terminal, etc.).
package terminal

import (
	"strings"
	"unicode"
)

// Backend abstracts terminal tab/pane operations.
type Backend interface {
	Name() string              // Backend key, e.g. "tmux", "wt"
	Available() bool           // Whether this backend is active (checks env vars)
	Info() (id, title string)  // Discover current tab/pane
	Title(id string) string    // Refresh title for known ID
	Select(id string) error    // Switch focus to tab/pane
}

// StripTitlePrefix removes leading non-alphanumeric characters from a tab/pane
// title. Claude Code prefixes titles with status indicators like "✳ " but the
// exact character varies by platform and encoding.
func StripTitlePrefix(title string) string {
	i := strings.IndexFunc(title, func(r rune) bool {
		return unicode.IsLetter(r) || unicode.IsDigit(r)
	})
	if i > 0 {
		return title[i:]
	}
	return title
}
