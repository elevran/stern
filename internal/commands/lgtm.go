package commands

import (
	"context"
	"strings"

	gh "github.com/google/go-github/v72/github"

	"github.com/elevran/stern/internal/config"
	"github.com/elevran/stern/internal/event"
	ghclient "github.com/elevran/stern/internal/github"
	"github.com/elevran/stern/internal/labels"
	"github.com/elevran/stern/internal/merge"
	"github.com/elevran/stern/internal/owners"
)

// LGTMHandler handles /lgtm and /lgtm cancel.
type LGTMHandler struct{}

func (h *LGTMHandler) Handle(ctx context.Context, sc *event.Context, args []string, ghc ghclient.Client, opts *config.Options) error {
	if sc.PR == nil {
		return PermissionError("/lgtm may only be used on pull requests")
	}

	cancel := len(args) > 0 && strings.EqualFold(args[0], "cancel")

	if cancel {
		return lgtmCancel(ctx, sc, ghc, opts)
	}
	return lgtmAdd(ctx, sc, ghc, opts)
}

func lgtmAdd(ctx context.Context, sc *event.Context, ghc ghclient.Client, opts *config.Options) error {
	// Self-LGTM check.
	if !opts.LGTM.AllowSelfLGTM {
		if sc.PR.User != nil && sc.PR.User.GetLogin() == sc.Author {
			return PermissionError("you cannot LGTM your own pull request")
		}
	}

	// OWNERS check — only if OWNERS files exist for the changed paths.
	if err := checkLGTMOwners(ctx, sc, ghc, opts); err != nil {
		return err
	}

	if err := ghc.AddLabels(ctx, sc.Org, sc.Repo, sc.IssueNumber, []string{labels.LGTM}); err != nil {
		return err
	}

	// Refresh the PR to get updated labels, then check eligibility.
	pr, err := ghc.GetPullRequest(ctx, sc.Org, sc.Repo, sc.IssueNumber)
	if err != nil {
		return err
	}
	return merge.CheckAndApplyAutoMerge(ctx, ghc, pr, sc.Org, sc.Repo, opts)
}

func lgtmCancel(ctx context.Context, sc *event.Context, ghc ghclient.Client, opts *config.Options) error {
	if err := ghc.RemoveLabel(ctx, sc.Org, sc.Repo, sc.IssueNumber, labels.LGTM); err != nil && !isLabelNotFound(err) {
		return err
	}
	return merge.DisableAutoMerge(ctx, ghc, sc.Org, sc.Repo, sc.IssueNumber)
}

// checkLGTMOwners verifies the commenter is allowed to LGTM based on OWNERS.
func checkLGTMOwners(ctx context.Context, sc *event.Context, ghc ghclient.Client, opts *config.Options) error {
	if sc.PR.Head == nil {
		return nil
	}
	ref := sc.PR.Head.GetSHA()

	// Get changed files to find relevant OWNERS.
	files, err := ghc.ListPullRequestFiles(ctx, sc.Org, sc.Repo, sc.IssueNumber)
	if err != nil {
		return err
	}
	changedPaths := fileNames(files)

	resolved, err := owners.LoadForPaths(ctx, ghc, sc.Org, sc.Repo, ref, changedPaths)
	if err != nil {
		return err
	}

	if !resolved.HasOwners() {
		// No OWNERS files: any commenter is accepted.
		return nil
	}

	if !resolved.IsReviewer(sc.Author) && !resolved.IsApprover(sc.Author) {
		return PermissionError("%s is not in the OWNERS reviewers list for this PR's changed files", sc.Author)
	}
	return nil
}

// InvalidateLGTMOnPush removes the lgtm label when a PR receives new commits.
// Called from the pr-event handler on "synchronize" events.
func InvalidateLGTMOnPush(ctx context.Context, ghc ghclient.Client, org, repo string, pr *gh.PullRequest, opts *config.Options) error {
	if !opts.LGTM.InvalidateOnPush {
		return nil
	}
	number := pr.GetNumber()
	if err := ghc.RemoveLabel(ctx, org, repo, number, labels.LGTM); err != nil && !isLabelNotFound(err) {
		return err
	}
	return merge.DisableAutoMerge(ctx, ghc, org, repo, number)
}

func fileNames(files []*gh.CommitFile) []string {
	names := make([]string, len(files))
	for i, f := range files {
		names[i] = f.GetFilename()
	}
	return names
}

// isLabelNotFound returns true when the GitHub API returns 404 for a label
// removal (label was already absent — treat as success).
func isLabelNotFound(err error) bool {
	return merge.IsNotFoundError(err)
}
