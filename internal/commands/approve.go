package commands

import (
	"context"
	"fmt"
	"strings"

	"github.com/sirupsen/logrus"

	"github.com/elevran/stern/internal/config"
	"github.com/elevran/stern/internal/event"
	"github.com/elevran/stern/internal/github"
	"github.com/elevran/stern/internal/labels"
	"github.com/elevran/stern/internal/owners"
	"github.com/elevran/stern/internal/pr"
)

// approveClient is the minimum Client surface ApproveHandler uses.
type approveClient interface {
	github.LabelsClient
	github.PullRequestsClient
	github.ContentClient
	github.ReviewsClient
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
		h.dismissBotReview(ctx, sc, "approval cancelled via /approve cancel")
		return nil
	}

	if err := h.ghc.AddLabels(ctx, sc.Org, sc.Repo, sc.IssueNumber, []string{labels.Approved}); err != nil {
		return err
	}
	h.submitBotReview(ctx, sc)
	return nil
}

// submitBotReview creates an APPROVE review from the bot if the bot has not
// already approved. Review errors are non-fatal and logged.
func (h *ApproveHandler) submitBotReview(ctx context.Context, sc *event.Context) {
	if h.opts.BotLogin == "" {
		return
	}
	reviews, err := h.ghc.ListPullRequestReviews(ctx, sc.Org, sc.Repo, sc.IssueNumber)
	if err != nil {
		logrus.WithError(err).Warn("approve: list reviews failed")
		return
	}
	if pr.FindBotApprovedReview(reviews, h.opts.BotLogin) != nil {
		return
	}
	body := fmt.Sprintf("Approved via /approve by @%s", sc.Author)
	if err := h.ghc.CreatePullRequestReview(ctx, sc.Org, sc.Repo, sc.IssueNumber, "APPROVE", body); err != nil {
		logrus.WithError(err).Warn("approve: create review failed")
	}
}

// dismissBotReview dismisses the bot's own APPROVED review if present.
func (h *ApproveHandler) dismissBotReview(ctx context.Context, sc *event.Context, msg string) {
	if h.opts.BotLogin == "" {
		return
	}
	reviews, err := h.ghc.ListPullRequestReviews(ctx, sc.Org, sc.Repo, sc.IssueNumber)
	if err != nil {
		logrus.WithError(err).Warn("approve: list reviews failed")
		return
	}
	r := pr.FindBotApprovedReview(reviews, h.opts.BotLogin)
	if r == nil {
		return
	}
	if err := h.ghc.DismissPullRequestReview(ctx, sc.Org, sc.Repo, sc.IssueNumber, r.ID, msg); err != nil {
		logrus.WithError(err).Warn("approve: dismiss review failed")
	}
}

func (h *ApproveHandler) checkApproveOwners(ctx context.Context, sc *event.Context) error {
	if !h.opts.Approve.RequireOwner {
		return nil
	}
	return checkOwners(ctx, sc, h.ghc, func(r *owners.ResolvedOwners) bool {
		return r.IsApprover(sc.Author)
	}, "%s is not in the OWNERS approvers list for this PR's changed files")
}
