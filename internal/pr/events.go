package pr

import (
	"context"
	"slices"
	"strings"

	"github.com/sirupsen/logrus"

	"github.com/elevran/stern/internal/config"
	"github.com/elevran/stern/internal/event"
	"github.com/elevran/stern/internal/github"
	"github.com/elevran/stern/internal/labels"
	"github.com/elevran/stern/internal/merge"
)

// prClient is the minimum Client surface the PR event handlers use.
// It mirrors the parts of github.Client needed by EventHandler,
// HandlePREventSize, and HandlePREventReviewAssignment.
type prClient interface {
	github.LabelsClient
	github.PullRequestsClient
	github.UsersClient
	github.CommentsClient
	github.ContentClient
}

// EventHandler routes pull_request_target webhook events to the
// appropriate sub-handlers (WIP label, size label, LGTM/approve
// invalidation, reviewer assignment). It holds the GitHub client and
// resolved config so individual sub-handlers can be invoked with narrow
// signatures.
type EventHandler struct {
	ghc  prClient
	opts *config.Options
}

// NewEventHandler constructs a handler bound to the given GitHub client
// and resolved configuration.
func NewEventHandler(ghc prClient, opts *config.Options) *EventHandler {
	return &EventHandler{ghc: ghc, opts: opts}
}

// Handle dispatches a pull_request_target event to the appropriate sub-handlers.
func (h *EventHandler) Handle(ctx context.Context, org, repo string, evt *event.PREvent) error {
	action := evt.GetAction()

	log := logrus.WithFields(logrus.Fields{
		"org":    org,
		"repo":   repo,
		"pr":     evt.GetNumber(),
		"action": action,
	})

	sender := evt.GetSender().GetLogin()
	if event.IsBot(sender, h.opts.BotLogin) {
		log.WithField("sender", sender).Info("pr-event: skipping bot-triggered event")
		return nil
	}

	rawPR := evt.GetPullRequest()
	if rawPR == nil {
		log.Warn("pr-event: no pull request in payload")
		return nil
	}
	p := github.PullRequestFromGH(rawPR)

	switch action {
	case "opened", "reopened", "synchronize":
		if err := h.handleWIP(ctx, org, repo, p); err != nil {
			log.WithError(err).Warn("WIP detection failed")
		}
		if err := HandlePREventSize(ctx, h.ghc, org, repo, p, h.opts); err != nil {
			log.WithError(err).Warn("size labeling failed")
		}
	case "edited":
		if evt.Changes != nil && evt.Changes.Title != nil {
			if err := h.handleWIP(ctx, org, repo, p); err != nil {
				log.WithError(err).Warn("WIP detection on title edit failed")
			}
		}
	}

	if action == "synchronize" {
		if err := h.invalidateLGTMOnPush(ctx, org, repo, p); err != nil {
			log.WithError(err).Warn("LGTM invalidation failed")
		}
		if err := h.invalidateApproveOnPush(ctx, org, repo, p); err != nil {
			log.WithError(err).Warn("approve invalidation failed")
		}
	}

	// Review assignment runs only on the initial open of a PR, not on
	// reopened or synchronize.
	if action == "opened" {
		if err := HandlePREventReviewAssignment(ctx, h.ghc, org, repo, p, h.opts); err != nil {
			log.WithError(err).Warn("review assignment failed")
		}
	}

	log.Info("pr-event processed")
	return nil
}

// IsTitleWIP reports whether a PR title indicates WIP status.
func IsTitleWIP(title string) bool {
	t := strings.TrimSpace(title)
	lower := strings.ToLower(t)
	for _, p := range []string{"wip:", "[wip]", "[draft]", "draft:"} {
		if strings.HasPrefix(lower, p) {
			return true
		}
	}
	return false
}

// handleWIP applies or removes the WIP label based on PR title and draft state.
func (h *EventHandler) handleWIP(ctx context.Context, org, repo string, p github.PullRequest) error {
	shouldHaveWIP := IsTitleWIP(p.Title) || p.IsDraft
	currentWIP := slices.ContainsFunc(p.Labels, func(l string) bool { return strings.EqualFold(l, labels.WIP) })
	number := p.Number

	if shouldHaveWIP && !currentWIP {
		if err := h.ghc.AddLabels(ctx, org, repo, number, []string{labels.WIP}); err != nil {
			return err
		}
		return merge.DisableAutoMerge(ctx, h.ghc, p.NodeID)
	}

	if !shouldHaveWIP && currentWIP {
		if err := h.ghc.RemoveLabel(ctx, org, repo, number, labels.WIP); err != nil && !github.IsNotFoundError(err) {
			return err
		}
		freshPR, err := h.ghc.GetPullRequest(ctx, org, repo, number)
		if err != nil {
			return err
		}
		return merge.CheckAndApplyAutoMerge(ctx, h.ghc, freshPR, h.opts)
	}

	return nil
}

// invalidateLGTMOnPush removes the lgtm label when a PR receives new commits.
func (h *EventHandler) invalidateLGTMOnPush(ctx context.Context, org, repo string, p github.PullRequest) error {
	if !h.opts.LGTM.InvalidateOnPush {
		return nil
	}
	if err := h.ghc.RemoveLabel(ctx, org, repo, p.Number, labels.LGTM); err != nil && !github.IsNotFoundError(err) {
		return err
	}
	return merge.DisableAutoMerge(ctx, h.ghc, p.NodeID)
}

// invalidateApproveOnPush removes the approved label when a PR receives new commits.
func (h *EventHandler) invalidateApproveOnPush(ctx context.Context, org, repo string, p github.PullRequest) error {
	if !h.opts.Approve.InvalidateOnPush {
		return nil
	}
	if err := h.ghc.RemoveLabel(ctx, org, repo, p.Number, labels.Approved); err != nil && !github.IsNotFoundError(err) {
		return err
	}
	return merge.DisableAutoMerge(ctx, h.ghc, p.NodeID)
}

// HandlePREvent dispatches a pull_request_target event to the appropriate handlers.
//
// Deprecated: construct an EventHandler with NewEventHandler and call its
// Handle method. This wrapper is retained for callers that have not migrated.
func HandlePREvent(ctx context.Context, ghc github.Client, org, repo string, evt *event.PREvent, opts *config.Options) error {
	return NewEventHandler(ghc, opts).Handle(ctx, org, repo, evt)
}

// HandlePREventWIP applies or removes the WIP label based on PR title and draft state.
//
// Deprecated: call (*EventHandler).handleWIP via an EventHandler instead.
func HandlePREventWIP(ctx context.Context, ghc prClient, org, repo string, p github.PullRequest, opts *config.Options) error {
	return NewEventHandler(ghc, opts).handleWIP(ctx, org, repo, p)
}

// InvalidateLGTMOnPush removes the lgtm label when a PR receives new commits.
//
// Deprecated: call (*EventHandler).invalidateLGTMOnPush via an EventHandler instead.
func InvalidateLGTMOnPush(ctx context.Context, ghc prClient, org, repo string, p github.PullRequest, opts *config.Options) error {
	return NewEventHandler(ghc, opts).invalidateLGTMOnPush(ctx, org, repo, p)
}

// InvalidateApproveOnPush removes the approved label when a PR receives new commits.
//
// Deprecated: call (*EventHandler).invalidateApproveOnPush via an EventHandler instead.
func InvalidateApproveOnPush(ctx context.Context, ghc prClient, org, repo string, p github.PullRequest, opts *config.Options) error {
	return NewEventHandler(ghc, opts).invalidateApproveOnPush(ctx, org, repo, p)
}
