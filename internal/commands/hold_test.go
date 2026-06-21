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
	reg := commands.Registry{"hold": commands.NewHoldHandler}
	commands.Dispatch(context.Background(), sc, "/hold", reg, ghc, holdOpts())

	assert.True(t, ghc.IssueLabels[1]["do-not-merge/hold"], "expected hold label to be added")
	require.NotEmpty(t, ghc.Reactions)
	assert.Equal(t, "+1", ghc.Reactions[0].Content)
}

func TestHold_AnyOrgMemberCanHold(t *testing.T) {
	sc, ghc := prContext("author")
	sc.Author = "contributor" // non-write user
	ghc.WriteAccess["elevran/stern/contributor"] = false

	reg := commands.Registry{"hold": commands.NewHoldHandler}
	commands.Dispatch(context.Background(), sc, "/hold", reg, ghc, holdOpts())

	assert.True(t, ghc.IssueLabels[1]["do-not-merge/hold"], "expected any org member to be able to hold")
	require.NotEmpty(t, ghc.Reactions)
	assert.Equal(t, "+1", ghc.Reactions[0].Content)
}

func TestHold_Cancel_RequiresWriteAccess(t *testing.T) {
	sc, ghc := prContext("author")
	sc.Author = "reader"
	ghc.WriteAccess["elevran/stern/reader"] = false
	ghc.IssueLabels[1] = map[string]bool{"do-not-merge/hold": true}

	reg := commands.Registry{"hold": commands.NewHoldHandler}
	commands.Dispatch(context.Background(), sc, "/hold cancel", reg, ghc, holdOpts())

	assert.True(t, ghc.IssueLabels[1]["do-not-merge/hold"], "expected hold label NOT removed without write access")
	require.NotEmpty(t, ghc.Reactions)
	assert.Equal(t, "-1", ghc.Reactions[0].Content)
}

func TestHold_Cancel_WithWriteAccess(t *testing.T) {
	sc, ghc := prContext("author")
	sc.Author = "maintainer"
	ghc.WriteAccess["elevran/stern/maintainer"] = true
	ghc.IssueLabels[1] = map[string]bool{"do-not-merge/hold": true}

	reg := commands.Registry{"hold": commands.NewHoldHandler}
	commands.Dispatch(context.Background(), sc, "/hold cancel", reg, ghc, holdOpts())

	assert.False(t, ghc.IssueLabels[1]["do-not-merge/hold"], "expected hold label removed for writer")
	require.NotEmpty(t, ghc.Reactions)
	assert.Equal(t, "+1", ghc.Reactions[0].Content)
}

func TestHold_Cancel_ReenablesAutoMerge_WhenEligible(t *testing.T) {
	sc, ghc := prContext("author")
	sc.Author = "maintainer"
	ghc.WriteAccess["elevran/stern/maintainer"] = true
	ghc.IssueLabels[1] = map[string]bool{"do-not-merge/hold": true}

	// After hold is removed, GetPullRequest returns a PR with lgtm + approved → eligible.
	ghc.PullRequests[1].Labels = []string{"lgtm", "approved"}

	reg := commands.Registry{"hold": commands.NewHoldHandler}
	commands.Dispatch(context.Background(), sc, "/hold cancel", reg, ghc, holdOpts())

	assert.False(t, ghc.IssueLabels[1]["do-not-merge/hold"], "expected hold label removed")
	assert.NotEmpty(t, ghc.AutoMergeEnabled, "expected EnableAutoMerge called when PR becomes eligible after hold cancel")
	assert.Empty(t, ghc.AutoMergeDisabled, "expected DisableAutoMerge NOT called when PR is eligible")
}

func TestHold_AddsLabel_DisablesAutoMerge(t *testing.T) {
	sc, ghc := prContext("author")
	reg := commands.Registry{"hold": commands.NewHoldHandler}
	commands.Dispatch(context.Background(), sc, "/hold", reg, ghc, holdOpts())

	assert.NotEmpty(t, ghc.AutoMergeDisabled, "expected DisableAutoMerge called when hold label added")
}

func TestHold_NotOnPR(t *testing.T) {
	sc, ghc := prContext("author")
	sc.PR = nil

	reg := commands.Registry{"hold": commands.NewHoldHandler}
	commands.Dispatch(context.Background(), sc, "/hold", reg, ghc, holdOpts())

	require.NotEmpty(t, ghc.Reactions)
	assert.Equal(t, "-1", ghc.Reactions[0].Content, "expected -1 for /hold on non-PR")
}

func TestHold_HandleError_SuppressesPost(t *testing.T) {
	sc, ghc := prContext("author")
	ghc.Errors["AddLabels"] = errors.New("boom")

	reg := commands.Registry{"hold": commands.NewHoldHandler}
	commands.Dispatch(context.Background(), sc, "/hold", reg, ghc, holdOpts())

	assert.Empty(t, ghc.AutoMergeEnabled, "expected Post NOT to run when Handle errors")
	assert.Empty(t, ghc.AutoMergeDisabled, "expected Post NOT to run when Handle errors")
	require.NotEmpty(t, ghc.Reactions)
	assert.Equal(t, "confused", ghc.Reactions[0].Content, "expected confused reaction on internal error")
}