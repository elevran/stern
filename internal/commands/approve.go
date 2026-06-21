package commands

import (
	"context"
	"strings"

	"github.com/elevran/stern/internal/config"
	"github.com/elevran/stern/internal/event"
	"github.com/elevran/stern/internal/github"
	"github.com/elevran/stern/internal/labels"
	"github.com/elevran/stern/internal/merge"
	"github.com/elevran/stern/internal/owners"
)

// ApproveHandler handles /approve and /approve cancel.
type ApproveHandler struct {
	nopPost
	ghc  github.Client
	opts *config.Options
}

// NewApproveHandler constructs an ApproveHandler with all dependencies injected.
func NewApproveHandler(_ *event.Context, ghc github.Client, opts *config.Options) Handler {
	return &ApproveHandler{ghc: ghc, opts: opts}
}

func (h *ApproveHandler) Pre(ctx context.Context, sc *event.Context, args []string) error {
	if sc.PR == nil {
		return PermissionError("/approve may only be used on pull requests")
	}
	if len(args) > 0 && strings.EqualFold(args[0], "cancel") {
		return nil
	}
	if !h.opts.Approve.AllowSelfApproval && sc.PR.Author == sc.Author {
		return PermissionError("you cannot approve your own pull request")
	}
	return h.checkApproveOwners(ctx, sc)
}

func (h *ApproveHandler) Handle(ctx context.Context, sc *event.Context, args []string) error {
	if len(args) > 0 && strings.EqualFold(args[0], "cancel") {
		if err := h.ghc.RemoveLabel(ctx, sc.Org, sc.Repo, sc.IssueNumber, labels.Approved); err != nil && !github.IsNotFoundError(err) {
			return err
		}
		pr, err := h.ghc.GetPullRequest(ctx, sc.Org, sc.Repo, sc.IssueNumber)
		if err != nil {
			return err
		}
		return merge.CheckAndApplyAutoMerge(ctx, h.ghc, pr, h.opts)
	}

	if err := h.ghc.AddLabels(ctx, sc.Org, sc.Repo, sc.IssueNumber, []string{labels.Approved}); err != nil {
		return err
	}
	pr, err := h.ghc.GetPullRequest(ctx, sc.Org, sc.Repo, sc.IssueNumber)
	if err != nil {
		return err
	}
	return merge.CheckAndApplyAutoMerge(ctx, h.ghc, pr, h.opts)
}

func (h *ApproveHandler) checkApproveOwners(ctx context.Context, sc *event.Context) error {
	if !h.opts.Approve.RequireOwner {
		return nil
	}
	if sc.PR.HeadSHA == "" {
		return nil
	}
	files, err := h.ghc.ListPullRequestFiles(ctx, sc.Org, sc.Repo, sc.IssueNumber)
	if err != nil {
		return err
	}
	resolved, err := owners.LoadForPaths(ctx, h.ghc, sc.Org, sc.Repo, sc.PR.HeadSHA, files)
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
