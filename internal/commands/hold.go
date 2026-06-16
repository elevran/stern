package commands

import (
	"context"
	"strings"

	"github.com/elevran/stern/internal/config"
	"github.com/elevran/stern/internal/event"
	ghclient "github.com/elevran/stern/internal/github"
	"github.com/elevran/stern/internal/labels"
	"github.com/elevran/stern/internal/merge"
)

// HoldHandler handles /hold and /hold cancel.
type HoldHandler struct{}

func (h *HoldHandler) Handle(ctx context.Context, sc *event.Context, args []string, ghc ghclient.Client, opts *config.Options) error {
	if sc.PR == nil {
		return PermissionError("/hold may only be used on pull requests")
	}

	cancel := len(args) > 0 && strings.EqualFold(args[0], "cancel")
	if cancel {
		return holdCancel(ctx, sc, ghc, opts)
	}
	return holdAdd(ctx, sc, ghc)
}

func holdAdd(ctx context.Context, sc *event.Context, ghc ghclient.Client) error {
	if err := ghc.AddLabels(ctx, sc.Org, sc.Repo, sc.IssueNumber, []string{labels.Hold}); err != nil {
		return err
	}
	return merge.DisableAutoMerge(ctx, ghc, sc.Org, sc.Repo, sc.IssueNumber)
}

func holdCancel(ctx context.Context, sc *event.Context, ghc ghclient.Client, opts *config.Options) error {
	// Cancel requires write access (original commenter could also cancel, but
	// we use write access as the gate since we don't track who placed the hold).
	ok, err := ghc.HasWriteAccess(ctx, sc.Org, sc.Repo, sc.Author)
	if err != nil {
		return err
	}
	if !ok {
		return PermissionError("%s does not have write access to remove a hold", sc.Author)
	}

	if err := ghc.RemoveLabel(ctx, sc.Org, sc.Repo, sc.IssueNumber, labels.Hold); err != nil && !isLabelNotFound(err) {
		return err
	}

	pr, err := ghc.GetPullRequest(ctx, sc.Org, sc.Repo, sc.IssueNumber)
	if err != nil {
		return err
	}
	return merge.CheckAndApplyAutoMerge(ctx, ghc, pr, sc.Org, sc.Repo, opts)
}
