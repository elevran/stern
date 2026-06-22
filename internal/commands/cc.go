package commands

import (
	"context"

	"github.com/elevran/stern/internal/config"
	"github.com/elevran/stern/internal/event"
	"github.com/elevran/stern/internal/github"
)

// CCHandler handles /cc and /uncc.
type CCHandler struct {
	ghc  userCommandClient
	verb string // "cc" or "uncc"
}

// NewCCHandler constructs a CCHandler bound to a single verb ("cc" or "uncc").
// Each verb is registered as its own factory in DefaultRegistry.
func NewCCHandler(verb string) HandlerFactory {
	return func(_ *event.Context, ghc github.Client, _ *config.Options) Handler {
		return &CCHandler{ghc: ghc, verb: verb}
	}
}

// Pre enforces /cc and /uncc are used on a PR and require at least one user argument.
func (h *CCHandler) Pre(_ context.Context, sc *event.Context, args []string) error {
	if sc.PR == nil {
		return PermissionError("/%s may only be used on pull requests", h.verb)
	}
	if len(args) == 0 {
		return PermissionError("usage: /%s @user [@user ...]", h.verb)
	}
	return nil
}

// Handle parses the user list and applies RequestReviewers (cc) or
// RemoveReviewers (uncc) per the handler's verb.
func (h *CCHandler) Handle(ctx context.Context, sc *event.Context, args []string) error {
	users, err := parseUsers(args, "")
	if err != nil {
		return PermissionError("usage: /%s @user [@user ...]", h.verb)
	}
	if h.verb == "uncc" {
		return h.ghc.RemoveReviewers(ctx, sc.Org, sc.Repo, sc.IssueNumber, users)
	}
	return h.ghc.RequestReviewers(ctx, sc.Org, sc.Repo, sc.IssueNumber, users)
}

// Post is a no-op: cc/uncc do not affect auto-merge eligibility.
func (h *CCHandler) Post(_ context.Context, _ *event.Context, _ []string, _ error) error {
	return nil
}
