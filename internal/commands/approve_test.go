package commands_test

import (
	"context"
	"errors"
	"testing"

	"github.com/elevran/stern/internal/commands"
	"github.com/elevran/stern/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	ghc.PRFiles[1] = []string{"main.go"}

	reg := commands.Registry{"approve": {Factory: commands.NewApproveHandler}}
	commands.Dispatch(context.Background(), sc, "/approve", reg, ghc, approveOpts(false))

	assert.True(t, ghc.IssueLabels[1]["approved"], "expected approved label to be added")
	require.NotEmpty(t, ghc.Reactions)
	assert.Equal(t, "+1", ghc.Reactions[0].Content, "expected +1 reaction after successful /approve")
}

func TestApprove_Cancel_RemovesLabel(t *testing.T) {
	sc, ghc := prContext("author")
	sc.Author = "approver"
	ghc.IssueLabels[1] = map[string]bool{"approved": true}

	reg := commands.Registry{"approve": {Factory: commands.NewApproveHandler}}
	commands.Dispatch(context.Background(), sc, "/approve cancel", reg, ghc, approveOpts(false))

	assert.False(t, ghc.IssueLabels[1]["approved"], "expected approved label removed on cancel")
	require.NotEmpty(t, ghc.Reactions)
	assert.Equal(t, "+1", ghc.Reactions[0].Content, "expected +1 reaction after successful /approve cancel")
}

func TestApprove_SelfApprovalDenied(t *testing.T) {
	sc, ghc := prContext("approver")
	sc.Author = "approver" // PR author == commenter

	reg := commands.Registry{"approve": {Factory: commands.NewApproveHandler}}
	commands.Dispatch(context.Background(), sc, "/approve", reg, ghc, approveOpts(false))

	assert.False(t, ghc.IssueLabels[1]["approved"], "expected approved NOT added for self-approval")
	require.NotEmpty(t, ghc.Reactions)
	assert.Equal(t, "-1", ghc.Reactions[0].Content, "expected -1 reaction")
}

func TestApprove_NonApproverDenied(t *testing.T) {
	sc, ghc := prContext("author")
	sc.Author = "outsider"
	ghc.FileContent["OWNERS@abc123"] = []byte("approvers:\n  - alice\n")
	ghc.PRFiles[1] = []string{"main.go"}

	reg := commands.Registry{"approve": {Factory: commands.NewApproveHandler}}
	commands.Dispatch(context.Background(), sc, "/approve", reg, ghc, approveOpts(false))

	assert.False(t, ghc.IssueLabels[1]["approved"], "expected approved NOT added for non-approver")
	require.NotEmpty(t, ghc.Reactions)
	assert.Equal(t, "-1", ghc.Reactions[0].Content, "expected -1 reaction")
}

func TestApprove_BothLGTMAndApproved_TriggersAutoMerge(t *testing.T) {
	sc, ghc := prContext("author")
	sc.Author = "approver"
	// Pre-load the PR with both labels so Post's re-check via GetPullRequest
	// sees an eligible PR and triggers EnableAutoMerge. The mock's AddLabels
	// does not sync back into PullRequests[1].Labels, so we set the final
	// state here.
	ghc.PullRequests[1].Labels = []string{"lgtm", "approved"}
	ghc.IssueLabels[1] = map[string]bool{"lgtm": true, "approved": true}
	// No OWNERS files: any commenter can approve.

	reg := commands.Registry{"approve": {Factory: commands.NewApproveHandler}}
	opts := &config.Options{
		Approve: config.ApproveOptions{RequireOwner: false},
		Merge:   config.MergeOptions{Method: "squash", BlockingLabels: []string{"do-not-merge/hold"}},
	}
	commands.Dispatch(context.Background(), sc, "/approve", reg, ghc, opts)

	assert.True(t, ghc.IssueLabels[1]["approved"], "expected approved label added")
	assert.Equal(t, 1, ghc.EnableAutoMergeCallCount, "expected EnableAutoMergeCallCount=1 after both labels present")
	require.NotEmpty(t, ghc.Reactions)
	assert.Equal(t, "+1", ghc.Reactions[0].Content, "expected +1 reaction after successful /approve")
}

func TestApprove_HandleError_SuppressesPost(t *testing.T) {
	sc, ghc := prContext("author")
	sc.Author = "approver"
	ghc.Errors["AddLabels"] = errors.New("boom")

	reg := commands.Registry{"approve": {Factory: commands.NewApproveHandler}}
	commands.Dispatch(context.Background(), sc, "/approve", reg, ghc, approveOpts(false))

	assert.Empty(t, ghc.AutoMergeEnabled, "expected Post NOT to run when Handle errors")
	assert.Empty(t, ghc.AutoMergeDisabled, "expected Post NOT to run when Handle errors")
	require.NotEmpty(t, ghc.Reactions)
	assert.Equal(t, "confused", ghc.Reactions[0].Content, "expected confused reaction on internal error")
}
