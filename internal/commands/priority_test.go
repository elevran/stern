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

	assert.True(t, ghc.IssueLabels[1]["priority/P0"], "expected priority/P0 label to be added")
	require.NotEmpty(t, ghc.Reactions)
	assert.Equal(t, "+1", ghc.Reactions[0].Content)
}

func TestPriority_MutuallyExclusive(t *testing.T) {
	// Setting P1 when P0 already exists should remove P0 first.
	sc, ghc := prContext("author")
	sc.PR.Labels = []string{"priority/P0"}
	ghc.IssueLabels[1] = map[string]bool{"priority/P0": true}

	reg := commands.Registry{"priority": commands.NewPriorityHandler}
	commands.Dispatch(context.Background(), sc, "/priority P1", reg, ghc, priorityOpts())

	assert.False(t, ghc.IssueLabels[1]["priority/P0"], "expected existing priority/P0 label to be removed (mutual exclusion)")
	assert.True(t, ghc.IssueLabels[1]["priority/P1"], "expected priority/P1 label to be added")
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

	assert.False(t, ghc.IssueLabels[1]["priority/P0"], "expected priority/P0 to be removed")
	assert.False(t, ghc.IssueLabels[1]["priority/P1"], "expected priority/P1 to be removed")
	assert.True(t, ghc.IssueLabels[1]["priority/P2"], "expected priority/P2 to be added")
	// Non-priority labels must be untouched.
	assert.True(t, ghc.IssueLabels[1]["area/api"], "expected non-priority label area/api to remain untouched")
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

	assert.False(t, ghc.IssueLabels[1]["priority/P0"], "expected priority/P0 removed on cancel")
	assert.False(t, ghc.IssueLabels[1]["priority/P1"], "expected priority/P1 removed on cancel")
	require.NotEmpty(t, ghc.Reactions)
	assert.Equal(t, "+1", ghc.Reactions[0].Content)
}

func TestPriority_NoArg_RemovesAll(t *testing.T) {
	sc, ghc := prContext("author")
	sc.PR.Labels = []string{"priority/P0"}
	ghc.IssueLabels[1] = map[string]bool{"priority/P0": true}

	reg := commands.Registry{"priority": commands.NewPriorityHandler}
	commands.Dispatch(context.Background(), sc, "/priority", reg, ghc, priorityOpts())

	assert.False(t, ghc.IssueLabels[1]["priority/P0"], "expected priority/P0 removed on /priority with no arg")
}

func TestPriority_InvalidValue(t *testing.T) {
	sc, ghc := prContext("author")
	reg := commands.Registry{"priority": commands.NewPriorityHandler}
	commands.Dispatch(context.Background(), sc, "/priority P9", reg, ghc, priorityOpts())

	assert.Empty(t, ghc.IssueLabels[1], "expected no labels added for invalid priority")
	require.NotEmpty(t, ghc.Reactions)
	assert.Equal(t, "-1", ghc.Reactions[0].Content)
}

func TestPriority_NotOnPR(t *testing.T) {
	sc, ghc := prContext("author")
	sc.PR = nil

	reg := commands.Registry{"priority": commands.NewPriorityHandler}
	commands.Dispatch(context.Background(), sc, "/priority P0", reg, ghc, priorityOpts())

	require.NotEmpty(t, ghc.Reactions)
	assert.Equal(t, "-1", ghc.Reactions[0].Content, "expected -1 for /priority on non-PR")
}

func TestPriority_HandleError_SuppressesPost(t *testing.T) {
	sc, ghc := prContext("author")
	ghc.Errors["AddLabels"] = errors.New("boom")

	reg := commands.Registry{"priority": commands.NewPriorityHandler}
	commands.Dispatch(context.Background(), sc, "/priority P0", reg, ghc, priorityOpts())

	assert.Empty(t, ghc.AutoMergeEnabled, "expected Post NOT to run when Handle errors")
	assert.Empty(t, ghc.AutoMergeDisabled, "expected Post NOT to run when Handle errors")
	require.NotEmpty(t, ghc.Reactions)
	assert.Equal(t, "confused", ghc.Reactions[0].Content, "expected confused reaction on internal error")
}