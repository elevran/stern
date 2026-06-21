package commands

import (
	"context"

	"github.com/elevran/stern/internal/event"
	"github.com/elevran/stern/internal/github"
	"github.com/elevran/stern/internal/owners"
)

type ownersCheckerClient interface {
	github.PullRequestsClient
	github.ContentClient
}

// checkOwners runs the OWNERS authorization check for a Pre handler.
// check returns true if the caller is authorized.
// Returns nil when HeadSHA is empty, no OWNERS files exist, or check passes.
func checkOwners(
	ctx context.Context,
	sc *event.Context,
	ghc ownersCheckerClient,
	check func(*owners.ResolvedOwners) bool,
	permFmt string,
) error {
	if sc.PR.HeadSHA == "" {
		return nil
	}
	files, err := ghc.ListPullRequestFiles(ctx, sc.Org, sc.Repo, sc.IssueNumber)
	if err != nil {
		return err
	}
	resolved, err := owners.LoadForPaths(ctx, ghc, sc.Org, sc.Repo, sc.PR.HeadSHA, files)
	if err != nil {
		return err
	}
	if !resolved.HasOwners() {
		return nil
	}
	if !check(resolved) {
		return PermissionError(permFmt, sc.Author)
	}
	return nil
}
