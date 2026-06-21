package config_test

import (
	"context"
	"os"
	"testing"

	"github.com/elevran/stern/internal/config"
	"github.com/elevran/stern/internal/github"
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
	current := []github.Label{
		{Name: "lgtm", Color: "0e8a16", Description: "Looks good to me"},
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
	current := []github.Label{
		{Name: "lgtm", Color: "ffffff", Description: "Looks good to me"},
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
	current := []github.Label{
		{Name: "lgtm", Color: "0e8a16", Description: "Old description"},
	}
	plan := config.DiffLabels(desired, current)
	if len(plan.Updates) != 1 {
		t.Errorf("expected 1 update for description change, got %d", len(plan.Updates))
	}
}

func TestDiffLabels_Extra(t *testing.T) {
	current := []github.Label{
		{Name: "extra-label", Color: "000000", Description: "Not in config"},
	}
	plan := config.DiffLabels(nil, current)
	if len(plan.Extras) != 1 {
		t.Errorf("expected 1 extra, got %d", len(plan.Extras))
	}
}

func TestDiffLabels_NoPrune(t *testing.T) {
	ghc := github.NewMockClient()
	current := []github.Label{
		{Name: "extra", Color: "000000", Description: ""},
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
	ghc := github.NewMockClient()
	ghc.RepoLabels["extra"] = github.Label{Name: "extra"}
	current := []github.Label{
		{Name: "extra", Color: "000000", Description: ""},
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
	ghc := github.NewMockClient()
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
	current := []github.Label{
		{Name: "approved", Color: "0e8a16", Description: "approved"},
		{Name: "hold", Color: "ffffff", Description: "hold"},
		{Name: "extra", Color: "000000", Description: ""},
	}
	plan := config.DiffLabels(desired, current)
	// Just ensure it doesn't panic and produces output.
	plan.Print(os.Stdout)
}
