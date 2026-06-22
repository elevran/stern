package commands

import (
	"context"
	"slices"
	"strings"

	"github.com/elevran/stern/internal/config"
	"github.com/elevran/stern/internal/event"
	"github.com/elevran/stern/internal/github"
)

// priorityClient is the minimum Client surface PriorityHandler uses.
type priorityClient interface {
	github.LabelsClient
	github.PullRequestsClient
}

// PriorityHandler handles /priority <value> and /priority cancel.
// Only one priority/* label may be set at a time (mutual exclusion).
type PriorityHandler struct {
	labelMutatingBase // provides Post and opts
	ghc               priorityClient
}

// NewPriorityHandler constructs a PriorityHandler with all dependencies injected.
func NewPriorityHandler(_ *event.Context, ghc github.Client, opts *config.Options) Handler {
	return &PriorityHandler{
		labelMutatingBase: labelMutatingBase{mergeGHC: ghc, opts: opts},
		ghc:               ghc,
	}
}

// Pre enforces that /priority is used on a PR and validates the value
// argument (or "cancel"/empty, both of which are always allowed).
func (h *PriorityHandler) Pre(_ context.Context, sc *event.Context, args []string) error {
	if sc.PR == nil {
		return PermissionError("/priority may only be used on pull requests")
	}
	// cancel / no arg is always valid (removes all priority/*)
	if len(args) == 0 || strings.EqualFold(args[0], "cancel") {
		return nil
	}
	if !slices.Contains(h.opts.Priority.Values, args[0]) {
		return PermissionError("invalid priority %q (must be one of: %v)", args[0], h.opts.Priority.Values)
	}
	return nil
}

// Handle strips every existing priority/* label then adds the new
// priority/<value> label (or none, if "cancel"/empty). Mutual exclusion:
// at most one priority/* label is on a PR after this returns.
func (h *PriorityHandler) Handle(ctx context.Context, sc *event.Context, args []string) error {
	if err := h.removeAllPriorityLabels(ctx, sc); err != nil {
		return err
	}
	if len(args) == 0 || strings.EqualFold(args[0], "cancel") {
		return nil
	}
	return h.ghc.AddLabels(ctx, sc.Org, sc.Repo, sc.IssueNumber, []string{"priority/" + args[0]})
}

// removeAllPriorityLabels strips every "priority/*" label currently on the PR.
// Uses sc.PR.Labels (captured at dispatch time) to know which to remove.
func (h *PriorityHandler) removeAllPriorityLabels(ctx context.Context, sc *event.Context) error {
	for _, l := range sc.PR.Labels {
		if !strings.HasPrefix(l, "priority/") {
			continue
		}
		if err := h.ghc.RemoveLabel(ctx, sc.Org, sc.Repo, sc.IssueNumber, l); err != nil && !github.IsNotFoundError(err) {
			return err
		}
	}
	return nil
}
