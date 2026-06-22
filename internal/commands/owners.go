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
	github.PermissionsClient
}

// checkOwners runs the OWNERS authorization check for a Pre handler.
// check returns true if the caller is authorized.
//
// OWNERS is loaded from the PR's base ref (trusted target code), not the PR
// head SHA (which is attacker-controlled on fork PRs). When no OWNERS files
// cover the changed paths, the check falls back to repo write access so the
// no-OWNERS case fails closed rather than open.
func checkOwners(
	ctx context.Context,
	sc *event.Context,
	ghc ownersCheckerClient,
	check func(*owners.ResolvedOwners) bool,
	permFmt string,
) error {
	if sc.PR.BaseSHA == "" {
		return PermissionError("cannot verify OWNERS authorization: PR base SHA is unknown")
	}
	files, err := ghc.ListPullRequestFiles(ctx, sc.Org, sc.Repo, sc.IssueNumber)
	if err != nil {
		return err
	}
	resolved, err := owners.LoadForPaths(ctx, ghc, nil, sc.Org, sc.Repo, sc.PR.BaseSHA, files)
	if err != nil {
		return err
	}
	if !resolved.HasOwners() {
		// No OWNERS file covers the changed paths; require repo write access
		// rather than allowing any commenter through.
		ok, err := ghc.HasWriteAccess(ctx, sc.Org, sc.Repo, sc.Author)
		if err != nil {
			return err
		}
		if !ok {
			return PermissionError("%s is not in OWNERS and does not have write access to %s/%s",
				sc.Author, sc.Org, sc.Repo)
		}
		return nil
	}
	if !check(resolved) {
		return PermissionError(permFmt, sc.Author)
	}
	return nil
}
