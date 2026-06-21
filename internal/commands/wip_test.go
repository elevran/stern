package commands_test

import (
	"context"
	"errors"
	"testing"

	"github.com/elevran/stern/internal/commands"
	"github.com/elevran/stern/internal/config"
	"github.com/elevran/stern/internal/github"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

	assert.True(t, ghc.IssueLabels[1]["do-not-merge/wip"], "expected wip label added")
	require.NotEmpty(t, ghc.Reactions)
	assert.Equal(t, "+1", ghc.Reactions[0].Content)
}

func TestWIP_RemovesLabel(t *testing.T) {
	sc, ghc := prContext("author")
	sc.PR.Labels = []string{"do-not-merge/wip"}
	ghc.IssueLabels[1] = map[string]bool{"do-not-merge/wip": true}

	reg := commands.Registry{"wip": commands.NewWIPHandler}
	commands.Dispatch(context.Background(), sc, "/wip", reg, ghc, wipOpts())

	assert.False(t, ghc.IssueLabels[1]["do-not-merge/wip"], "expected wip label removed")
	require.NotEmpty(t, ghc.Reactions)
	assert.Equal(t, "+1", ghc.Reactions[0].Content)
}

func TestWIP_Cancel_ReenablesAutoMerge_WhenEligible(t *testing.T) {
	sc, ghc := prContext("author")
	// Set wip on sc.PR so handler sees hasWIP=true.
	sc.PR.Labels = []string{"do-not-merge/wip"}
	ghc.IssueLabels[1] = map[string]bool{"do-not-merge/wip": true}
	// Replace stored PR with post-cancel state (lgtm + approved → eligible).
	ghc.PullRequests[1] = &github.PullRequest{
		Number: 1,
		NodeID: "test-node-id",
		Author: "author",
		Labels: []string{"lgtm", "approved"},
	}

	reg := commands.Registry{"wip": commands.NewWIPHandler}
	commands.Dispatch(context.Background(), sc, "/wip", reg, ghc, wipOpts())

	assert.False(t, ghc.IssueLabels[1]["do-not-merge/wip"], "expected wip label removed")
	assert.NotEmpty(t, ghc.AutoMergeEnabled, "expected EnableAutoMerge called when PR becomes eligible after wip cancel")
	assert.Empty(t, ghc.AutoMergeDisabled, "expected DisableAutoMerge NOT called when PR is eligible")
}

func TestWIP_AddsLabel_DisablesAutoMerge(t *testing.T) {
	sc, ghc := prContext("author")
	reg := commands.Registry{"wip": commands.NewWIPHandler}
	commands.Dispatch(context.Background(), sc, "/wip", reg, ghc, wipOpts())

	assert.NotEmpty(t, ghc.AutoMergeDisabled, "expected DisableAutoMerge called when wip label added")
}

func TestWIP_NotOnPR(t *testing.T) {
	sc, ghc := prContext("author")
	sc.PR = nil

	reg := commands.Registry{"wip": commands.NewWIPHandler}
	commands.Dispatch(context.Background(), sc, "/wip", reg, ghc, wipOpts())

	require.NotEmpty(t, ghc.Reactions)
	assert.Equal(t, "-1", ghc.Reactions[0].Content, "expected -1 for /wip on non-PR")
}

func TestWIP_HandleError_SuppressesPost(t *testing.T) {
	sc, ghc := prContext("author")
	ghc.Errors["AddLabels"] = errors.New("boom")

	reg := commands.Registry{"wip": commands.NewWIPHandler}
	commands.Dispatch(context.Background(), sc, "/wip", reg, ghc, wipOpts())

	assert.Empty(t, ghc.AutoMergeEnabled, "expected Post NOT to run when Handle errors")
	assert.Empty(t, ghc.AutoMergeDisabled, "expected Post NOT to run when Handle errors")
	require.NotEmpty(t, ghc.Reactions)
	assert.Equal(t, "confused", ghc.Reactions[0].Content, "expected confused reaction on internal error")
}
