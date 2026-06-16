package merge

import (
	"context"
	"errors"
	"net/http"
	"strings"

	gh "github.com/google/go-github/v72/github"

	"github.com/elevran/stern/internal/config"
	"github.com/elevran/stern/internal/labels"
	ghclient "github.com/elevran/stern/internal/github"
)

// EligibilityResult reports whether a PR is ready to auto-merge.
type EligibilityResult struct {
	Ready          bool
	MissingLabels  []string // required labels not yet present (e.g. ["lgtm"])
	BlockingLabels []string // present labels that block merging (e.g. ["do-not-merge/hold"])
}

// CheckEligibility determines whether pr is ready for auto-merge based on opts.
// A PR is ready when it has all required labels and none of the blocking labels.
func CheckEligibility(pr *gh.PullRequest, opts *config.Options) EligibilityResult {
	present := make(map[string]bool)
	for _, l := range pr.Labels {
		present[strings.ToLower(l.GetName())] = true
	}

	var missing, blocking []string

	// Required labels: lgtm + approved.
	required := []string{labels.LGTM, labels.Approved}
	for _, req := range required {
		if !present[strings.ToLower(req)] {
			missing = append(missing, req)
		}
	}

	// Blocking labels from config, falling back to the standard set.
	blockList := opts.Merge.BlockingLabels
	if len(blockList) == 0 {
		blockList = []string{labels.Hold, labels.WIP, labels.NeedsRebase}
	}
	for _, bl := range blockList {
		if present[strings.ToLower(bl)] {
			blocking = append(blocking, bl)
		}
	}

	return EligibilityResult{
		Ready:          len(missing) == 0 && len(blocking) == 0,
		MissingLabels:  missing,
		BlockingLabels: blocking,
	}
}

// EnableAutoMerge enables GitHub's native auto-merge on a PR.
// Returns nil if auto-merge is already enabled.
func EnableAutoMerge(ctx context.Context, ghc ghclient.Client, owner, repo string, number int, method string) error {
	if method == "" {
		method = "squash"
	}
	return ghc.EnableAutoMerge(ctx, owner, repo, number, method)
}

// DisableAutoMerge disables GitHub's native auto-merge on a PR.
// Returns nil if auto-merge is not enabled.
func DisableAutoMerge(ctx context.Context, ghc ghclient.Client, owner, repo string, number int) error {
	return ghc.DisableAutoMerge(ctx, owner, repo, number)
}

// CheckAndApplyAutoMerge calls CheckEligibility and enables/disables auto-merge
// on the PR accordingly. It is a convenience wrapper used by label-modifying handlers.
func CheckAndApplyAutoMerge(ctx context.Context, ghc ghclient.Client, pr *gh.PullRequest, owner, repo string, opts *config.Options) error {
	result := CheckEligibility(pr, opts)
	if result.Ready {
		return EnableAutoMerge(ctx, ghc, owner, repo, pr.GetNumber(), opts.Merge.Method)
	}
	return DisableAutoMerge(ctx, ghc, owner, repo, pr.GetNumber())
}

// IsNotFoundError reports whether err is a 404 from the GitHub API.
func IsNotFoundError(err error) bool {
	var ghErr *gh.ErrorResponse
	return errors.As(err, &ghErr) && ghErr.Response != nil &&
		ghErr.Response.StatusCode == http.StatusNotFound
}
