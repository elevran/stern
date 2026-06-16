package config_test

import (
	"context"
	"os"
	"testing"

	gh "github.com/google/go-github/v72/github"

	"github.com/elevran/stern/internal/config"
	ghclient "github.com/elevran/stern/internal/github"
)

func TestDiffLabels_Create(t *testing.T) {
	desired := []config.LabelDefinition{
		{Name: "lgtm", Color: "0e8a16", Description: "Looks good to me"},
	}
	diffs := config.DiffLabels(desired, nil)
	if len(diffs) != 1 || diffs[0].Action != config.LabelCreate {
		t.Errorf("expected CREATE, got %v", diffs)
	}
}

func TestDiffLabels_OK(t *testing.T) {
	desired := []config.LabelDefinition{
		{Name: "lgtm", Color: "0e8a16", Description: "Looks good to me"},
	}
	current := []*gh.Label{
		{Name: gh.Ptr("lgtm"), Color: gh.Ptr("0e8a16"), Description: gh.Ptr("Looks good to me")},
	}
	diffs := config.DiffLabels(desired, current)
	if len(diffs) != 1 || diffs[0].Action != config.LabelOK {
		t.Errorf("expected OK, got %v", diffs)
	}
}

func TestDiffLabels_Update_Color(t *testing.T) {
	desired := []config.LabelDefinition{
		{Name: "lgtm", Color: "0e8a16", Description: "Looks good to me"},
	}
	current := []*gh.Label{
		{Name: gh.Ptr("lgtm"), Color: gh.Ptr("ffffff"), Description: gh.Ptr("Looks good to me")},
	}
	diffs := config.DiffLabels(desired, current)
	if len(diffs) != 1 || diffs[0].Action != config.LabelUpdate {
		t.Errorf("expected UPDATE for color change, got %v", diffs)
	}
}

func TestDiffLabels_Update_Description(t *testing.T) {
	desired := []config.LabelDefinition{
		{Name: "lgtm", Color: "0e8a16", Description: "New description"},
	}
	current := []*gh.Label{
		{Name: gh.Ptr("lgtm"), Color: gh.Ptr("0e8a16"), Description: gh.Ptr("Old description")},
	}
	diffs := config.DiffLabels(desired, current)
	if len(diffs) != 1 || diffs[0].Action != config.LabelUpdate {
		t.Errorf("expected UPDATE for description change, got %v", diffs)
	}
}

func TestDiffLabels_Extra(t *testing.T) {
	current := []*gh.Label{
		{Name: gh.Ptr("extra-label"), Color: gh.Ptr("000000"), Description: gh.Ptr("Not in config")},
	}
	diffs := config.DiffLabels(nil, current)
	if len(diffs) != 1 || diffs[0].Action != config.LabelExtra {
		t.Errorf("expected EXTRA, got %v", diffs)
	}
}

func TestDiffLabels_NoPrune(t *testing.T) {
	ghc := ghclient.NewMockClient()
	current := []*gh.Label{
		{Name: gh.Ptr("extra"), Color: gh.Ptr("000000"), Description: gh.Ptr("")},
	}
	diffs := config.DiffLabels(nil, current)
	// Apply without prune: should NOT delete extra labels.
	if err := config.ApplyLabelDiffs(context.Background(), ghc, "owner", "repo", diffs, false); err != nil {
		t.Fatalf("ApplyLabelDiffs() error = %v", err)
	}
	if len(ghc.RepoLabels) != 0 {
		t.Error("expected no label mutations when not pruning")
	}
}

func TestDiffLabels_WithPrune(t *testing.T) {
	ghc := ghclient.NewMockClient()
	ghc.RepoLabels["extra"] = &gh.Label{Name: gh.Ptr("extra")}
	current := []*gh.Label{
		{Name: gh.Ptr("extra"), Color: gh.Ptr("000000"), Description: gh.Ptr("")},
	}
	diffs := config.DiffLabels(nil, current)
	if err := config.ApplyLabelDiffs(context.Background(), ghc, "owner", "repo", diffs, true); err != nil {
		t.Fatalf("ApplyLabelDiffs() error = %v", err)
	}
	if _, ok := ghc.RepoLabels["extra"]; ok {
		t.Error("expected extra label to be deleted when pruning")
	}
}

func TestApplyLabelDiffs_Create(t *testing.T) {
	ghc := ghclient.NewMockClient()
	diffs := []config.LabelDiff{
		{
			Name:    "lgtm",
			Action:  config.LabelCreate,
			Desired: &config.LabelDefinition{Name: "lgtm", Color: "0e8a16", Description: "desc"},
		},
	}
	if err := config.ApplyLabelDiffs(context.Background(), ghc, "o", "r", diffs, false); err != nil {
		t.Fatalf("ApplyLabelDiffs() error = %v", err)
	}
	if _, ok := ghc.RepoLabels["lgtm"]; !ok {
		t.Error("expected lgtm label to be created")
	}
}

func TestPrintLabelPlan(t *testing.T) {
	diffs := []config.LabelDiff{
		{Name: "lgtm", Action: config.LabelCreate, Desired: &config.LabelDefinition{Name: "lgtm", Color: "0e8a16", Description: "desc"}},
		{Name: "approved", Action: config.LabelOK, Desired: &config.LabelDefinition{Name: "approved"}},
		{Name: "hold", Action: config.LabelUpdate, Desired: &config.LabelDefinition{Name: "hold"}, Current: &gh.Label{Name: gh.Ptr("hold")}},
		{Name: "extra", Action: config.LabelExtra, Current: &gh.Label{Name: gh.Ptr("extra")}},
	}
	// Just ensure it doesn't panic and produces output.
	config.PrintLabelPlan(os.Stdout, diffs)
}
