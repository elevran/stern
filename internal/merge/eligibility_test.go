package merge_test

import (
	"context"
	"testing"

	"github.com/elevran/stern/internal/config"
	"github.com/elevran/stern/internal/github"
	"github.com/elevran/stern/internal/merge"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func pr(labelNames ...string) github.PullRequest {
	return github.PullRequest{
		Number: 1,
		NodeID: "test-node-id",
		Labels: labelNames,
	}
}

func opts() *config.Options {
	return &config.Options{
		Merge: config.MergeOptions{
			Method: "squash",
			BlockingLabels: []string{
				"do-not-merge/hold",
				"do-not-merge/wip",
				"needs-rebase",
			},
		},
	}
}

func TestCheckEligibility_Ready(t *testing.T) {
	result := merge.CheckEligibility(pr("lgtm", "approved"), opts())
	assert.True(t, result.Ready, "expected Ready=true, got MissingLabels=%v BlockingLabels=%v", result.MissingLabels, result.BlockingLabels)
}

func TestCheckEligibility_MissingLGTM(t *testing.T) {
	result := merge.CheckEligibility(pr("approved"), opts())
	assert.False(t, result.Ready, "expected Ready=false when lgtm is missing")
	assert.NotEmpty(t, result.MissingLabels, "expected MissingLabels to include lgtm")
}

func TestCheckEligibility_MissingApproved(t *testing.T) {
	result := merge.CheckEligibility(pr("lgtm"), opts())
	assert.False(t, result.Ready, "expected Ready=false when approved is missing")
}

func TestCheckEligibility_BothMissing(t *testing.T) {
	result := merge.CheckEligibility(pr(), opts())
	assert.False(t, result.Ready, "expected Ready=false when both labels missing")
	assert.Len(t, result.MissingLabels, 2, "expected 2 missing labels, got %v", result.MissingLabels)
}

func TestCheckEligibility_Hold(t *testing.T) {
	result := merge.CheckEligibility(pr("lgtm", "approved", "do-not-merge/hold"), opts())
	assert.False(t, result.Ready, "expected Ready=false when hold is present")
	assert.NotEmpty(t, result.BlockingLabels, "expected BlockingLabels to include do-not-merge/hold")
}

func TestCheckEligibility_WIP(t *testing.T) {
	result := merge.CheckEligibility(pr("lgtm", "approved", "do-not-merge/wip"), opts())
	assert.False(t, result.Ready, "expected Ready=false when WIP is present")
}

func TestCheckEligibility_MultipleBlockers(t *testing.T) {
	result := merge.CheckEligibility(
		pr("lgtm", "approved", "do-not-merge/hold", "do-not-merge/wip"),
		opts(),
	)
	assert.False(t, result.Ready, "expected Ready=false")
	assert.Len(t, result.BlockingLabels, 2, "expected 2 blocking labels, got %v", result.BlockingLabels)
}

func TestCheckAndApplyAutoMerge_DisableUnavailable_NonFatal(t *testing.T) {
	ghc := github.NewMockClient()
	ghc.Errors["DisableAutoMerge"] = &github.GraphQLError{Message: "Resource not accessible by integration", Type: "FORBIDDEN"}

	// Only lgtm — not eligible, so DisableAutoMerge is called.
	p := pr("lgtm")
	assert.NoError(t, merge.CheckAndApplyAutoMerge(context.Background(), ghc, p, opts()),
		"auto-merge unavailable should be non-fatal")
}

func TestCheckAndApplyAutoMerge_EnableUnavailable_NonFatal(t *testing.T) {
	ghc := github.NewMockClient()
	ghc.Errors["EnableAutoMerge"] = &github.GraphQLError{Message: "Resource not accessible by integration", Type: "FORBIDDEN"}

	// Both labels present — eligible, so EnableAutoMerge is called.
	p := pr("lgtm", "approved")
	assert.NoError(t, merge.CheckAndApplyAutoMerge(context.Background(), ghc, p, opts()),
		"auto-merge unavailable should be non-fatal")
}

func TestCheckAndApplyAutoMerge_OtherDisableError_Propagates(t *testing.T) {
	ghc := github.NewMockClient()
	ghc.Errors["DisableAutoMerge"] = &github.GraphQLError{Message: "something unexpected", Type: "INTERNAL"}

	p := pr("lgtm")
	assert.Error(t, merge.CheckAndApplyAutoMerge(context.Background(), ghc, p, opts()),
		"unexpected error should propagate")
}

func TestDisableAutoMerge_Unavailable_ReturnsNil(t *testing.T) {
	ghc := github.NewMockClient()
	ghc.Errors["DisableAutoMerge"] = &github.GraphQLError{Message: "Resource not accessible by integration", Type: "FORBIDDEN"}

	assert.NoError(t, merge.DisableAutoMerge(context.Background(), ghc, "PR_test_node_id"),
		"unavailable error should be suppressed")
}

func TestDisableAutoMerge_OtherError_Propagates(t *testing.T) {
	ghc := github.NewMockClient()
	ghc.Errors["DisableAutoMerge"] = &github.GraphQLError{Message: "some real failure", Type: "INTERNAL"}

	assert.Error(t, merge.DisableAutoMerge(context.Background(), ghc, "PR_test_node_id"),
		"non-unavailable error should propagate")
}

func TestCheckAndApplyAutoMerge_EnablesWhenReady(t *testing.T) {
	ghc := github.NewMockClient()
	p := pr("lgtm", "approved")
	require.NoError(t, merge.CheckAndApplyAutoMerge(context.Background(), ghc, p, opts()))
	assert.Equal(t, 1, ghc.EnableAutoMergeCallCount, "expected EnableAutoMergeCallCount=1")
}

func TestCheckAndApplyAutoMerge_DisablesWhenNotReady(t *testing.T) {
	ghc := github.NewMockClient()
	p := pr("lgtm") // missing approved
	require.NoError(t, merge.CheckAndApplyAutoMerge(context.Background(), ghc, p, opts()))
	assert.Equal(t, 1, ghc.DisableAutoMergeCallCount, "expected DisableAutoMergeCallCount=1")
}
