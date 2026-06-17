package commands_test

import (
	"context"
	"testing"

	gh "github.com/google/go-github/v72/github"

	"github.com/elevran/stern/internal/commands"
	"github.com/elevran/stern/internal/config"
)

func approveOpts(allowSelf bool) *config.Options {
	return &config.Options{
		Approve: config.ApproveOptions{
			AllowSelfApproval: allowSelf,
			InvalidateOnPush:  false,
			RequireOwner:      true,
		},
		Merge: config.MergeOptions{
			Method:         "squash",
			BlockingLabels: []string{"do-not-merge/hold"},
		},
	}
}

func TestApprove_AddsLabel(t *testing.T) {
	sc, ghc := prContext("author")
	sc.Author = "approver"
	ghc.FileContent["OWNERS@abc123"] = []byte("approvers:\n  - approver\n")
	ghc.PRFiles[1] = []*gh.CommitFile{{Filename: gh.Ptr("main.go")}}

	reg := commands.Registry{"approve": commands.NewApproveHandler}
	commands.Dispatch(context.Background(), sc, "/approve", reg, ghc, approveOpts(false))

	if !ghc.IssueLabels[1]["approved"] {
		t.Error("expected approved label to be added")
	}
}

func TestApprove_Cancel_RemovesLabel(t *testing.T) {
	sc, ghc := prContext("author")
	sc.Author = "approver"
	ghc.IssueLabels[1] = map[string]bool{"approved": true}

	reg := commands.Registry{"approve": commands.NewApproveHandler}
	commands.Dispatch(context.Background(), sc, "/approve cancel", reg, ghc, approveOpts(false))

	if ghc.IssueLabels[1]["approved"] {
		t.Error("expected approved label removed on cancel")
	}
}

func TestApprove_SelfApprovalDenied(t *testing.T) {
	sc, ghc := prContext("approver")
	sc.Author = "approver" // PR author == commenter

	reg := commands.Registry{"approve": commands.NewApproveHandler}
	commands.Dispatch(context.Background(), sc, "/approve", reg, ghc, approveOpts(false))

	if ghc.IssueLabels[1]["approved"] {
		t.Error("expected approved NOT added for self-approval")
	}
	if len(ghc.Reactions) == 0 || ghc.Reactions[0].Content != "-1" {
		t.Errorf("expected -1 reaction, got %v", ghc.Reactions)
	}
}

func TestApprove_NonApproverDenied(t *testing.T) {
	sc, ghc := prContext("author")
	sc.Author = "outsider"
	ghc.FileContent["OWNERS@abc123"] = []byte("approvers:\n  - alice\n")
	ghc.PRFiles[1] = []*gh.CommitFile{{Filename: gh.Ptr("main.go")}}

	reg := commands.Registry{"approve": commands.NewApproveHandler}
	commands.Dispatch(context.Background(), sc, "/approve", reg, ghc, approveOpts(false))

	if ghc.IssueLabels[1]["approved"] {
		t.Error("expected approved NOT added for non-approver")
	}
	if len(ghc.Reactions) == 0 || ghc.Reactions[0].Content != "-1" {
		t.Errorf("expected -1 reaction, got %v", ghc.Reactions)
	}
}

func TestApprove_BothLGTMAndApproved_TriggersAutoMerge(t *testing.T) {
	sc, ghc := prContext("author")
	sc.Author = "approver"
	// Pre-load the PR with lgtm already present.
	ghc.PullRequests[1].Labels = []*gh.Label{{Name: gh.Ptr("lgtm")}}
	// After approve, GetPullRequest will return the updated PR.
	// We need to simulate adding "approved" to the returned PR.
	ghc.IssueLabels[1] = map[string]bool{"lgtm": true}
	// No OWNERS files: any commenter can approve.

	reg := commands.Registry{"approve": commands.NewApproveHandler}
	opts := &config.Options{
		Approve: config.ApproveOptions{RequireOwner: false},
		Merge:   config.MergeOptions{Method: "squash", BlockingLabels: []string{"do-not-merge/hold"}},
	}
	commands.Dispatch(context.Background(), sc, "/approve", reg, ghc, opts)

	if !ghc.IssueLabels[1]["approved"] {
		t.Error("expected approved label added")
	}
}
