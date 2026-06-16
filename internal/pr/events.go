package pr

import (
	"context"

	gh "github.com/google/go-github/v72/github"
	"github.com/sirupsen/logrus"

	"github.com/elevran/stern/internal/commands"
	"github.com/elevran/stern/internal/config"
	ghclient "github.com/elevran/stern/internal/github"
)

// HandlePREvent dispatches a pull_request_target event to the appropriate handlers.
// evt is the parsed PullRequestEvent from GITHUB_EVENT_PATH.
func HandlePREvent(ctx context.Context, ghc ghclient.Client, org, repo string, evt *gh.PullRequestEvent, opts *config.Options) error {
	action := evt.GetAction()
	pr := evt.GetPullRequest()

	log := logrus.WithFields(logrus.Fields{
		"org":    org,
		"repo":   repo,
		"pr":     evt.GetNumber(),
		"action": action,
	})

	if pr == nil {
		log.Warn("pr-event: no pull request in payload")
		return nil
	}

	// WIP detection runs on all relevant actions.
	switch action {
	case "opened", "reopened", "synchronize":
		if err := commands.HandlePREventWIP(ctx, ghc, org, repo, pr, opts, false); err != nil {
			log.WithError(err).Warn("WIP detection failed")
		}
	case "edited":
		// Only re-evaluate WIP when the title actually changed.
		titleChanged := evt.Changes != nil && evt.Changes.Title != nil
		if titleChanged {
			if err := commands.HandlePREventWIP(ctx, ghc, org, repo, pr, opts, true); err != nil {
				log.WithError(err).Warn("WIP detection on title edit failed")
			}
		}
	}

	// Label invalidation on new commits.
	if action == "synchronize" {
		if err := commands.InvalidateLGTMOnPush(ctx, ghc, org, repo, pr, opts); err != nil {
			log.WithError(err).Warn("LGTM invalidation failed")
		}
		if err := commands.InvalidateApproveOnPush(ctx, ghc, org, repo, pr, opts); err != nil {
			log.WithError(err).Warn("approve invalidation failed")
		}
	}

	log.Info("pr-event processed")
	return nil
}
