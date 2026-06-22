package event

import (
	"github.com/google/go-github/v72/github"

	ighub "github.com/elevran/stern/internal/github"
)

// Type aliases — callers import event, not go-github directly. The revive
// `exported` rule requires each alias to begin with the type name; the
// per-type doc comments below satisfy that rule.
type (
	// CommentEvent is the GitHub issue_comment webhook payload.
	CommentEvent = github.IssueCommentEvent
	// PREvent is the GitHub pull_request webhook payload.
	PREvent = github.PullRequestEvent
	// IssueEvent is the GitHub issues webhook payload.
	IssueEvent = github.IssuesEvent
	// CheckSuiteEvent is the GitHub check_suite webhook payload.
	CheckSuiteEvent = github.CheckSuiteEvent
)

// Context is the runtime state passed to every handler invocation.
type Context struct {
	Org  string
	Repo string

	// BotLogin is the GitHub login of the bot account (e.g. "github-actions[bot]").
	BotLogin string

	// ActionsRunURL is the URL of the current Actions run, used in error comments.
	ActionsRunURL string

	// Set for slash-command invocations.
	CommentID   int64
	Author      string // login of the comment author
	IssueNumber int

	// PR is non-nil when the slash-command targets a pull request.
	PR *ighub.PullRequest
}
