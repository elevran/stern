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

func TestWIP_Cancel_ReenablesAutoMerge_WhenEligible(t *testing.T) {
	sc, ghc := prContext("author")
	// sc.PR and ghc.PullRequests[1] share the same pointer from prContext.
	// Set wip on sc.PR (so handler sees hasWIP=true), then replace ghc.PullRequests[1]
	// with a fresh struct representing the post-cancel state.
	sc.PR.Labels = []*gh.Label{{Name: gh.Ptr("do-not-merge/wip")}}
	ghc.IssueLabels[1] = map[string]bool{"do-not-merge/wip": true}
	ghc.PullRequests[1] = &gh.PullRequest{
		Number: gh.Ptr(1),
		NodeID: gh.Ptr("test-node-id"),
		User:   &gh.User{Login: gh.Ptr("author")},
		Labels: []*gh.Label{
			{Name: gh.Ptr("lgtm")},
			{Name: gh.Ptr("approved")},
		},
	}

	reg := commands.Registry{"wip": commands.NewWIPHandler}
	commands.Dispatch(context.Background(), sc, "/wip", reg, ghc, wipOpts())

	if ghc.IssueLabels[1]["do-not-merge/wip"] {
		t.Error("expected wip label removed")
	}
	if len(ghc.AutoMergeEnabled) == 0 {
		t.Error("expected EnableAutoMerge called when PR becomes eligible after wip cancel")
	}
	if len(ghc.AutoMergeDisabled) > 0 {
		t.Error("expected DisableAutoMerge NOT called when PR is eligible")
	}
}

func TestWIP_AddsLabel_DisablesAutoMerge(t *testing.T) {
	sc, ghc := prContext("author")
	reg := commands.Registry{"wip": commands.NewWIPHandler}
	commands.Dispatch(context.Background(), sc, "/wip", reg, ghc, wipOpts())

	if len(ghc.AutoMergeDisabled) == 0 {
		t.Error("expected DisableAutoMerge called when wip label added")
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
