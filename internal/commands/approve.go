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

// ApproveHandler handles /approve and /approve cancel.
type ApproveHandler struct{}

func (h *ApproveHandler) Handle(ctx context.Context, sc *event.Context, args []string, ghc ghclient.Client, opts *config.Options) error {
	if sc.PR == nil {
		return PermissionError("/approve may only be used on pull requests")
	}

	cancel := len(args) > 0 && strings.EqualFold(args[0], "cancel")
	if cancel {
		return approveCancel(ctx, sc, ghc, opts)
	}
	return approveAdd(ctx, sc, ghc, opts)
}

func approveAdd(ctx context.Context, sc *event.Context, ghc ghclient.Client, opts *config.Options) error {
	// Self-approval check.
	if !opts.Approve.AllowSelfApproval {
		if sc.PR.User != nil && sc.PR.User.GetLogin() == sc.Author {
			return PermissionError("you cannot approve your own pull request")
		}
	}

	// OWNERS check.
	if err := checkApproveOwners(ctx, sc, ghc, opts); err != nil {
		return err
	}

	if err := ghc.AddLabels(ctx, sc.Org, sc.Repo, sc.IssueNumber, []string{labels.Approved}); err != nil {
		return err
	}

	pr, err := ghc.GetPullRequest(ctx, sc.Org, sc.Repo, sc.IssueNumber)
	if err != nil {
		return err
	}
	return merge.CheckAndApplyAutoMerge(ctx, ghc, pr, sc.Org, sc.Repo, opts)
}

func approveCancel(ctx context.Context, sc *event.Context, ghc ghclient.Client, opts *config.Options) error {
	if err := ghc.RemoveLabel(ctx, sc.Org, sc.Repo, sc.IssueNumber, labels.Approved); err != nil && !isLabelNotFound(err) {
		return err
	}
	return merge.DisableAutoMerge(ctx, ghc, sc.Org, sc.Repo, sc.IssueNumber)
}

func checkApproveOwners(ctx context.Context, sc *event.Context, ghc ghclient.Client, opts *config.Options) error {
	if !opts.Approve.RequireOwner {
		return nil
	}
	if sc.PR.Head == nil {
		return nil
	}
	ref := sc.PR.Head.GetSHA()

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
		return nil
	}

	if !resolved.IsApprover(sc.Author) {
		return PermissionError("%s is not in the OWNERS approvers list for this PR's changed files", sc.Author)
	}
	return nil
}

// InvalidateApproveOnPush removes the approved label when a PR receives new commits.
func InvalidateApproveOnPush(ctx context.Context, ghc ghclient.Client, org, repo string, pr *gh.PullRequest, opts *config.Options) error {
	if !opts.Approve.InvalidateOnPush {
		return nil
	}
	number := pr.GetNumber()
	if err := ghc.RemoveLabel(ctx, org, repo, number, labels.Approved); err != nil && !isLabelNotFound(err) {
		return err
	}
	return merge.DisableAutoMerge(ctx, ghc, org, repo, number)
}
