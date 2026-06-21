package pr

import (
	"context"
	"strings"

	"github.com/elevran/stern/internal/config"
	"github.com/elevran/stern/internal/github"
	"github.com/elevran/stern/internal/labels"
)

// BucketForSize returns the bucket name whose [Min, Max] inclusive range
// contains the given line count. Min == 0 means no lower bound, Max == 0
// means no upper bound. Returns "" if no bucket matches.
func BucketForSize(lines int, buckets []config.SizeBucket) string {
	for _, b := range buckets {
		if b.Min > 0 && lines < b.Min {
			continue
		}
		if b.Max > 0 && lines > b.Max {
			continue
		}
		return b.Name
	}
	return ""
}

// HandlePREventSize applies or refreshes the size/* label on a PR based on
// the diff size (Additions + Deletions). Any existing size/* labels are
// removed first so the PR carries exactly one.
func HandlePREventSize(ctx context.Context, ghc prClient, org, repo string, p github.PullRequest, opts *config.Options) error {
	if len(opts.Size.Buckets) == 0 {
		return nil
	}
	bucket := BucketForSize(p.Additions+p.Deletions, opts.Size.Buckets)
	if bucket == "" {
		return nil
	}
	for _, l := range p.Labels {
		if !strings.HasPrefix(l, labels.SizePrefix) {
			continue
		}
		if err := ghc.RemoveLabel(ctx, org, repo, p.Number, l); err != nil && !github.IsNotFoundError(err) {
			return err
		}
	}
	return ghc.AddLabels(ctx, org, repo, p.Number, []string{labels.SizePrefix + bucket})
}
