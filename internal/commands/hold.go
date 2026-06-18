package commands

import (
	"context"
	"strings"

	"github.com/elevran/stern/internal/config"
	"github.com/elevran/stern/internal/event"
	"github.com/elevran/stern/internal/github"
	"github.com/elevran/stern/internal/labels"
	"github.com/elevran/stern/internal/merge"
)

// HoldHandler handles /hold and /hold cancel.
type HoldHandler struct {
	nopPost
	ghc  github.Client
	opts *config.Options
}

// NewHoldHandler constructs a HoldHandler with all dependencies injected.
func NewHoldHandler(_ *event.Context, ghc github.Client, opts *config.Options) Handler {
	return &HoldHandler{ghc: ghc, opts: opts}
}

func (h *HoldHandler) Pre(ctx context.Context, sc *event.Context, args []string) error {
	if sc.PR == nil {
		return PermissionError("/hold may only be used on pull requests")
	}
	if len(args) > 0 && strings.EqualFold(args[0], "cancel") {
		ok, err := h.ghc.HasWriteAccess(ctx, sc.Org, sc.Repo, sc.Author)
		if err != nil {
			return err
		}
		if !ok {
			return PermissionError("%s does not have write access to remove a hold", sc.Author)
		}
	}
	return nil
}

func (h *HoldHandler) Handle(ctx context.Context, sc *event.Context, args []string) error {
	if len(args) > 0 && strings.EqualFold(args[0], "cancel") {
		if err := h.ghc.RemoveLabel(ctx, sc.Org, sc.Repo, sc.IssueNumber, labels.Hold); err != nil && !github.IsNotFoundError(err) {
			return err
		}
		pr, err := h.ghc.GetPullRequest(ctx, sc.Org, sc.Repo, sc.IssueNumber)
		if err != nil {
			return err
		}
		return merge.CheckAndApplyAutoMerge(ctx, h.ghc, pr, h.opts)
	}

	if err := h.ghc.AddLabels(ctx, sc.Org, sc.Repo, sc.IssueNumber, []string{labels.Hold}); err != nil {
		return err
	}
	return merge.DisableAutoMerge(ctx, h.ghc, sc.PR.NodeID)
}
