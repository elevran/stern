package commands

import "strings"

// isCancel reports whether the first argument is the word "cancel" (case-insensitive).
// Used by handlers that support a "/<verb> cancel" form.
func isCancel(args []string) bool {
	return len(args) > 0 && strings.EqualFold(args[0], "cancel")
}