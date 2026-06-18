package commands_test

import (
	"context"
	"testing"

	gh "github.com/google/go-github/v72/github"

	"github.com/elevran/stern/internal/commands"
	"github.com/elevran/stern/internal/config"
)

func wipOpts() *config.Options {
	return &config.Options{
		Merge: config.MergeOptions{
			Method:         "squash",
			BlockingLabels: []string{"do-not-merge/wip"},
		},
	}
}

func TestWIP_AddsLabel(t *testing.T) {
	sc, ghc := prContext("author")
	reg := commands.Registry{"wip": commands.NewWIPHandler}
	commands.Dispatch(context.Background(), sc, "/wip", reg, ghc, wipOpts())

	if !ghc.IssueLabels[1]["do-not-merge/wip"] {
		t.Error("expected wip label added")
	}
	if len(ghc.Reactions) == 0 || ghc.Reactions[0].Content != "+1" {
		t.Errorf("expected +1 reaction after successful /wip, got %v", ghc.Reactions)
	}
}

func TestWIP_RemovesLabel(t *testing.T) {
	sc, ghc := prContext("author")
	sc.PR.Labels = []*gh.Label{{Name: gh.Ptr("do-not-merge/wip")}}
	ghc.IssueLabels[1] = map[string]bool{"do-not-merge/wip": true}

	reg := commands.Registry{"wip": commands.NewWIPHandler}
	commands.Dispatch(context.Background(), sc, "/wip", reg, ghc, wipOpts())

	if ghc.IssueLabels[1]["do-not-merge/wip"] {
		t.Error("expected wip label removed")
	}
	if len(ghc.Reactions) == 0 || ghc.Reactions[0].Content != "+1" {
		t.Errorf("expected +1 reaction after successful /wip toggle-off, got %v", ghc.Reactions)
	}
}

func TestWIP_NotOnPR(t *testing.T) {
	sc, ghc := prContext("author")
	sc.PR = nil

	reg := commands.Registry{"wip": commands.NewWIPHandler}
	commands.Dispatch(context.Background(), sc, "/wip", reg, ghc, wipOpts())

	if len(ghc.Reactions) == 0 || ghc.Reactions[0].Content != "-1" {
		t.Errorf("expected -1 for /wip on non-PR, got %v", ghc.Reactions)
	}
}
