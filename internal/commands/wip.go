package commands

import (
	"context"
	"strings"

	gh "github.com/google/go-github/v72/github"

	"github.com/elevran/stern/internal/config"
	"github.com/elevran/stern/internal/event"
	ghclient "github.com/elevran/stern/internal/github"
	"github.com/elevran/stern/internal/labels"
	"github.com/elevran/stern/internal/merge"
)

// WIPHandler handles /wip (toggle).
type WIPHandler struct{}

func (h *WIPHandler) Handle(ctx context.Context, sc *event.Context, args []string, ghc ghclient.Client, opts *config.Options) error {
	if sc.PR == nil {
		return PermissionError("/wip may only be used on pull requests")
	}

	hasWIP := prHasLabel(sc.PR, labels.WIP)
	if hasWIP {
		// Remove WIP label.
		if err := ghc.RemoveLabel(ctx, sc.Org, sc.Repo, sc.IssueNumber, labels.WIP); err != nil && !isLabelNotFound(err) {
			return err
		}
		pr, err := ghc.GetPullRequest(ctx, sc.Org, sc.Repo, sc.IssueNumber)
		if err != nil {
			return err
		}
		return merge.CheckAndApplyAutoMerge(ctx, ghc, pr, sc.Org, sc.Repo, opts)
	}

	// Add WIP label.
	if err := ghc.AddLabels(ctx, sc.Org, sc.Repo, sc.IssueNumber, []string{labels.WIP}); err != nil {
		return err
	}
	return merge.DisableAutoMerge(ctx, ghc, sc.Org, sc.Repo, sc.IssueNumber)
}

// IsTitleWIP reports whether a PR title indicates WIP status.
func IsTitleWIP(title string) bool {
	t := strings.TrimSpace(title)
	prefixes := []string{"wip:", "[wip]", "[draft]", "draft:"}
	lower := strings.ToLower(t)
	for _, p := range prefixes {
		if strings.HasPrefix(lower, p) {
			return true
		}
	}
	return false
}

// HandlePREventWIP applies or removes the WIP label based on PR title and draft state.
// Called from the pr-event handler on opened/edited/synchronize/reopened events.
// titleChanged should be true only when the title actually changed (on "edited" events).
func HandlePREventWIP(ctx context.Context, ghc ghclient.Client, org, repo string, pr *gh.PullRequest, opts *config.Options, _ bool) error {
	// Determine WIP state from title.
	titleWIP := IsTitleWIP(pr.GetTitle())
	draftWIP := pr.GetDraft()
	shouldHaveWIP := titleWIP || draftWIP

	currentWIP := prHasLabel(pr, labels.WIP)
	number := pr.GetNumber()

	if shouldHaveWIP && !currentWIP {
		if err := ghc.AddLabels(ctx, org, repo, number, []string{labels.WIP}); err != nil {
			return err
		}
		return merge.DisableAutoMerge(ctx, ghc, org, repo, number)
	}

	if !shouldHaveWIP && currentWIP {
		if err := ghc.RemoveLabel(ctx, org, repo, number, labels.WIP); err != nil && !isLabelNotFound(err) {
			return err
		}
		freshPR, err := ghc.GetPullRequest(ctx, org, repo, number)
		if err != nil {
			return err
		}
		return merge.CheckAndApplyAutoMerge(ctx, ghc, freshPR, org, repo, opts)
	}

	return nil
}

func prHasLabel(pr *gh.PullRequest, labelName string) bool {
	for _, l := range pr.Labels {
		if strings.EqualFold(l.GetName(), labelName) {
			return true
		}
	}
	return false
}
