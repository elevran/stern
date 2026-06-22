package commands

import (
	"context"
	"strings"

	"github.com/elevran/stern/internal/config"
	"github.com/elevran/stern/internal/event"
	"github.com/elevran/stern/internal/github"
	"github.com/elevran/stern/internal/labels"
	"github.com/elevran/stern/internal/owners"
)

// approveClient is the minimum Client surface ApproveHandler uses.
type approveClient interface {
	github.LabelsClient
	github.PullRequestsClient
	github.ContentClient
	github.PermissionsClient
}

// ApproveHandler handles /approve and /approve cancel.
type ApproveHandler struct {
	labelMutatingBase // provides Post and opts
	ghc               approveClient
}

// NewApproveHandler constructs an ApproveHandler with all dependencies injected.
func NewApproveHandler(_ *event.Context, ghc github.Client, opts *config.Options) Handler {
	return &ApproveHandler{
		labelMutatingBase: labelMutatingBase{mergeGHC: ghc, opts: opts},
		ghc:               ghc,
	}
}

// Pre enforces that /approve is used on a PR, that the caller is not the PR
// author (unless allow_self_approval), and that the caller is in OWNERS
// approvers (with a write-access fallback when no OWNERS covers the changed files).
// /approve cancel is allowed for any caller with write access.
func (h *ApproveHandler) Pre(ctx context.Context, sc *event.Context, args []string) error {
	if sc.PR == nil {
		return PermissionError("/approve may only be used on pull requests")
	}
	if isCancel(args) {
		// Removing an existing approval requires write access (mirrors /hold cancel).
		ok, err := h.ghc.HasWriteAccess(ctx, sc.Org, sc.Repo, sc.Author)
		if err != nil {
			return err
		}
		if !ok {
			return PermissionError("%s does not have write access to remove an approval", sc.Author)
		}
		return nil
	}
	if !h.opts.Approve.AllowSelfApproval && sc.PR.Author == sc.Author {
		return PermissionError("you cannot approve your own pull request")
	}
	return h.checkApproveOwners(ctx, sc)
}

// Handle adds the approved label (or removes it on cancel, treating 404 as success).
func (h *ApproveHandler) Handle(ctx context.Context, sc *event.Context, args []string) error {
	if isCancel(args) {
		if err := h.ghc.RemoveLabel(ctx, sc.Org, sc.Repo, sc.IssueNumber, labels.Approved); err != nil && !github.IsNotFoundError(err) {
			return err
		}
		return nil
	}

	return h.ghc.AddLabels(ctx, sc.Org, sc.Repo, sc.IssueNumber, []string{labels.Approved})
}

// checkApproveOwners runs two layered checks:
//  1. The shared checkOwners handles the base-ref OWNERS load and the
//     write-access fallback when no OWNERS covers the changed paths.
//  2. owners.UncoveredFiles then enforces per-file coverage (M6 model):
//     a known approver must also be an approver for every changed file.
func (h *ApproveHandler) checkApproveOwners(ctx context.Context, sc *event.Context) error {
	files, err := h.ghc.ListPullRequestFiles(ctx, sc.Org, sc.Repo, sc.IssueNumber)
	if err != nil {
		return err
	}
	if err := checkOwners(ctx, sc, h.ghc, files, func(r *owners.ResolvedOwners) bool {
		return r.IsApprover(sc.Author)
	}, "%s is not in the OWNERS approvers list for this PR's changed files"); err != nil {
		return err
	}
	uncovered, err := owners.UncoveredFiles(ctx, h.ghc, sc.Org, sc.Repo, sc.PR.BaseSHA, sc.Author, files)
	if err != nil {
		return err
	}
	if len(uncovered) == 0 {
		return nil
	}
	return PermissionError(
		"%s does not have approval authority over all changed files. Uncovered: %s",
		sc.Author, strings.Join(uncovered, ", "),
	)
}
