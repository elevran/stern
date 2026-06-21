package commands

import (
	"context"
	"slices"
	"strings"

	"github.com/elevran/stern/internal/config"
	"github.com/elevran/stern/internal/event"
	"github.com/elevran/stern/internal/github"
	"github.com/elevran/stern/internal/labels"
)

// wipClient is the minimum Client surface WIPHandler uses.
type wipClient interface {
	github.LabelsClient
	github.PullRequestsClient
}

// WIPHandler handles /wip (toggle).
type WIPHandler struct {
	labelMutatingBase // provides Post and opts
	ghc               wipClient
}

// NewWIPHandler constructs a WIPHandler with all dependencies injected.
func NewWIPHandler(_ *event.Context, ghc github.Client, opts *config.Options) Handler {
	return &WIPHandler{
		labelMutatingBase: labelMutatingBase{mergeGHC: ghc, opts: opts},
		ghc:               ghc,
	}
}

func (h *WIPHandler) Pre(_ context.Context, sc *event.Context, _ []string) error {
	if sc.PR == nil {
		return PermissionError("/wip may only be used on pull requests")
	}
	return nil
}

func (h *WIPHandler) Handle(ctx context.Context, sc *event.Context, _ []string) error {
	hasWIP := slices.ContainsFunc(sc.PR.Labels, func(l string) bool { return strings.EqualFold(l, labels.WIP) })
	if hasWIP {
		if err := h.ghc.RemoveLabel(ctx, sc.Org, sc.Repo, sc.IssueNumber, labels.WIP); err != nil && !github.IsNotFoundError(err) {
			return err
		}
		return nil
	}

	return h.ghc.AddLabels(ctx, sc.Org, sc.Repo, sc.IssueNumber, []string{labels.WIP})
}
