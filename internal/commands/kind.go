package commands

import (
	"context"
	"slices"

	"github.com/elevran/stern/internal/config"
	"github.com/elevran/stern/internal/event"
	"github.com/elevran/stern/internal/github"
)

// kindClient is the minimum Client surface KindHandler uses.
type kindClient interface {
	github.LabelsClient
	github.PullRequestsClient
}

// KindHandler handles /kind <value>. Multiple values may coexist on a PR.
type KindHandler struct {
	labelMutatingBase // provides Post and opts
	ghc               kindClient
}

// NewKindHandler constructs a KindHandler with all dependencies injected.
func NewKindHandler(_ *event.Context, ghc github.Client, opts *config.Options) Handler {
	return &KindHandler{
		labelMutatingBase: labelMutatingBase{mergeGHC: ghc, opts: opts},
		ghc:               ghc,
	}
}

func (h *KindHandler) Pre(_ context.Context, sc *event.Context, args []string) error {
	if sc.PR == nil {
		return PermissionError("/kind may only be used on pull requests")
	}
	if len(args) == 0 {
		return PermissionError("usage: /kind <value>")
	}
	if !slices.Contains(h.opts.Kind.Values, args[0]) {
		return PermissionError("invalid kind %q (must be one of: %v)", args[0], h.opts.Kind.Values)
	}
	return nil
}

func (h *KindHandler) Handle(ctx context.Context, sc *event.Context, args []string) error {
	return h.ghc.AddLabels(ctx, sc.Org, sc.Repo, sc.IssueNumber, []string{"kind/" + args[0]})
}
