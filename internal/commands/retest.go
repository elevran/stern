package commands

import (
	"context"

	"github.com/elevran/stern/internal/config"
	"github.com/elevran/stern/internal/event"
	"github.com/elevran/stern/internal/github"
)

// retestClient is the minimum Client surface RetestHandler uses.
type retestClient interface {
	github.PermissionsClient
	github.ChecksClient
	github.CommentsClient
}

// RetestHandler handles /retest — re-runs failed check runs on the PR's head SHA.
type RetestHandler struct {
	ghc retestClient
}

// NewRetestHandler constructs a RetestHandler with all dependencies injected.
func NewRetestHandler(_ *event.Context, ghc github.Client, _ *config.Options) Handler {
	return &RetestHandler{ghc: ghc}
}

func (h *RetestHandler) Pre(ctx context.Context, sc *event.Context, _ []string) error {
	if sc.PR == nil {
		return PermissionError("/retest may only be used on pull requests")
	}
	ok, err := h.ghc.HasWriteAccess(ctx, sc.Org, sc.Repo, sc.Author)
	if err != nil {
		return err
	}
	if !ok {
		return PermissionError("%s does not have write access to /retest", sc.Author)
	}
	return nil
}

func (h *RetestHandler) Handle(ctx context.Context, sc *event.Context, _ []string) error {
	failed, err := h.ghc.ListFailedCheckRuns(ctx, sc.Org, sc.Repo, sc.PR.HeadSHA)
	if err != nil {
		return err
	}
	if len(failed) == 0 {
		return h.ghc.CreateIssueComment(ctx, sc.Org, sc.Repo, sc.IssueNumber,
			"No failed checks to re-run.")
	}
	for _, run := range failed {
		if err := h.ghc.RerunCheckRun(ctx, sc.Org, sc.Repo, run.ID); err != nil {
			return err
		}
	}
	return nil
}

func (h *RetestHandler) Post(_ context.Context, _ *event.Context, _ []string, _ error) error {
	return nil
}
