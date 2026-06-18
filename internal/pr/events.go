package pr

import (
	"context"
	"slices"
	"strings"

	gh "github.com/google/go-github/v72/github"
	"github.com/sirupsen/logrus"

	"github.com/elevran/stern/internal/config"
	ghclient "github.com/elevran/stern/internal/github"
	"github.com/elevran/stern/internal/labels"
	"github.com/elevran/stern/internal/merge"
)

// HandlePREvent dispatches a pull_request_target event to the appropriate handlers.
func HandlePREvent(ctx context.Context, ghc ghclient.Client, org, repo string, evt *gh.PullRequestEvent, opts *config.Options) error {
	action := evt.GetAction()
	pr := evt.GetPullRequest()

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

	if pr == nil {
		log.Warn("pr-event: no pull request in payload")
		return nil
	}

	switch action {
	case "opened", "reopened", "synchronize":
		if err := HandlePREventWIP(ctx, ghc, org, repo, pr, opts); err != nil {
			log.WithError(err).Warn("WIP detection failed")
		}
	case "edited":
		titleChanged := evt.Changes != nil && evt.Changes.Title != nil
		if titleChanged {
			if err := HandlePREventWIP(ctx, ghc, org, repo, pr, opts); err != nil {
				log.WithError(err).Warn("WIP detection on title edit failed")
			}
		}
	}

	if action == "synchronize" {
		if err := InvalidateLGTMOnPush(ctx, ghc, org, repo, pr, opts); err != nil {
			log.WithError(err).Warn("LGTM invalidation failed")
		}
		if err := InvalidateApproveOnPush(ctx, ghc, org, repo, pr, opts); err != nil {
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
func HandlePREventWIP(ctx context.Context, ghc ghclient.Client, org, repo string, pr *gh.PullRequest, opts *config.Options) error {
	shouldHaveWIP := IsTitleWIP(pr.GetTitle()) || pr.GetDraft()
	currentWIP := slices.ContainsFunc(pr.Labels, func(l *gh.Label) bool { return strings.EqualFold(l.GetName(), labels.WIP) })
	number := pr.GetNumber()

	if shouldHaveWIP && !currentWIP {
		if err := ghc.AddLabels(ctx, org, repo, number, []string{labels.WIP}); err != nil {
			return err
		}
		return ghc.DisableAutoMerge(ctx, pr.GetNodeID())
	}

	if !shouldHaveWIP && currentWIP {
		if err := ghc.RemoveLabel(ctx, org, repo, number, labels.WIP); err != nil && !merge.IsNotFoundError(err) {
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
func InvalidateLGTMOnPush(ctx context.Context, ghc ghclient.Client, org, repo string, pr *gh.PullRequest, opts *config.Options) error {
	if !opts.LGTM.InvalidateOnPush {
		return nil
	}
	number := pr.GetNumber()
	if err := ghc.RemoveLabel(ctx, org, repo, number, labels.LGTM); err != nil && !merge.IsNotFoundError(err) {
		return err
	}
	return ghc.DisableAutoMerge(ctx, pr.GetNodeID())
}

// InvalidateApproveOnPush removes the approved label when a PR receives new commits.
func InvalidateApproveOnPush(ctx context.Context, ghc ghclient.Client, org, repo string, pr *gh.PullRequest, opts *config.Options) error {
	if !opts.Approve.InvalidateOnPush {
		return nil
	}
	number := pr.GetNumber()
	if err := ghc.RemoveLabel(ctx, org, repo, number, labels.Approved); err != nil && !merge.IsNotFoundError(err) {
		return err
	}
	return ghc.DisableAutoMerge(ctx, pr.GetNodeID())
}
