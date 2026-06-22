package commands

import (
	"context"

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
	github.PermissionsClient
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

// Pre enforces that /lgtm is used on a PR, that the caller is not the PR
// author (unless allow_self_lgtm), and that the caller is in OWNERS
// reviewers (with a write-access fallback when no OWNERS covers the changed
// files). /lgtm cancel requires write access for any caller.
func (h *LGTMHandler) Pre(ctx context.Context, sc *event.Context, args []string) error {
	if sc.PR == nil {
		return PermissionError("/lgtm may only be used on pull requests")
	}
	if isCancel(args) {
		// Removing an existing LGTM requires write access (mirrors /hold cancel).
		ok, err := h.ghc.HasWriteAccess(ctx, sc.Org, sc.Repo, sc.Author)
		if err != nil {
			return err
		}
		if !ok {
			return PermissionError("%s does not have write access to remove an LGTM", sc.Author)
		}
		return nil
	}
	if !h.opts.LGTM.AllowSelfLGTM && sc.PR.Author == sc.Author {
		return PermissionError("you cannot LGTM your own pull request")
	}
	return h.checkLGTMOwners(ctx, sc)
}

// Handle adds the lgtm label (or removes it on cancel, treating 404 as success).
func (h *LGTMHandler) Handle(ctx context.Context, sc *event.Context, args []string) error {
	if isCancel(args) {
		if err := h.ghc.RemoveLabel(ctx, sc.Org, sc.Repo, sc.IssueNumber, labels.LGTM); err != nil && !github.IsNotFoundError(err) {
			return err
		}
		return nil
	}

	return h.ghc.AddLabels(ctx, sc.Org, sc.Repo, sc.IssueNumber, []string{labels.LGTM})
}

func (h *LGTMHandler) checkLGTMOwners(ctx context.Context, sc *event.Context) error {
	files, err := h.ghc.ListPullRequestFiles(ctx, sc.Org, sc.Repo, sc.IssueNumber)
	if err != nil {
		return err
	}
	return checkOwners(ctx, sc, h.ghc, files, func(r *owners.ResolvedOwners) bool {
		return r.IsReviewer(sc.Author) || r.IsApprover(sc.Author)
	}, "%s is not in the OWNERS reviewers list for this PR's changed files")
}
