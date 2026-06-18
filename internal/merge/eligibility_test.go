package merge_test

import (
	"context"
	"fmt"
	"testing"

	gh "github.com/google/go-github/v72/github"

	"github.com/elevran/stern/internal/config"
	"github.com/elevran/stern/internal/ghclient"
	"github.com/elevran/stern/internal/merge"
)

func pr(labelNames ...string) *gh.PullRequest {
	labels := make([]*gh.Label, len(labelNames))
	for i, n := range labelNames {
		name := n
		labels[i] = &gh.Label{Name: &name}
	}
	num := 1
	nodeID := "test-node-id"
	return &gh.PullRequest{Number: &num, NodeID: &nodeID, Labels: labels}
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
	if !result.Ready {
		t.Errorf("expected Ready=true, got MissingLabels=%v BlockingLabels=%v", result.MissingLabels, result.BlockingLabels)
	}
}

func TestCheckEligibility_MissingLGTM(t *testing.T) {
	result := merge.CheckEligibility(pr("approved"), opts())
	if result.Ready {
		t.Error("expected Ready=false when lgtm is missing")
	}
	if len(result.MissingLabels) == 0 {
		t.Error("expected MissingLabels to include lgtm")
	}
}

func TestCheckEligibility_MissingApproved(t *testing.T) {
	result := merge.CheckEligibility(pr("lgtm"), opts())
	if result.Ready {
		t.Error("expected Ready=false when approved is missing")
	}
}

func TestCheckEligibility_BothMissing(t *testing.T) {
	result := merge.CheckEligibility(pr(), opts())
	if result.Ready {
		t.Error("expected Ready=false when both labels missing")
	}
	if len(result.MissingLabels) != 2 {
		t.Errorf("expected 2 missing labels, got %v", result.MissingLabels)
	}
}

func TestCheckEligibility_Hold(t *testing.T) {
	result := merge.CheckEligibility(pr("lgtm", "approved", "do-not-merge/hold"), opts())
	if result.Ready {
		t.Error("expected Ready=false when hold is present")
	}
	if len(result.BlockingLabels) == 0 {
		t.Error("expected BlockingLabels to include do-not-merge/hold")
	}
}

func TestCheckEligibility_WIP(t *testing.T) {
	result := merge.CheckEligibility(pr("lgtm", "approved", "do-not-merge/wip"), opts())
	if result.Ready {
		t.Error("expected Ready=false when WIP is present")
	}
}

func TestCheckEligibility_MultipleBlockers(t *testing.T) {
	result := merge.CheckEligibility(
		pr("lgtm", "approved", "do-not-merge/hold", "do-not-merge/wip"),
		opts(),
	)
	if result.Ready {
		t.Error("expected Ready=false")
	}
	if len(result.BlockingLabels) != 2 {
		t.Errorf("expected 2 blocking labels, got %v", result.BlockingLabels)
	}
}

func TestCheckAndApplyAutoMerge_DisableUnavailable_NonFatal(t *testing.T) {
	ghc := ghclient.NewMockClient()
	ghc.Errors["DisableAutoMerge"] = fmt.Errorf("graphql: Resource not accessible by integration")

	// Only lgtm — not eligible, so DisableAutoMerge is called.
	p := pr("lgtm")
	if err := merge.CheckAndApplyAutoMerge(context.Background(), ghc, p, opts()); err != nil {
		t.Errorf("auto-merge unavailable should be non-fatal, got: %v", err)
	}
}

func TestCheckAndApplyAutoMerge_EnableUnavailable_NonFatal(t *testing.T) {
	ghc := ghclient.NewMockClient()
	ghc.Errors["EnableAutoMerge"] = fmt.Errorf("graphql: Resource not accessible by integration")

	// Both labels present — eligible, so EnableAutoMerge is called.
	p := pr("lgtm", "approved")
	if err := merge.CheckAndApplyAutoMerge(context.Background(), ghc, p, opts()); err != nil {
		t.Errorf("auto-merge unavailable should be non-fatal, got: %v", err)
	}
}

func TestCheckAndApplyAutoMerge_OtherDisableError_Propagates(t *testing.T) {
	ghc := ghclient.NewMockClient()
	ghc.Errors["DisableAutoMerge"] = fmt.Errorf("graphql: something unexpected")

	p := pr("lgtm")
	if err := merge.CheckAndApplyAutoMerge(context.Background(), ghc, p, opts()); err == nil {
		t.Error("unexpected error should propagate")
	}
}

func TestDisableAutoMerge_Unavailable_ReturnsNil(t *testing.T) {
	ghc := ghclient.NewMockClient()
	ghc.Errors["DisableAutoMerge"] = fmt.Errorf("graphql: Resource not accessible by integration")

	if err := merge.DisableAutoMerge(context.Background(), ghc, "PR_test_node_id"); err != nil {
		t.Errorf("unavailable error should be suppressed, got: %v", err)
	}
}

func TestDisableAutoMerge_OtherError_Propagates(t *testing.T) {
	ghc := ghclient.NewMockClient()
	ghc.Errors["DisableAutoMerge"] = fmt.Errorf("graphql: some real failure")

	if err := merge.DisableAutoMerge(context.Background(), ghc, "PR_test_node_id"); err == nil {
		t.Error("non-unavailable error should propagate")
	}
}

func TestCheckAndApplyAutoMerge_EnablesWhenReady(t *testing.T) {
	ghc := ghclient.NewMockClient()
	p := pr("lgtm", "approved")
	if err := merge.CheckAndApplyAutoMerge(context.Background(), ghc, p, opts()); err != nil {
		t.Errorf("CheckAndApplyAutoMerge() error = %v", err)
	}
	if ghc.Errors["EnableAutoMerge"] != nil {
		t.Error("expected EnableAutoMerge to be called")
	}
}

func TestCheckAndApplyAutoMerge_DisablesWhenNotReady(t *testing.T) {
	ghc := ghclient.NewMockClient()
	p := pr("lgtm") // missing approved
	if err := merge.CheckAndApplyAutoMerge(context.Background(), ghc, p, opts()); err != nil {
		t.Errorf("CheckAndApplyAutoMerge() error = %v", err)
	}
}
