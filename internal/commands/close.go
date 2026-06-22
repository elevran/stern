package commands

import (
	"context"

	"github.com/elevran/stern/internal/config"
	"github.com/elevran/stern/internal/event"
	"github.com/elevran/stern/internal/github"
)

// closeClient is the minimum Client surface CloseHandler uses.
type closeClient interface {
	github.PermissionsClient
	github.IssueStateClient
}

// CloseHandler handles /close and /reopen. Works on both issues and PRs.
type CloseHandler struct {
	verb string // "close" or "reopen"
	ghc  closeClient
}

// newCloseHandler returns a HandlerFactory that produces a CloseHandler bound
// to the given verb.
func newCloseHandler(verb string) HandlerFactory {
	return func(_ *event.Context, ghc github.Client, _ *config.Options) Handler {
		return &CloseHandler{verb: verb, ghc: ghc}
	}
}

// Pre enforces that /close and /reopen require repo write access.
// Works on both issues and PRs.
func (h *CloseHandler) Pre(ctx context.Context, sc *event.Context, _ []string) error {
	ok, err := h.ghc.HasWriteAccess(ctx, sc.Org, sc.Repo, sc.Author)
	if err != nil {
		return err
	}
	if !ok {
		return PermissionError("%s does not have write access to %s issues", sc.Author, h.verb)
	}
	return nil
}

// Handle calls CloseIssue (or ReopenIssue for the reopen verb) on the issue or PR.
func (h *CloseHandler) Handle(ctx context.Context, sc *event.Context, _ []string) error {
	if h.verb == "reopen" {
		return h.ghc.ReopenIssue(ctx, sc.Org, sc.Repo, sc.IssueNumber)
	}
	return h.ghc.CloseIssue(ctx, sc.Org, sc.Repo, sc.IssueNumber)
}

// Post is a no-op: close/reopen do not affect auto-merge eligibility.
func (h *CloseHandler) Post(_ context.Context, _ *event.Context, _ []string, _ error) error {
	return nil
}
