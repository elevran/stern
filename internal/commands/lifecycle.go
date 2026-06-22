package commands

import (
	"context"

	"github.com/elevran/stern/internal/config"
	"github.com/elevran/stern/internal/event"
	"github.com/elevran/stern/internal/github"
	"github.com/elevran/stern/internal/labels"
)

// lifecycleClient is the minimum Client surface LifecycleHandler uses.
type lifecycleClient interface {
	github.LabelsClient
}

// LifecycleHandler handles /lifecycle stale|rotten|frozen|active.
// Each subcommand sets (or clears) one of the lifecycle/* labels and removes
// the other two so the lifecycle state is mutually exclusive.
type LifecycleHandler struct {
	ghc  lifecycleClient
	opts *config.Options
}

// NewLifecycleHandler constructs a LifecycleHandler with all dependencies injected.
func NewLifecycleHandler(_ *event.Context, ghc github.Client, opts *config.Options) Handler {
	return &LifecycleHandler{ghc: ghc, opts: opts}
}

// Pre enforces that the lifecycle plugin is enabled and validates the
// subcommand argument (stale|rotten|frozen|active).
func (h *LifecycleHandler) Pre(_ context.Context, _ *event.Context, args []string) error {
	if !h.opts.Lifecycle.Enabled {
		return PermissionError("lifecycle plugin is not enabled")
	}
	if len(args) == 0 {
		return PermissionError("usage: /lifecycle stale|rotten|frozen|active")
	}
	switch args[0] {
	case "stale", "rotten", "frozen", "active":
		return nil
	default:
		return PermissionError("unknown lifecycle subcommand %q; use stale, rotten, frozen, or active", args[0])
	}
}

// Handle sets the matching lifecycle/* label and removes the others.
// "active" clears all three labels — there is no lifecycle/active label.
func (h *LifecycleHandler) Handle(ctx context.Context, sc *event.Context, args []string) error {
	switch args[0] {
	case "stale":
		return h.setLifecycle(ctx, sc, labels.LifecycleStale,
			labels.LifecycleRotten, labels.LifecycleFrozen)
	case "rotten":
		return h.setLifecycle(ctx, sc, labels.LifecycleRotten,
			labels.LifecycleStale, labels.LifecycleFrozen)
	case "frozen":
		return h.setLifecycle(ctx, sc, labels.LifecycleFrozen,
			labels.LifecycleStale, labels.LifecycleRotten)
	case "active":
		return h.clearAll(ctx, sc)
	}
	return nil
}

// Post is a no-op: lifecycle labels do not feed into auto-merge eligibility.
func (h *LifecycleHandler) Post(_ context.Context, _ *event.Context, _ []string, _ error) error {
	return nil
}

// setLifecycle adds toAdd and removes the listed conflicting labels.
// Missing labels are tolerated (IsNotFoundError suppressed) so the command
// stays idempotent.
func (h *LifecycleHandler) setLifecycle(ctx context.Context, sc *event.Context, toAdd string, remove ...string) error {
	for _, l := range remove {
		if err := h.ghc.RemoveLabel(ctx, sc.Org, sc.Repo, sc.IssueNumber, l); err != nil && !github.IsNotFoundError(err) {
			return err
		}
	}
	return h.ghc.AddLabels(ctx, sc.Org, sc.Repo, sc.IssueNumber, []string{toAdd})
}

// clearAll removes all lifecycle/* labels. There is no "lifecycle/active"
// label — "active" is the state where none of the three labels are present.
func (h *LifecycleHandler) clearAll(ctx context.Context, sc *event.Context) error {
	for _, l := range []string{labels.LifecycleStale, labels.LifecycleRotten, labels.LifecycleFrozen} {
		if err := h.ghc.RemoveLabel(ctx, sc.Org, sc.Repo, sc.IssueNumber, l); err != nil && !github.IsNotFoundError(err) {
			return err
		}
	}
	return nil
}
