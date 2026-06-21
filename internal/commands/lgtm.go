package commands

import (
	"context"
	"strings"

	"github.com/elevran/stern/internal/config"
	"github.com/elevran/stern/internal/event"
	"github.com/elevran/stern/internal/github"
	"github.com/elevran/stern/internal/labels"
	"github.com/elevran/stern/internal/owners"
)

// lgtmClient is the minimum Client surface LGTMHandler uses.
type lgtmClient interface {
	github.LabelsClient
	github.PullRequestsClient
	github.ContentClient
}

// LGTMHandler handles /lgtm and /lgtm cancel.
type LGTMHandler struct {
	labelMutatingBase // provides Post and opts
	ghc               lgtmClient
}

// NewLGTMHandler constructs a LGTMHandler with all dependencies injected.
func NewLGTMHandler(_ *event.Context, ghc github.Client, opts *config.Options) Handler {
	return &LGTMHandler{
		labelMutatingBase: labelMutatingBase{mergeGHC: ghc, opts: opts},
		ghc:               ghc,
	}
}

func (h *LGTMHandler) Pre(ctx context.Context, sc *event.Context, args []string) error {
	if sc.PR == nil {
		return PermissionError("/lgtm may only be used on pull requests")
	}
	if len(args) > 0 && strings.EqualFold(args[0], "cancel") {
		return nil
	}
	if !h.opts.LGTM.AllowSelfLGTM && sc.PR.Author == sc.Author {
		return PermissionError("you cannot LGTM your own pull request")
	}
	return h.checkLGTMOwners(ctx, sc)
}

func (h *LGTMHandler) Handle(ctx context.Context, sc *event.Context, args []string) error {
	if len(args) > 0 && strings.EqualFold(args[0], "cancel") {
		if err := h.ghc.RemoveLabel(ctx, sc.Org, sc.Repo, sc.IssueNumber, labels.LGTM); err != nil && !github.IsNotFoundError(err) {
			return err
		}
		return nil
	}

	return h.ghc.AddLabels(ctx, sc.Org, sc.Repo, sc.IssueNumber, []string{labels.LGTM})
}

func (h *LGTMHandler) checkLGTMOwners(ctx context.Context, sc *event.Context) error {
	if sc.PR.HeadSHA == "" {
		return nil
	}
	files, err := h.ghc.ListPullRequestFiles(ctx, sc.Org, sc.Repo, sc.IssueNumber)
	if err != nil {
		return err
	}
	resolved, err := owners.LoadForPaths(ctx, h.ghc, sc.Org, sc.Repo, sc.PR.HeadSHA, files)
	if err != nil {
		return err
	}
	if !resolved.HasOwners() {
		return nil
	}
	if !resolved.IsReviewer(sc.Author) && !resolved.IsApprover(sc.Author) {
		return PermissionError("%s is not in the OWNERS reviewers list for this PR's changed files", sc.Author)
	}
	return nil
}
