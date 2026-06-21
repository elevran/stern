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

func areaOpts() *config.Options {
	return &config.Options{
		Area: config.AreaOptions{
			Values: []string{"api", "cli", "ui"},
		},
	}
}

func TestArea_AddsLabel(t *testing.T) {
	sc, ghc := prContext("author")
	reg := commands.Registry{"area": {Factory: commands.NewAreaHandler}}
	commands.Dispatch(context.Background(), sc, "/area api", reg, ghc, areaOpts())

	assert.True(t, ghc.IssueLabels[1]["area/api"], "expected area/api label to be added")
	require.NotEmpty(t, ghc.Reactions)
	assert.Equal(t, "+1", ghc.Reactions[0].Content, "expected +1 reaction after successful /area")
}

func TestArea_AllowsMultipleValues(t *testing.T) {
	sc, ghc := prContext("author")
	sc.PR.Labels = []string{"area/api"}
	ghc.IssueLabels[1] = map[string]bool{"area/api": true}

	reg := commands.Registry{"area": {Factory: commands.NewAreaHandler}}
	commands.Dispatch(context.Background(), sc, "/area cli", reg, ghc, areaOpts())

	assert.True(t, ghc.IssueLabels[1]["area/api"], "expected pre-existing area/api label to remain (no mutual exclusion)")
	assert.True(t, ghc.IssueLabels[1]["area/cli"], "expected new area/cli label to be added")
}

func TestArea_InvalidValue(t *testing.T) {
	sc, ghc := prContext("author")
	reg := commands.Registry{"area": {Factory: commands.NewAreaHandler}}
	commands.Dispatch(context.Background(), sc, "/area unknown", reg, ghc, areaOpts())

	assert.Empty(t, ghc.IssueLabels[1], "expected no labels added for invalid area")
	require.NotEmpty(t, ghc.Reactions)
	assert.Equal(t, "-1", ghc.Reactions[0].Content, "expected -1 reaction for invalid area")
}

func TestArea_NoArg(t *testing.T) {
	sc, ghc := prContext("author")
	reg := commands.Registry{"area": {Factory: commands.NewAreaHandler}}
	commands.Dispatch(context.Background(), sc, "/area", reg, ghc, areaOpts())

	require.NotEmpty(t, ghc.Reactions)
	assert.Equal(t, "-1", ghc.Reactions[0].Content, "expected -1 reaction for /area with no arg")
}

func TestArea_NotOnPR(t *testing.T) {
	sc, ghc := prContext("author")
	sc.PR = nil

	reg := commands.Registry{"area": {Factory: commands.NewAreaHandler}}
	commands.Dispatch(context.Background(), sc, "/area api", reg, ghc, areaOpts())

	require.NotEmpty(t, ghc.Reactions)
	assert.Equal(t, "-1", ghc.Reactions[0].Content, "expected -1 for /area on non-PR")
}

func TestArea_HandleError_SuppressesPost(t *testing.T) {
	sc, ghc := prContext("author")
	ghc.Errors["AddLabels"] = errors.New("boom")

	reg := commands.Registry{"area": {Factory: commands.NewAreaHandler}}
	commands.Dispatch(context.Background(), sc, "/area api", reg, ghc, areaOpts())

	assert.Empty(t, ghc.AutoMergeEnabled, "expected Post NOT to run when Handle errors")
	assert.Empty(t, ghc.AutoMergeDisabled, "expected Post NOT to run when Handle errors")
	require.NotEmpty(t, ghc.Reactions)
	assert.Equal(t, "confused", ghc.Reactions[0].Content, "expected confused reaction on internal error")
}
