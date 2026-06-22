package commands

import (
	"context"
	"slices"

	"github.com/elevran/stern/internal/config"
	"github.com/elevran/stern/internal/event"
	"github.com/elevran/stern/internal/github"
)

// areaClient is the minimum Client surface AreaHandler uses.
type areaClient interface {
	github.LabelsClient
	github.PullRequestsClient
}

// AreaHandler handles /area <value>. Multiple values may coexist on a PR.
type AreaHandler struct {
	labelMutatingBase // provides Post and opts
	ghc               areaClient
}

// NewAreaHandler constructs an AreaHandler with all dependencies injected.
func NewAreaHandler(_ *event.Context, ghc github.Client, opts *config.Options) Handler {
	return &AreaHandler{
		labelMutatingBase: labelMutatingBase{mergeGHC: ghc, opts: opts},
		ghc:               ghc,
	}
}

// Pre enforces that /area is used on a PR with a single argument that matches
// one of the configured Area.Values.
func (h *AreaHandler) Pre(_ context.Context, sc *event.Context, args []string) error {
	if sc.PR == nil {
		return PermissionError("/area may only be used on pull requests")
	}
	if len(args) == 0 {
		return PermissionError("usage: /area <value>")
	}
	if !slices.Contains(h.opts.Area.Values, args[0]) {
		return PermissionError("invalid area %q (must be one of: %v)", args[0], h.opts.Area.Values)
	}
	return nil
}

// Handle adds the area/<value> label to the PR. Multiple areas may coexist
// (additive; no removal).
func (h *AreaHandler) Handle(ctx context.Context, sc *event.Context, args []string) error {
	return h.ghc.AddLabels(ctx, sc.Org, sc.Repo, sc.IssueNumber, []string{"area/" + args[0]})
}
