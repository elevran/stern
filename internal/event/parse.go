package event

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// IsBot reports whether sender should be treated as a bot and its events
// skipped. Matches accounts ending in "[bot]" (GitHub Apps) or the configured
// bot login.
func IsBot(sender, botLogin string) bool {
	return strings.HasSuffix(sender, "[bot]") || sender == botLogin
}

// ParseCommentEvent parses GITHUB_EVENT_PATH as an IssueCommentEvent.
func ParseCommentEvent() (*CommentEvent, error) {
	return parseEvent[CommentEvent]()
}

// ParsePREvent parses GITHUB_EVENT_PATH as a PullRequestEvent.
func ParsePREvent() (*PREvent, error) {
	return parseEvent[PREvent]()
}

// ParseIssueEvent parses GITHUB_EVENT_PATH as an IssuesEvent.
func ParseIssueEvent() (*IssueEvent, error) {
	return parseEvent[IssueEvent]()
}

// ParseCheckSuiteEvent parses GITHUB_EVENT_PATH as a CheckSuiteEvent.
func ParseCheckSuiteEvent() (*CheckSuiteEvent, error) {
	return parseEvent[CheckSuiteEvent]()
}

func parseEvent[T any]() (*T, error) {
	path := os.Getenv("GITHUB_EVENT_PATH")
	if path == "" {
		return nil, fmt.Errorf("GITHUB_EVENT_PATH is not set")
	}
	data, err := os.ReadFile(path) // #nosec G304,G703 -- path is set by GitHub Actions in the runner environment
	if err != nil {
		return nil, fmt.Errorf("reading event file %s: %w", path, err)
	}
	var e T
	if err := json.Unmarshal(data, &e); err != nil {
		return nil, fmt.Errorf("parsing event: %w", err)
	}
	return &e, nil
}

// OrgRepoFromEnv splits GITHUB_REPOSITORY ("owner/repo") into org and repo.
func OrgRepoFromEnv() (org, repo string, err error) {
	s := os.Getenv("GITHUB_REPOSITORY")
	if s == "" {
		return "", "", fmt.Errorf("GITHUB_REPOSITORY is not set")
	}
	parts := strings.SplitN(s, "/", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid GITHUB_REPOSITORY: %q", s)
	}
	return parts[0], parts[1], nil
}

// ActionsRunURL returns the URL of the current GitHub Actions run.
func ActionsRunURL() string {
	serverURL := os.Getenv("GITHUB_SERVER_URL")
	if serverURL == "" {
		serverURL = "https://github.com"
	}
	repo := os.Getenv("GITHUB_REPOSITORY")
	runID := os.Getenv("GITHUB_RUN_ID")
	if repo == "" || runID == "" {
		return ""
	}
	return serverURL + "/" + repo + "/actions/runs/" + runID
}

// ContextFromComment builds a Context from an IssueCommentEvent.
func ContextFromComment(evt *CommentEvent, org, repo, botLogin string) *Context {
	sc := &Context{
		Org:           org,
		Repo:          repo,
		BotLogin:      botLogin,
		ActionsRunURL: ActionsRunURL(),
	}
	if evt.Comment != nil {
		if evt.Comment.ID != nil {
			sc.CommentID = *evt.Comment.ID
		}
		if evt.Comment.User != nil && evt.Comment.User.Login != nil {
			sc.Author = *evt.Comment.User.Login
		}
	}
	if evt.Issue != nil && evt.Issue.Number != nil {
		sc.IssueNumber = *evt.Issue.Number
	}
	return sc
}
