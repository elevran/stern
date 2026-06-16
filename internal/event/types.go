package event

import "github.com/google/go-github/v72/github"

// Type aliases — callers import event, not go-github directly.
type (
	CommentEvent    = github.IssueCommentEvent
	PREvent         = github.PullRequestEvent
	IssueEvent      = github.IssuesEvent
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
	PR *github.PullRequest
}
