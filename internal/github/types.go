package github

import (
	"errors"
	"net/http"

	gh "github.com/google/go-github/v72/github"
)

// PullRequest is a slim representation of a GitHub pull request.
type PullRequest struct {
	Number    int
	Author    string
	Title     string
	IsDraft   bool
	Labels    []string // label names only
	NodeID    string
	HeadSHA   string
	Additions int // lines added in the PR diff
	Deletions int // lines removed in the PR diff
}

// Label represents a GitHub repository label.
type Label struct {
	Name        string
	Color       string
	Description string
}

// Milestone represents a GitHub repository milestone.
type Milestone struct {
	Number int
	Title  string
}

// CheckRun represents a single GitHub check run on a ref.
type CheckRun struct {
	ID         int64
	Name       string
	Conclusion string // "failure", "timed_out", "cancelled", "action_required", "success", "skipped", "neutral", ""
}

// IsNotFoundError reports whether err is a 404 from the GitHub API.
func IsNotFoundError(err error) bool {
	var ghErr *gh.ErrorResponse
	return errors.As(err, &ghErr) && ghErr.Response != nil &&
		ghErr.Response.StatusCode == http.StatusNotFound
}

// PullRequestFromGH converts a go-github PullRequest to the internal type.
func PullRequestFromGH(pr *gh.PullRequest) PullRequest {
	if pr == nil {
		return PullRequest{}
	}
	labels := make([]string, 0, len(pr.Labels))
	for _, l := range pr.Labels {
		labels = append(labels, l.GetName())
	}
	var headSHA, author string
	if pr.Head != nil {
		headSHA = pr.Head.GetSHA()
	}
	if pr.User != nil {
		author = pr.User.GetLogin()
	}
	return PullRequest{
		Number:    pr.GetNumber(),
		Author:    author,
		Title:     pr.GetTitle(),
		IsDraft:   pr.GetDraft(),
		Labels:    labels,
		NodeID:    pr.GetNodeID(),
		HeadSHA:   headSHA,
		Additions: pr.GetAdditions(),
		Deletions: pr.GetDeletions(),
	}
}

// labelToGH converts an internal Label to a go-github Label pointer for API calls.
func labelToGH(l Label) *gh.Label {
	return &gh.Label{
		Name:        gh.Ptr(l.Name),
		Color:       gh.Ptr(l.Color),
		Description: gh.Ptr(l.Description),
	}
}

// labelFromGH converts a go-github Label to the internal type.
func labelFromGH(l *gh.Label) Label {
	return Label{
		Name:        l.GetName(),
		Color:       l.GetColor(),
		Description: l.GetDescription(),
	}
}
