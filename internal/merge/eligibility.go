package merge

import (
	"context"
	"strings"

	"github.com/sirupsen/logrus"

	"github.com/elevran/stern/internal/config"
	"github.com/elevran/stern/internal/github"
	"github.com/elevran/stern/internal/labels"
)

// EligibilityResult reports whether a PR is ready to auto-merge.
type EligibilityResult struct {
	Ready          bool
	MissingLabels  []string // required labels not yet present (e.g. ["lgtm"])
	BlockingLabels []string // present labels that block merging (e.g. ["do-not-merge/hold"])
}

// CheckEligibility determines whether pr is ready for auto-merge based on opts.
// A PR is ready when it has all required labels and none of the blocking labels.
func CheckEligibility(pr github.PullRequest, opts *config.Options) EligibilityResult {
	present := make(map[string]bool)
	for _, l := range pr.Labels {
		present[strings.ToLower(l)] = true
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

// CheckAndApplyAutoMerge calls CheckEligibility and enables/disables auto-merge
// on the PR accordingly. It is a convenience wrapper used by label-modifying handlers.
// Auto-merge errors are non-fatal: the primary label operation already succeeded.
func CheckAndApplyAutoMerge(ctx context.Context, ghc github.PullRequestsClient, pr github.PullRequest, opts *config.Options) error {
	result := CheckEligibility(pr, opts)
	nodeID := pr.NodeID
	if result.Ready {
		method := opts.Merge.Method
		if method == "" {
			method = "squash"
		}
		if err := ghc.EnableAutoMerge(ctx, nodeID, method); err != nil {
			if isAutoMergeUnavailable(err) {
				logrus.WithError(err).Warn("auto-merge: PR is eligible but auto-merge could not be enabled; " +
					"ensure 'Allow auto-merge' is enabled in repository Settings → General")
				return nil
			}
			return err
		}
		return nil
	}
	return DisableAutoMerge(ctx, ghc, nodeID)
}

// DisableAutoMerge disables GitHub's native auto-merge on a PR. If the feature
// is unavailable for this repository, the error is logged at debug level and nil
// is returned — disabling is a no-op when auto-merge was never enabled.
func DisableAutoMerge(ctx context.Context, ghc github.PullRequestsClient, nodeID string) error {
	if err := ghc.DisableAutoMerge(ctx, nodeID); err != nil {
		if isAutoMergeUnavailable(err) {
			logrus.WithError(err).Debug("auto-merge: disable skipped — feature not available for this repository")
			return nil
		}
		return err
	}
	return nil
}

// isAutoMergeUnavailable reports whether err is GitHub's "Resource not accessible
// by integration" response, which occurs when auto-merge is disabled at the
// repository level (Settings → General → Allow auto-merge) or the token lacks access.
func isAutoMergeUnavailable(err error) bool {
	return err != nil && strings.Contains(err.Error(), "Resource not accessible by integration")
}
