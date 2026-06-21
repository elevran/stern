package pr_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elevran/stern/internal/config"
	"github.com/elevran/stern/internal/github"
	"github.com/elevran/stern/internal/pr"
)

// defaultSizeOpts returns an Options whose Size.Buckets matches the package
// defaults, suitable for tests that need a populated bucket list.
func defaultSizeOpts() *config.Options {
	return &config.Options{
		Size: config.SizeOptions{Buckets: []config.SizeBucket{
			{Name: "XS", Max: 10},
			{Name: "S", Min: 11, Max: 30},
			{Name: "M", Min: 31, Max: 100},
			{Name: "L", Min: 101, Max: 300},
			{Name: "XL", Min: 301, Max: 1000},
			{Name: "XXL", Min: 1001},
		}},
	}
}

func TestBucketForSize_Defaults(t *testing.T) {
	opts := defaultSizeOpts()
	cases := []struct {
		lines int
		want  string
	}{
		{0, "XS"},
		{1, "XS"},
		{10, "XS"},
		{11, "S"},
		{20, "S"},
		{30, "S"},
		{31, "M"},
		{50, "M"},
		{100, "M"},
		{101, "L"},
		{200, "L"},
		{300, "L"},
		{301, "XL"},
		{500, "XL"},
		{1000, "XL"},
		{1001, "XXL"},
		{5000, "XXL"},
	}
	for _, tc := range cases {
		got := pr.BucketForSize(tc.lines, opts.Size.Buckets)
		assert.Equal(t, tc.want, got, "BucketForSize(%d)", tc.lines)
	}
}

func TestBucketForSize_NoMatch(t *testing.T) {
	buckets := []config.SizeBucket{
		{Name: "MIN", Min: 1, Max: 5},
		{Name: "MAX", Min: 100, Max: 200},
	}
	assert.Empty(t, pr.BucketForSize(50, buckets), "BucketForSize(50)")
	assert.Empty(t, pr.BucketForSize(0, buckets), "BucketForSize(0)")
}

func TestBucketForSize_OpenEndedBuckets(t *testing.T) {
	buckets := []config.SizeBucket{
		{Name: "ANY", Min: 0, Max: 0},
	}
	assert.Equal(t, "ANY", pr.BucketForSize(0, buckets), "BucketForSize(0)")
	assert.Equal(t, "ANY", pr.BucketForSize(1_000_000, buckets), "BucketForSize(big)")
}

func TestBucketForSize_EmptyBuckets(t *testing.T) {
	assert.Empty(t, pr.BucketForSize(5, nil), "BucketForSize(5, nil)")
}

func TestHandlePREventSize_AddsLabel(t *testing.T) {
	ghc := github.NewMockClient()
	opts := defaultSizeOpts()
	p := github.PullRequest{
		Number:    1,
		Additions: 40,
		Deletions: 10,
		Labels:    []string{},
	}

	require.NoError(t, pr.HandlePREventSize(context.Background(), ghc, "o", "r", p, opts))
	assert.True(t, ghc.IssueLabels[1]["size/M"], "expected size/M label added")
}

func TestHandlePREventSize_ReplacesExistingLabel(t *testing.T) {
	ghc := github.NewMockClient()
	ghc.IssueLabels[1] = map[string]bool{"size/L": true}
	opts := defaultSizeOpts()
	p := github.PullRequest{
		Number:    1,
		Additions: 40,
		Deletions: 10,
		Labels:    []string{"size/L"},
	}

	require.NoError(t, pr.HandlePREventSize(context.Background(), ghc, "o", "r", p, opts))
	assert.False(t, ghc.IssueLabels[1]["size/L"], "expected old size/L label removed")
	assert.True(t, ghc.IssueLabels[1]["size/M"], "expected new size/M label added")
}

func TestHandlePREventSize_NoOpWhenBucketsEmpty(t *testing.T) {
	ghc := github.NewMockClient()
	p := github.PullRequest{
		Number:    1,
		Additions: 50,
		Deletions: 0,
	}
	opts := &config.Options{} // no buckets configured

	require.NoError(t, pr.HandlePREventSize(context.Background(), ghc, "o", "r", p, opts))
	assert.Empty(t, ghc.IssueLabels, "expected no label changes")
}

func TestHandlePREventSize_NoOpWhenNoBucketMatches(t *testing.T) {
	ghc := github.NewMockClient()
	opts := &config.Options{
		Size: config.SizeOptions{
			Buckets: []config.SizeBucket{
				{Name: "TINY", Max: 10},
				{Name: "HUGE", Min: 1001},
			},
		},
	}
	p := github.PullRequest{
		Number:    1,
		Additions: 500,
		Deletions: 0,
	}

	require.NoError(t, pr.HandlePREventSize(context.Background(), ghc, "o", "r", p, opts))
	assert.Empty(t, ghc.IssueLabels, "expected no label changes")
}

func TestHandlePREventSize_RemovesMultiplePriorSizeLabels(t *testing.T) {
	ghc := github.NewMockClient()
	ghc.IssueLabels[1] = map[string]bool{
		"size/XS":  true,
		"size/XXL": true,
	}
	opts := defaultSizeOpts()
	p := github.PullRequest{
		Number:    1,
		Additions: 5,
		Deletions: 1,
		Labels:    []string{"size/XS", "size/XXL"},
	}

	require.NoError(t, pr.HandlePREventSize(context.Background(), ghc, "o", "r", p, opts))
	// size/XXL must be gone (we added back size/XS, so checking XXL specifically
	// confirms the prior XXL was actually removed rather than left in place).
	assert.False(t, ghc.IssueLabels[1]["size/XXL"], "expected size/XXL removed; got %v", ghc.IssueLabels[1])
	assert.True(t, ghc.IssueLabels[1]["size/XS"], "expected size/XS added back; got %v", ghc.IssueLabels[1])
	assert.Len(t, ghc.IssueLabels[1], 1, "expected exactly one size label, got %v", ghc.IssueLabels[1])
}

func TestHandlePREventSize_LeavesNonSizeLabelsAlone(t *testing.T) {
	ghc := github.NewMockClient()
	ghc.IssueLabels[1] = map[string]bool{
		"do-not-merge/hold": true,
	}
	opts := defaultSizeOpts()
	p := github.PullRequest{
		Number:    1,
		Additions: 5,
		Deletions: 1,
		Labels:    []string{"do-not-merge/hold"},
	}

	require.NoError(t, pr.HandlePREventSize(context.Background(), ghc, "o", "r", p, opts))
	assert.True(t, ghc.IssueLabels[1]["do-not-merge/hold"], "expected non-size label untouched")
	assert.True(t, ghc.IssueLabels[1]["size/XS"], "expected size/XS added")
}
