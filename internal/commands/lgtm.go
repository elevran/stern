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
	"github.com/elevran/stern/internal/owners"
	"github.com/elevran/stern/internal/permissions"
)

// LGTMHandler handles /lgtm and /lgtm cancel.
type LGTMHandler struct {
	nopPost
	ghc     ghclient.Client
	opts    *config.Options
	checker permissions.Checker
}

// NewLGTMHandler constructs a LGTMHandler with all dependencies injected.
func NewLGTMHandler(sc *event.Context, ghc ghclient.Client, opts *config.Options) Handler {
	return &LGTMHandler{
		ghc:     ghc,
		opts:    opts,
		checker: permissions.New(ghc, sc),
	}
}

func (h *LGTMHandler) Pre(ctx context.Context, sc *event.Context, args []string) error {
	if sc.PR == nil {
		return PermissionError("/lgtm may only be used on pull requests")
	}
	if len(args) > 0 && strings.EqualFold(args[0], "cancel") {
		return nil
	}
	if !h.opts.LGTM.AllowSelfLGTM && h.checker.IsPRAuthor(sc.PR, sc.Author) {
		return PermissionError("you cannot LGTM your own pull request")
	}
	return h.checkLGTMOwners(ctx, sc)
}

func (h *LGTMHandler) Handle(ctx context.Context, sc *event.Context, args []string) error {
	if len(args) > 0 && strings.EqualFold(args[0], "cancel") {
		if err := h.ghc.RemoveLabel(ctx, sc.Org, sc.Repo, sc.IssueNumber, labels.LGTM); err != nil && !isLabelNotFound(err) {
			return err
		}
		return merge.DisableAutoMerge(ctx, h.ghc, sc.Org, sc.Repo, sc.IssueNumber)
	}

	if err := h.ghc.AddLabels(ctx, sc.Org, sc.Repo, sc.IssueNumber, []string{labels.LGTM}); err != nil {
		return err
	}
	pr, err := h.ghc.GetPullRequest(ctx, sc.Org, sc.Repo, sc.IssueNumber)
	if err != nil {
		return err
	}
	return merge.CheckAndApplyAutoMerge(ctx, h.ghc, pr, sc.Org, sc.Repo, h.opts)
}

func (h *LGTMHandler) checkLGTMOwners(ctx context.Context, sc *event.Context) error {
	if sc.PR.Head == nil {
		return nil
	}
	files, err := h.ghc.ListPullRequestFiles(ctx, sc.Org, sc.Repo, sc.IssueNumber)
	if err != nil {
		return err
	}
	resolved, err := owners.LoadForPaths(ctx, h.ghc, sc.Org, sc.Repo, sc.PR.Head.GetSHA(), fileNames(files))
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

// fileNames extracts filenames from a list of commit files.
func fileNames(files []*gh.CommitFile) []string {
	names := make([]string, len(files))
	for i, f := range files {
		names[i] = f.GetFilename()
	}
	return names
}

// isLabelNotFound returns true when the GitHub API returns 404 on label removal.
func isLabelNotFound(err error) bool {
	return merge.IsNotFoundError(err)
}
