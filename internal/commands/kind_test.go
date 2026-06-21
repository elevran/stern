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

func kindOpts() *config.Options {
	return &config.Options{
		Kind: config.KindOptions{
			Values: []string{"bug", "feature", "docs"},
		},
	}
}

func TestKind_AddsLabel(t *testing.T) {
	sc, ghc := prContext("author")
	reg := commands.Registry{"kind": {Factory: commands.NewKindHandler}}
	commands.Dispatch(context.Background(), sc, "/kind bug", reg, ghc, kindOpts())

	assert.True(t, ghc.IssueLabels[1]["kind/bug"], "expected kind/bug label to be added")
	require.NotEmpty(t, ghc.Reactions)
	assert.Equal(t, "+1", ghc.Reactions[0].Content)
}

func TestKind_AllowsMultipleValues(t *testing.T) {
	// /kind and /area do NOT enforce mutual exclusion — multiple values may coexist.
	sc, ghc := prContext("author")
	sc.PR.Labels = []string{"kind/bug"}
	ghc.IssueLabels[1] = map[string]bool{"kind/bug": true}

	reg := commands.Registry{"kind": {Factory: commands.NewKindHandler}}
	commands.Dispatch(context.Background(), sc, "/kind feature", reg, ghc, kindOpts())

	assert.True(t, ghc.IssueLabels[1]["kind/bug"], "expected pre-existing kind/bug label to remain (no mutual exclusion)")
	assert.True(t, ghc.IssueLabels[1]["kind/feature"], "expected new kind/feature label to be added")
}

func TestKind_InvalidValue(t *testing.T) {
	sc, ghc := prContext("author")
	reg := commands.Registry{"kind": {Factory: commands.NewKindHandler}}
	commands.Dispatch(context.Background(), sc, "/kind unknown", reg, ghc, kindOpts())

	assert.Empty(t, ghc.IssueLabels[1], "expected no labels added for invalid kind")
	require.NotEmpty(t, ghc.Reactions)
	assert.Equal(t, "-1", ghc.Reactions[0].Content)
}

func TestKind_NoArg(t *testing.T) {
	sc, ghc := prContext("author")
	reg := commands.Registry{"kind": {Factory: commands.NewKindHandler}}
	commands.Dispatch(context.Background(), sc, "/kind", reg, ghc, kindOpts())

	require.NotEmpty(t, ghc.Reactions)
	assert.Equal(t, "-1", ghc.Reactions[0].Content, "expected -1 reaction for /kind with no arg")
}

func TestKind_NotOnPR(t *testing.T) {
	sc, ghc := prContext("author")
	sc.PR = nil

	reg := commands.Registry{"kind": {Factory: commands.NewKindHandler}}
	commands.Dispatch(context.Background(), sc, "/kind bug", reg, ghc, kindOpts())

	require.NotEmpty(t, ghc.Reactions)
	assert.Equal(t, "-1", ghc.Reactions[0].Content, "expected -1 for /kind on non-PR")
}

func TestKind_HandleError_SuppressesPost(t *testing.T) {
	sc, ghc := prContext("author")
	ghc.Errors["AddLabels"] = errors.New("boom")

	reg := commands.Registry{"kind": {Factory: commands.NewKindHandler}}
	commands.Dispatch(context.Background(), sc, "/kind bug", reg, ghc, kindOpts())

	assert.Empty(t, ghc.AutoMergeEnabled, "expected Post NOT to run when Handle errors")
	assert.Empty(t, ghc.AutoMergeDisabled, "expected Post NOT to run when Handle errors")
	require.NotEmpty(t, ghc.Reactions)
	assert.Equal(t, "confused", ghc.Reactions[0].Content, "expected confused reaction on internal error")
}
