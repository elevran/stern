package commands_test

import (
	"context"
	"errors"
	"testing"

	"github.com/elevran/stern/internal/commands"
	"github.com/elevran/stern/internal/config"
)

func priorityOpts() *config.Options {
	return &config.Options{
		Priority: config.PriorityOptions{
			Values: []string{"P0", "P1", "P2"},
		},
	}
}

func TestPriority_AddsLabel(t *testing.T) {
	sc, ghc := prContext("author")
	reg := commands.Registry{"priority": commands.NewPriorityHandler}
	commands.Dispatch(context.Background(), sc, "/priority P0", reg, ghc, priorityOpts())

	if !ghc.IssueLabels[1]["priority/P0"] {
		t.Error("expected priority/P0 label to be added")
	}
	if len(ghc.Reactions) == 0 || ghc.Reactions[0].Content != "+1" {
		t.Errorf("expected +1 reaction after successful /priority, got %v", ghc.Reactions)
	}
}

func TestPriority_MutuallyExclusive(t *testing.T) {
	// Setting P1 when P0 already exists should remove P0 first.
	sc, ghc := prContext("author")
	sc.PR.Labels = []string{"priority/P0"}
	ghc.IssueLabels[1] = map[string]bool{"priority/P0": true}

	reg := commands.Registry{"priority": commands.NewPriorityHandler}
	commands.Dispatch(context.Background(), sc, "/priority P1", reg, ghc, priorityOpts())

	if ghc.IssueLabels[1]["priority/P0"] {
		t.Error("expected existing priority/P0 label to be removed (mutual exclusion)")
	}
	if !ghc.IssueLabels[1]["priority/P1"] {
		t.Error("expected priority/P1 label to be added")
	}
}

func TestPriority_RemovesAllPriorityLabels(t *testing.T) {
	// Multiple pre-existing priority/* labels should all be removed.
	sc, ghc := prContext("author")
	sc.PR.Labels = []string{"priority/P0", "priority/P1", "area/api"}
	ghc.IssueLabels[1] = map[string]bool{
		"priority/P0": true,
		"priority/P1": true,
		"area/api":    true,
	}

	reg := commands.Registry{"priority": commands.NewPriorityHandler}
	commands.Dispatch(context.Background(), sc, "/priority P2", reg, ghc, priorityOpts())

	if ghc.IssueLabels[1]["priority/P0"] {
		t.Error("expected priority/P0 to be removed")
	}
	if ghc.IssueLabels[1]["priority/P1"] {
		t.Error("expected priority/P1 to be removed")
	}
	if !ghc.IssueLabels[1]["priority/P2"] {
		t.Error("expected priority/P2 to be added")
	}
	// Non-priority labels must be untouched.
	if !ghc.IssueLabels[1]["area/api"] {
		t.Error("expected non-priority label area/api to remain untouched")
	}
}

func TestPriority_Cancel_RemovesAll(t *testing.T) {
	sc, ghc := prContext("author")
	sc.PR.Labels = []string{"priority/P0", "priority/P1"}
	ghc.IssueLabels[1] = map[string]bool{
		"priority/P0": true,
		"priority/P1": true,
	}

	reg := commands.Registry{"priority": commands.NewPriorityHandler}
	commands.Dispatch(context.Background(), sc, "/priority cancel", reg, ghc, priorityOpts())

	if ghc.IssueLabels[1]["priority/P0"] {
		t.Error("expected priority/P0 removed on cancel")
	}
	if ghc.IssueLabels[1]["priority/P1"] {
		t.Error("expected priority/P1 removed on cancel")
	}
	if len(ghc.Reactions) == 0 || ghc.Reactions[0].Content != "+1" {
		t.Errorf("expected +1 reaction after successful /priority cancel, got %v", ghc.Reactions)
	}
}

func TestPriority_NoArg_RemovesAll(t *testing.T) {
	sc, ghc := prContext("author")
	sc.PR.Labels = []string{"priority/P0"}
	ghc.IssueLabels[1] = map[string]bool{"priority/P0": true}

	reg := commands.Registry{"priority": commands.NewPriorityHandler}
	commands.Dispatch(context.Background(), sc, "/priority", reg, ghc, priorityOpts())

	if ghc.IssueLabels[1]["priority/P0"] {
		t.Error("expected priority/P0 removed on /priority with no arg")
	}
}

func TestPriority_InvalidValue(t *testing.T) {
	sc, ghc := prContext("author")
	reg := commands.Registry{"priority": commands.NewPriorityHandler}
	commands.Dispatch(context.Background(), sc, "/priority P9", reg, ghc, priorityOpts())

	if len(ghc.IssueLabels[1]) > 0 {
		t.Errorf("expected no labels added for invalid priority, got %v", ghc.IssueLabels[1])
	}
	if len(ghc.Reactions) == 0 || ghc.Reactions[0].Content != "-1" {
		t.Errorf("expected -1 reaction for invalid priority, got %v", ghc.Reactions)
	}
}

func TestPriority_NotOnPR(t *testing.T) {
	sc, ghc := prContext("author")
	sc.PR = nil

	reg := commands.Registry{"priority": commands.NewPriorityHandler}
	commands.Dispatch(context.Background(), sc, "/priority P0", reg, ghc, priorityOpts())

	if len(ghc.Reactions) == 0 || ghc.Reactions[0].Content != "-1" {
		t.Errorf("expected -1 for /priority on non-PR, got %v", ghc.Reactions)
	}
}

func TestPriority_HandleError_SuppressesPost(t *testing.T) {
	sc, ghc := prContext("author")
	ghc.Errors["AddLabels"] = errors.New("boom")

	reg := commands.Registry{"priority": commands.NewPriorityHandler}
	commands.Dispatch(context.Background(), sc, "/priority P0", reg, ghc, priorityOpts())

	if len(ghc.AutoMergeEnabled) > 0 || len(ghc.AutoMergeDisabled) > 0 {
		t.Errorf("expected Post NOT to run when Handle errors, got enabled=%v disabled=%v",
			ghc.AutoMergeEnabled, ghc.AutoMergeDisabled)
	}
	if len(ghc.Reactions) == 0 || ghc.Reactions[0].Content != "confused" {
		t.Errorf("expected confused reaction on internal error, got %v", ghc.Reactions)
	}
}
