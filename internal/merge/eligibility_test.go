package merge_test

import (
	"context"
	"testing"

	gh "github.com/google/go-github/v72/github"

	"github.com/elevran/stern/internal/config"
	"github.com/elevran/stern/internal/merge"
	ghclient "github.com/elevran/stern/internal/github"
)

func pr(labelNames ...string) *gh.PullRequest {
	labels := make([]*gh.Label, len(labelNames))
	for i, n := range labelNames {
		name := n
		labels[i] = &gh.Label{Name: &name}
	}
	num := 1
	return &gh.PullRequest{Number: &num, Labels: labels}
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

func TestEnableAutoMerge_AlreadyEnabled(t *testing.T) {
	ghc := ghclient.NewMockClient()
	// Default mock returns nil; simulate "already enabled" by doing nothing extra
	if err := merge.EnableAutoMerge(context.Background(), ghc, "o", "r", 1, "squash"); err != nil {
		t.Errorf("EnableAutoMerge() error = %v", err)
	}
}

func TestDisableAutoMerge_NotEnabled(t *testing.T) {
	ghc := ghclient.NewMockClient()
	if err := merge.DisableAutoMerge(context.Background(), ghc, "o", "r", 1); err != nil {
		t.Errorf("DisableAutoMerge() error = %v", err)
	}
}

func TestCheckAndApplyAutoMerge_EnablesWhenReady(t *testing.T) {
	ghc := ghclient.NewMockClient()
	p := pr("lgtm", "approved")
	if err := merge.CheckAndApplyAutoMerge(context.Background(), ghc, p, "o", "r", opts()); err != nil {
		t.Errorf("CheckAndApplyAutoMerge() error = %v", err)
	}
}

func TestCheckAndApplyAutoMerge_DisablesWhenNotReady(t *testing.T) {
	ghc := ghclient.NewMockClient()
	p := pr("lgtm") // missing approved
	if err := merge.CheckAndApplyAutoMerge(context.Background(), ghc, p, "o", "r", opts()); err != nil {
		t.Errorf("CheckAndApplyAutoMerge() error = %v", err)
	}
}
