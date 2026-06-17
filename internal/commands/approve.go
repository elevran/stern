package commands

import (
	"context"
	"strings"

	"github.com/elevran/stern/internal/config"
	"github.com/elevran/stern/internal/event"
	ghclient "github.com/elevran/stern/internal/github"
	"github.com/elevran/stern/internal/labels"
	"github.com/elevran/stern/internal/merge"
	"github.com/elevran/stern/internal/owners"
	"github.com/elevran/stern/internal/permissions"
)

// ApproveHandler handles /approve and /approve cancel.
type ApproveHandler struct {
	nopPost
	ghc     ghclient.Client
	opts    *config.Options
	checker permissions.Checker
}

// NewApproveHandler constructs an ApproveHandler with all dependencies injected.
func NewApproveHandler(sc *event.Context, ghc ghclient.Client, opts *config.Options) Handler {
	return &ApproveHandler{
		ghc:     ghc,
		opts:    opts,
		checker: permissions.New(ghc, sc),
	}
}

func (h *ApproveHandler) Pre(ctx context.Context, sc *event.Context, args []string) error {
	if sc.PR == nil {
		return PermissionError("/approve may only be used on pull requests")
	}
	if len(args) > 0 && strings.EqualFold(args[0], "cancel") {
		return nil
	}
	if !h.opts.Approve.AllowSelfApproval && h.checker.IsPRAuthor(sc.PR, sc.Author) {
		return PermissionError("you cannot approve your own pull request")
	}
	return h.checkApproveOwners(ctx, sc)
}

func (h *ApproveHandler) Handle(ctx context.Context, sc *event.Context, args []string) error {
	if len(args) > 0 && strings.EqualFold(args[0], "cancel") {
		if err := h.ghc.RemoveLabel(ctx, sc.Org, sc.Repo, sc.IssueNumber, labels.Approved); err != nil && !isLabelNotFound(err) {
			return err
		}
		return merge.DisableAutoMerge(ctx, h.ghc, sc.Org, sc.Repo, sc.IssueNumber)
	}

	if err := h.ghc.AddLabels(ctx, sc.Org, sc.Repo, sc.IssueNumber, []string{labels.Approved}); err != nil {
		return err
	}
	pr, err := h.ghc.GetPullRequest(ctx, sc.Org, sc.Repo, sc.IssueNumber)
	if err != nil {
		return err
	}
	return merge.CheckAndApplyAutoMerge(ctx, h.ghc, pr, sc.Org, sc.Repo, h.opts)
}

func (h *ApproveHandler) checkApproveOwners(ctx context.Context, sc *event.Context) error {
	if !h.opts.Approve.RequireOwner {
		return nil
	}
	if sc.PR.Head == nil {
		return nil
	}
	files, err := h.ghc.ListPullRequestFiles(ctx, sc.Org, sc.Repo, sc.IssueNumber)
	if err != nil {
		return err
	}
	resolved, err := owners.LoadForPaths(ctx, h.ghc, sc.Org, sc.Repo, sc.PR.Head.GetSHA(), fileNames(files))
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
