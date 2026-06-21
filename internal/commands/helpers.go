package commands

import (
	"strings"

	"github.com/elevran/stern/internal/github"
)

// isCancel reports whether the first argument is the word "cancel" (case-insensitive).
// Used by handlers that support a "/<verb> cancel" form.
func isCancel(args []string) bool {
	return len(args) > 0 && strings.EqualFold(args[0], "cancel")
}

// userCommandClient is the minimum Client surface used by handlers that
// resolve users against an org/repo (assign, cc, and their un- variants).
// Shared by AssignHandler and CCHandler to avoid duplicated interface
// declarations.
type userCommandClient interface {
	github.PermissionsClient
	github.UsersClient
}