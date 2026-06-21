package pr

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/sirupsen/logrus"

	"github.com/elevran/stern/internal/config"
	"github.com/elevran/stern/internal/github"
	"github.com/elevran/stern/internal/owners"
)

// HandlePREventReviewAssignment auto-assigns reviewers from OWNERS files when
// a PR is opened. Reviewers are selected deterministically (sorted, then the
// first Count candidates are chosen). The PR author is always excluded. The
// "least-busy" load-balancing strategy is logged as not yet implemented and
// falls back to the deterministic round-robin-style selection.
func HandlePREventReviewAssignment(ctx context.Context, ghc prClient, org, repo string, p github.PullRequest, opts *config.Options) error {
	if !opts.ReviewAssignment.Enabled {
		return nil
	}

	log := logrus.WithFields(logrus.Fields{
		"org":            org,
		"repo":           repo,
		"pr":             p.Number,
		"load_balancing": opts.ReviewAssignment.LoadBalancing,
	})

	if opts.ReviewAssignment.LoadBalancing == "least-busy" {
		log.Info("review-assignment: least-busy strategy not yet implemented; using round-robin fallback")
	}

	paths, err := ghc.ListPullRequestFiles(ctx, org, repo, p.Number)
	if err != nil {
		return fmt.Errorf("listing PR files: %w", err)
	}
	if len(paths) == 0 {
		return nil
	}

	resolved, err := owners.LoadForPaths(ctx, ghc, nil, org, repo, p.HeadSHA, paths)
	if err != nil {
		return fmt.Errorf("loading OWNERS for paths: %w", err)
	}
	if len(resolved.Approvers) == 0 {
		return nil
	}

	author := p.Author
	candidates := make([]string, 0, len(resolved.Approvers))
	for _, a := range resolved.Approvers {
		if author != "" && strings.EqualFold(a, author) {
			continue
		}
		candidates = append(candidates, a)
	}
	if len(candidates) == 0 {
		return nil
	}

	sort.Strings(candidates)
	n := opts.ReviewAssignment.Count
	if n > len(candidates) {
		n = len(candidates)
	}
	selected := candidates[:n]

	if err := ghc.RequestReviewers(ctx, org, repo, p.Number, selected); err != nil {
		return fmt.Errorf("requesting reviewers: %w", err)
	}

	mentions := make([]string, len(selected))
	for i, s := range selected {
		mentions[i] = "@" + s
	}
	body := "Assigned reviewers: " + strings.Join(mentions, ", ")
	if err := ghc.CreateIssueComment(ctx, org, repo, p.Number, body); err != nil {
		return fmt.Errorf("posting reviewer-assignment comment: %w", err)
	}

	log.WithField("reviewers", selected).Info("review-assignment: reviewers assigned")
	return nil
}
