package config_test

import (
	"context"
	"os"
	"testing"

	gh "github.com/google/go-github/v72/github"

	"github.com/elevran/stern/internal/config"
	"github.com/elevran/stern/internal/ghclient"
)

func TestDiffLabels_Create(t *testing.T) {
	desired := []config.LabelDefinition{
		{Name: "lgtm", Color: "0e8a16", Description: "Looks good to me"},
	}
	plan := config.DiffLabels(desired, nil)
	if len(plan.Creates) != 1 {
		t.Errorf("expected 1 create, got %d", len(plan.Creates))
	}
}

func TestDiffLabels_OK(t *testing.T) {
	desired := []config.LabelDefinition{
		{Name: "lgtm", Color: "0e8a16", Description: "Looks good to me"},
	}
	current := []*gh.Label{
		{Name: gh.Ptr("lgtm"), Color: gh.Ptr("0e8a16"), Description: gh.Ptr("Looks good to me")},
	}
	plan := config.DiffLabels(desired, current)
	if len(plan.Unchanged) != 1 {
		t.Errorf("expected 1 unchanged, got %d", len(plan.Unchanged))
	}
}

func TestDiffLabels_Update_Color(t *testing.T) {
	desired := []config.LabelDefinition{
		{Name: "lgtm", Color: "0e8a16", Description: "Looks good to me"},
	}
	current := []*gh.Label{
		{Name: gh.Ptr("lgtm"), Color: gh.Ptr("ffffff"), Description: gh.Ptr("Looks good to me")},
	}
	plan := config.DiffLabels(desired, current)
	if len(plan.Updates) != 1 {
		t.Errorf("expected 1 update for color change, got %d", len(plan.Updates))
	}
}

func TestDiffLabels_Update_Description(t *testing.T) {
	desired := []config.LabelDefinition{
		{Name: "lgtm", Color: "0e8a16", Description: "New description"},
	}
	current := []*gh.Label{
		{Name: gh.Ptr("lgtm"), Color: gh.Ptr("0e8a16"), Description: gh.Ptr("Old description")},
	}
	plan := config.DiffLabels(desired, current)
	if len(plan.Updates) != 1 {
		t.Errorf("expected 1 update for description change, got %d", len(plan.Updates))
	}
}

func TestDiffLabels_Extra(t *testing.T) {
	current := []*gh.Label{
		{Name: gh.Ptr("extra-label"), Color: gh.Ptr("000000"), Description: gh.Ptr("Not in config")},
	}
	plan := config.DiffLabels(nil, current)
	if len(plan.Extras) != 1 {
		t.Errorf("expected 1 extra, got %d", len(plan.Extras))
	}
}

func TestDiffLabels_NoPrune(t *testing.T) {
	ghc := ghclient.NewMockClient()
	current := []*gh.Label{
		{Name: gh.Ptr("extra"), Color: gh.Ptr("000000"), Description: gh.Ptr("")},
	}
	plan := config.DiffLabels(nil, current)
	if err := plan.Apply(context.Background(), ghc, "owner", "repo", false); err != nil {
		t.Fatalf("Apply() error = %v", err)
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
	plan := config.DiffLabels(nil, current)
	if err := plan.Apply(context.Background(), ghc, "owner", "repo", true); err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	if _, ok := ghc.RepoLabels["extra"]; ok {
		t.Error("expected extra label to be deleted when pruning")
	}
}

func TestApply_Create(t *testing.T) {
	ghc := ghclient.NewMockClient()
	desired := []config.LabelDefinition{
		{Name: "lgtm", Color: "0e8a16", Description: "desc"},
	}
	plan := config.DiffLabels(desired, nil)
	if err := plan.Apply(context.Background(), ghc, "o", "r", false); err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	if _, ok := ghc.RepoLabels["lgtm"]; !ok {
		t.Error("expected lgtm label to be created")
	}
}

func TestPrint(t *testing.T) {
	desired := []config.LabelDefinition{
		{Name: "lgtm", Color: "0e8a16", Description: "desc"},
		{Name: "approved", Color: "0e8a16", Description: "approved"},
		{Name: "hold", Color: "e11d48", Description: "hold"},
	}
	current := []*gh.Label{
		{Name: gh.Ptr("approved"), Color: gh.Ptr("0e8a16"), Description: gh.Ptr("approved")},
		{Name: gh.Ptr("hold"), Color: gh.Ptr("ffffff"), Description: gh.Ptr("hold")},
		{Name: gh.Ptr("extra"), Color: gh.Ptr("000000"), Description: gh.Ptr("")},
	}
	plan := config.DiffLabels(desired, current)
	// Just ensure it doesn't panic and produces output.
	plan.Print(os.Stdout)
}
