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

func (h *ApproveHandler) Pre(ctx context.Context, sc *event.Context, args []string) error {
	if sc.PR == nil {
		return PermissionError("/approve may only be used on pull requests")
	}
	if isCancel(args) {
		return nil
	}
	if !h.opts.Approve.AllowSelfApproval && sc.PR.Author == sc.Author {
		return PermissionError("you cannot approve your own pull request")
	}
	return h.checkApproveOwners(ctx, sc)
}

func (h *ApproveHandler) Handle(ctx context.Context, sc *event.Context, args []string) error {
	if isCancel(args) {
		if err := h.ghc.RemoveLabel(ctx, sc.Org, sc.Repo, sc.IssueNumber, labels.Approved); err != nil && !github.IsNotFoundError(err) {
			return err
		}
		return nil
	}

	return h.ghc.AddLabels(ctx, sc.Org, sc.Repo, sc.IssueNumber, []string{labels.Approved})
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
	uncovered, err := owners.UncoveredFiles(ctx, h.ghc, sc.Org, sc.Repo, sc.PR.HeadSHA, sc.Author, files)
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
