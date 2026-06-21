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
type prClient interface {
	github.LabelsClient
	github.PullRequestsClient
}

// HandlePREvent dispatches a pull_request_target event to the appropriate handlers.
func HandlePREvent(ctx context.Context, ghc prClient, org, repo string, evt *event.PREvent, opts *config.Options) error {
	action := evt.GetAction()

	log := logrus.WithFields(logrus.Fields{
		"org":    org,
		"repo":   repo,
		"pr":     evt.GetNumber(),
		"action": action,
	})

	sender := evt.GetSender().GetLogin()
	if strings.HasSuffix(sender, "[bot]") || sender == opts.BotLogin {
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
		if err := HandlePREventWIP(ctx, ghc, org, repo, p, opts); err != nil {
			log.WithError(err).Warn("WIP detection failed")
		}
	case "edited":
		if evt.Changes != nil && evt.Changes.Title != nil {
			if err := HandlePREventWIP(ctx, ghc, org, repo, p, opts); err != nil {
				log.WithError(err).Warn("WIP detection on title edit failed")
			}
		}
	}

	if action == "synchronize" {
		if err := InvalidateLGTMOnPush(ctx, ghc, org, repo, p, opts); err != nil {
			log.WithError(err).Warn("LGTM invalidation failed")
		}
		if err := InvalidateApproveOnPush(ctx, ghc, org, repo, p, opts); err != nil {
			log.WithError(err).Warn("approve invalidation failed")
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

// HandlePREventWIP applies or removes the WIP label based on PR title and draft state.
func HandlePREventWIP(ctx context.Context, ghc prClient, org, repo string, p github.PullRequest, opts *config.Options) error {
	shouldHaveWIP := IsTitleWIP(p.Title) || p.IsDraft
	currentWIP := slices.ContainsFunc(p.Labels, func(l string) bool { return strings.EqualFold(l, labels.WIP) })
	number := p.Number

	if shouldHaveWIP && !currentWIP {
		if err := ghc.AddLabels(ctx, org, repo, number, []string{labels.WIP}); err != nil {
			return err
		}
		return merge.DisableAutoMerge(ctx, ghc, p.NodeID)
	}

	if !shouldHaveWIP && currentWIP {
		if err := ghc.RemoveLabel(ctx, org, repo, number, labels.WIP); err != nil && !github.IsNotFoundError(err) {
			return err
		}
		freshPR, err := ghc.GetPullRequest(ctx, org, repo, number)
		if err != nil {
			return err
		}
		return merge.CheckAndApplyAutoMerge(ctx, ghc, freshPR, opts)
	}

	return nil
}

// InvalidateLGTMOnPush removes the lgtm label when a PR receives new commits.
func InvalidateLGTMOnPush(ctx context.Context, ghc prClient, org, repo string, p github.PullRequest, opts *config.Options) error {
	if !opts.LGTM.InvalidateOnPush {
		return nil
	}
	if err := ghc.RemoveLabel(ctx, org, repo, p.Number, labels.LGTM); err != nil && !github.IsNotFoundError(err) {
		return err
	}
	return merge.DisableAutoMerge(ctx, ghc, p.NodeID)
}

// InvalidateApproveOnPush removes the approved label when a PR receives new commits.
func InvalidateApproveOnPush(ctx context.Context, ghc prClient, org, repo string, p github.PullRequest, opts *config.Options) error {
	if !opts.Approve.InvalidateOnPush {
		return nil
	}
	if err := ghc.RemoveLabel(ctx, org, repo, p.Number, labels.Approved); err != nil && !github.IsNotFoundError(err) {
		return err
	}
	return merge.DisableAutoMerge(ctx, ghc, p.NodeID)
}
