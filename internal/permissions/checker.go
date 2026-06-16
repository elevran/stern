package permissions

import (
	"context"

	gh "github.com/google/go-github/v72/github"

	"github.com/elevran/stern/internal/event"
	ghclient "github.com/elevran/stern/internal/github"
)

// Checker provides permission checks used by all command handlers.
type Checker interface {
	// IsOrgMember reports whether user is a member of org.
	IsOrgMember(ctx context.Context, org, user string) (bool, error)
	// HasWriteAccess reports whether user has write (or higher) access to owner/repo.
	HasWriteAccess(ctx context.Context, owner, repo, user string) (bool, error)
	// IsPRAuthor reports whether user is the author of pr.
	IsPRAuthor(pr *gh.PullRequest, user string) bool
	// IsBot reports whether user is the configured bot account.
	IsBot(user string) bool
}

type checker struct {
	ghc    ghclient.Client
	sternCtx *event.Context
}

// New returns a Checker backed by the given GitHub client and stern context.
func New(ghc ghclient.Client, sc *event.Context) Checker {
	return &checker{ghc: ghc, sternCtx: sc}
}

func (c *checker) IsOrgMember(ctx context.Context, org, user string) (bool, error) {
	return c.ghc.IsOrgMember(ctx, org, user)
}

func (c *checker) HasWriteAccess(ctx context.Context, owner, repo, user string) (bool, error) {
	return c.ghc.HasWriteAccess(ctx, owner, repo, user)
}

func (c *checker) IsPRAuthor(pr *gh.PullRequest, user string) bool {
	if pr == nil || pr.User == nil {
		return false
	}
	return pr.User.GetLogin() == user
}

func (c *checker) IsBot(user string) bool {
	return user == c.sternCtx.BotLogin
}
