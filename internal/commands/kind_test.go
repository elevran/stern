package commands_test

import (
	"context"
	"errors"
	"testing"

	"github.com/elevran/stern/internal/commands"
	"github.com/elevran/stern/internal/config"
)

func kindOpts() *config.Options {
	return &config.Options{
		Kind: config.KindOptions{
			Values: []string{"bug", "feature", "docs"},
		},
	}
}

func TestKind_AddsLabel(t *testing.T) {
	sc, ghc := prContext("author")
	reg := commands.Registry{"kind": commands.NewKindHandler}
	commands.Dispatch(context.Background(), sc, "/kind bug", reg, ghc, kindOpts())

	if !ghc.IssueLabels[1]["kind/bug"] {
		t.Error("expected kind/bug label to be added")
	}
	if len(ghc.Reactions) == 0 || ghc.Reactions[0].Content != "+1" {
		t.Errorf("expected +1 reaction after successful /kind, got %v", ghc.Reactions)
	}
}

func TestKind_AllowsMultipleValues(t *testing.T) {
	// /kind and /area do NOT enforce mutual exclusion — multiple values may coexist.
	sc, ghc := prContext("author")
	sc.PR.Labels = []string{"kind/bug"}
	ghc.IssueLabels[1] = map[string]bool{"kind/bug": true}

	reg := commands.Registry{"kind": commands.NewKindHandler}
	commands.Dispatch(context.Background(), sc, "/kind feature", reg, ghc, kindOpts())

	if !ghc.IssueLabels[1]["kind/bug"] {
		t.Error("expected pre-existing kind/bug label to remain (no mutual exclusion)")
	}
	if !ghc.IssueLabels[1]["kind/feature"] {
		t.Error("expected new kind/feature label to be added")
	}
}

func TestKind_InvalidValue(t *testing.T) {
	sc, ghc := prContext("author")
	reg := commands.Registry{"kind": commands.NewKindHandler}
	commands.Dispatch(context.Background(), sc, "/kind unknown", reg, ghc, kindOpts())

	if len(ghc.IssueLabels[1]) > 0 {
		t.Errorf("expected no labels added for invalid kind, got %v", ghc.IssueLabels[1])
	}
	if len(ghc.Reactions) == 0 || ghc.Reactions[0].Content != "-1" {
		t.Errorf("expected -1 reaction for invalid kind, got %v", ghc.Reactions)
	}
}

func TestKind_NoArg(t *testing.T) {
	sc, ghc := prContext("author")
	reg := commands.Registry{"kind": commands.NewKindHandler}
	commands.Dispatch(context.Background(), sc, "/kind", reg, ghc, kindOpts())

	if len(ghc.Reactions) == 0 || ghc.Reactions[0].Content != "-1" {
		t.Errorf("expected -1 reaction for /kind with no arg, got %v", ghc.Reactions)
	}
}

func TestKind_NotOnPR(t *testing.T) {
	sc, ghc := prContext("author")
	sc.PR = nil

	reg := commands.Registry{"kind": commands.NewKindHandler}
	commands.Dispatch(context.Background(), sc, "/kind bug", reg, ghc, kindOpts())

	if len(ghc.Reactions) == 0 || ghc.Reactions[0].Content != "-1" {
		t.Errorf("expected -1 for /kind on non-PR, got %v", ghc.Reactions)
	}
}

func TestKind_HandleError_SuppressesPost(t *testing.T) {
	sc, ghc := prContext("author")
	ghc.Errors["AddLabels"] = errors.New("boom")

	reg := commands.Registry{"kind": commands.NewKindHandler}
	commands.Dispatch(context.Background(), sc, "/kind bug", reg, ghc, kindOpts())

	if len(ghc.AutoMergeEnabled) > 0 || len(ghc.AutoMergeDisabled) > 0 {
		t.Errorf("expected Post NOT to run when Handle errors, got enabled=%v disabled=%v",
			ghc.AutoMergeEnabled, ghc.AutoMergeDisabled)
	}
	if len(ghc.Reactions) == 0 || ghc.Reactions[0].Content != "confused" {
		t.Errorf("expected confused reaction on internal error, got %v", ghc.Reactions)
	}
}
