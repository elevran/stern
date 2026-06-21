// Package lifecycle implements the scheduled sweep that advances open
// issues and pull requests through the stale → rotten → closed pipeline
// based on inactivity.
//
// The sweep uses the GitHub Issues API's updated_at as the inactivity
// timer: any activity (comment, label change, commit) bumps updated_at,
// and adding a lifecycle label also bumps it, naturally restarting the
// clock when the bot transitions an item. This matches the behaviour of
// Prow-derived lifecycle bots and avoids needing a separate timestamp
// store. A more precise approach using the Issues Events API could
// replace this later if needed.
package lifecycle

import (
	"context"
	"time"

	"slices"

	"github.com/sirupsen/logrus"

	"github.com/elevran/stern/internal/config"
	"github.com/elevran/stern/internal/github"
	"github.com/elevran/stern/internal/labels"
)

// sweepClient is the minimum Client surface the sweep uses.
type sweepClient interface {
	github.LifecycleClient
	github.LabelsClient
	github.CommentsClient
	github.IssueStateClient
}

// Sweep processes all open items in the repository and advances each one's
// lifecycle state. A single item failure is logged and skipped; remaining
// items are still processed. An error from ListOpenItems is fatal and
// returned to the caller.
//
// now is passed in so callers can supply a deterministic timestamp (the
// production caller passes time.Now()).
func Sweep(ctx context.Context, ghc sweepClient, org, repo string, opts *config.Options, now time.Time) error {
	items, err := ghc.ListOpenItems(ctx, org, repo)
	if err != nil {
		return err
	}
	for _, item := range items {
		if err := processItem(ctx, ghc, org, repo, item, opts, now); err != nil {
			logrus.WithError(err).
				WithFields(logrus.Fields{"org": org, "repo": repo, "number": item.Number}).
				Warn("lifecycle: failed to process item")
			// Continue with remaining items — partial progress is preferable
			// to an early abort on a single broken issue.
		}
	}
	return nil
}

// processItem evaluates one item and applies at most one transition.
// Items with lifecycle/frozen are skipped; PRs with a milestone (in-flight
// work) are also skipped.
func processItem(ctx context.Context, ghc sweepClient, org, repo string, item github.Item, opts *config.Options, now time.Time) error {
	var cfg config.LifecycleItemOptions
	if item.IsPR {
		cfg = opts.Lifecycle.ForPRs()
	} else {
		cfg = opts.Lifecycle.ForIssues()
	}

	if slices.Contains(item.Labels, labels.LifecycleFrozen) {
		return nil
	}
	if item.IsPR && item.HasMilestone {
		return nil
	}

	inactive := now.Sub(item.UpdatedAt)

	switch {
	case slices.Contains(item.Labels, labels.LifecycleRotten):
		if cfg.CloseAfter > 0 && inactive >= days(cfg.CloseAfter) {
			return closeItem(ctx, ghc, org, repo, item, cfg.CloseComment)
		}

	case slices.Contains(item.Labels, labels.LifecycleStale):
		// In close_stale mode, stale items skip rotten and close directly
		// once they reach the rotten threshold.
		if inactive >= days(cfg.RottenDays) {
			if opts.Lifecycle.CloseStale {
				return closeItem(ctx, ghc, org, repo, item, cfg.CloseComment)
			}
			return transitionToRotten(ctx, ghc, org, repo, item, cfg.RottenComment)
		}

	default:
		if inactive >= days(cfg.StaleDays) {
			return transitionToStale(ctx, ghc, org, repo, item, cfg.StaleComment)
		}
	}
	return nil
}

func transitionToStale(ctx context.Context, ghc sweepClient, org, repo string, item github.Item, comment string) error {
	if err := ghc.AddLabels(ctx, org, repo, item.Number, []string{labels.LifecycleStale}); err != nil {
		return err
	}
	return postIfSet(ctx, ghc, org, repo, item.Number, comment)
}

func transitionToRotten(ctx context.Context, ghc sweepClient, org, repo string, item github.Item, comment string) error {
	// Best-effort removal: tolerate 404 (label already gone) or any other
	// error to keep the transition forward-progressing.
	if err := ghc.RemoveLabel(ctx, org, repo, item.Number, labels.LifecycleStale); err != nil && !github.IsNotFoundError(err) {
		logrus.WithError(err).
			WithFields(logrus.Fields{"number": item.Number, "label": labels.LifecycleStale}).
			Warn("lifecycle: best-effort RemoveLabel failed; continuing transition")
	}
	if err := ghc.AddLabels(ctx, org, repo, item.Number, []string{labels.LifecycleRotten}); err != nil {
		return err
	}
	return postIfSet(ctx, ghc, org, repo, item.Number, comment)
}

func closeItem(ctx context.Context, ghc sweepClient, org, repo string, item github.Item, comment string) error {
	if err := postIfSet(ctx, ghc, org, repo, item.Number, comment); err != nil {
		return err
	}
	return ghc.CloseIssue(ctx, org, repo, item.Number)
}

// postIfSet creates a comment only when body is non-empty. An empty
// template suppresses the user-facing notification for a transition.
func postIfSet(ctx context.Context, ghc sweepClient, org, repo string, number int, body string) error {
	if body == "" {
		return nil
	}
	return ghc.CreateIssueComment(ctx, org, repo, number, body)
}

func days(n int) time.Duration { return time.Duration(n) * 24 * time.Hour }
