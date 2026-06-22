package github

import (
	"errors"
	"net/http"
	"time"

	gh "github.com/google/go-github/v72/github"
)

// PullRequest is a slim representation of a GitHub pull request.
type PullRequest struct {
	Number         int
	Author         string
	Title          string
	IsDraft        bool
	Labels         []string // label names only
	NodeID         string
	HeadSHA        string
	BaseSHA        string // SHA of the base branch (trusted target ref for OWNERS lookups)
	BaseRef        string // name of the base branch (e.g. "main"); for logging and fallback
	Additions      int    // lines added in the PR diff
	Deletions      int    // lines removed in the PR diff
	Merged         bool   // true when the PR is merged (set by GetPullRequest on merged PRs)
	MergeCommitSHA string // SHA of the merge commit; empty until merged
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

// Item is a slim representation of a GitHub issue or pull request, used by
// the lifecycle sweep. Both kinds surface through the Issues API; IsPR
// distinguishes them via the pull_request key on the underlying response.
type Item struct {
	Number       int
	Labels       []string
	UpdatedAt    time.Time
	IsPR         bool
	HasMilestone bool // true when the item has a milestone set
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
	var headSHA, baseSHA, baseRef, author string
	if pr.Head != nil {
		headSHA = pr.Head.GetSHA()
	}
	if pr.Base != nil {
		baseSHA = pr.Base.GetSHA()
		baseRef = pr.Base.GetRef()
	}
	if pr.User != nil {
		author = pr.User.GetLogin()
	}
	return PullRequest{
		Number:         pr.GetNumber(),
		Author:         author,
		Title:          pr.GetTitle(),
		IsDraft:        pr.GetDraft(),
		Labels:         labels,
		NodeID:         pr.GetNodeID(),
		HeadSHA:        headSHA,
		BaseSHA:        baseSHA,
		BaseRef:        baseRef,
		Additions:      pr.GetAdditions(),
		Deletions:      pr.GetDeletions(),
		Merged:         pr.GetMerged(),
		MergeCommitSHA: pr.GetMergeCommitSHA(),
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
