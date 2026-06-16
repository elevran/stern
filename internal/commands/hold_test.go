package commands_test

import (
	"context"
	"testing"

	"github.com/elevran/stern/internal/commands"
	"github.com/elevran/stern/internal/config"
)

func holdOpts() *config.Options {
	return &config.Options{
		Merge: config.MergeOptions{
			Method:         "squash",
			BlockingLabels: []string{"do-not-merge/hold"},
		},
	}
}

func TestHold_AddsLabel(t *testing.T) {
	sc, ghc := prContext("author")
	reg := commands.Registry{"hold": &commands.HoldHandler{}}
	commands.Dispatch(context.Background(), sc, "/hold", reg, ghc, holdOpts())

	if !ghc.IssueLabels[1]["do-not-merge/hold"] {
		t.Error("expected hold label to be added")
	}
}

func TestHold_AnyOrgMemberCanHold(t *testing.T) {
	sc, ghc := prContext("author")
	sc.Author = "contributor" // non-write user
	ghc.WriteAccess["elevran/stern/contributor"] = false

	reg := commands.Registry{"hold": &commands.HoldHandler{}}
	commands.Dispatch(context.Background(), sc, "/hold", reg, ghc, holdOpts())

	if !ghc.IssueLabels[1]["do-not-merge/hold"] {
		t.Error("expected any org member to be able to hold")
	}
}

func TestHold_Cancel_RequiresWriteAccess(t *testing.T) {
	sc, ghc := prContext("author")
	sc.Author = "reader"
	ghc.WriteAccess["elevran/stern/reader"] = false
	ghc.IssueLabels[1] = map[string]bool{"do-not-merge/hold": true}

	reg := commands.Registry{"hold": &commands.HoldHandler{}}
	commands.Dispatch(context.Background(), sc, "/hold cancel", reg, ghc, holdOpts())

	if !ghc.IssueLabels[1]["do-not-merge/hold"] {
		t.Error("expected hold label NOT removed without write access")
	}
	if len(ghc.Reactions) == 0 || ghc.Reactions[0].Content != "-1" {
		t.Errorf("expected -1 reaction, got %v", ghc.Reactions)
	}
}

func TestHold_Cancel_WithWriteAccess(t *testing.T) {
	sc, ghc := prContext("author")
	sc.Author = "maintainer"
	ghc.WriteAccess["elevran/stern/maintainer"] = true
	ghc.IssueLabels[1] = map[string]bool{"do-not-merge/hold": true}

	reg := commands.Registry{"hold": &commands.HoldHandler{}}
	commands.Dispatch(context.Background(), sc, "/hold cancel", reg, ghc, holdOpts())

	if ghc.IssueLabels[1]["do-not-merge/hold"] {
		t.Error("expected hold label removed for writer")
	}
}

func TestHold_NotOnPR(t *testing.T) {
	sc, ghc := prContext("author")
	sc.PR = nil

	reg := commands.Registry{"hold": &commands.HoldHandler{}}
	commands.Dispatch(context.Background(), sc, "/hold", reg, ghc, holdOpts())

	if len(ghc.Reactions) == 0 || ghc.Reactions[0].Content != "-1" {
		t.Errorf("expected -1 for /hold on non-PR, got %v", ghc.Reactions)
	}
}
