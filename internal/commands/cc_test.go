package commands_test

import (
	"context"
	"errors"
	"testing"

	"github.com/elevran/stern/internal/commands"
	"github.com/elevran/stern/internal/config"
	"github.com/elevran/stern/internal/event"
	"github.com/elevran/stern/internal/github"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func ccReg() commands.Registry {
	return commands.Registry{
		"cc":   {Factory: commands.NewCCHandler("cc")},
		"uncc": {Factory: commands.NewCCHandler("uncc")},
	}
}

func TestCC_NoArgs_UsageComment(t *testing.T) {
	sc, ghc := prContext("author")

	commands.Dispatch(context.Background(), sc, "/cc", ccReg(), ghc, &config.Options{})

	assert.Empty(t, ghc.ReviewersRequested, "expected RequestReviewers NOT called with no args")
	require.NotEmpty(t, ghc.Reactions)
	assert.Equal(t, "-1", ghc.Reactions[0].Content)
	require.NotEmpty(t, ghc.Comments)
	assert.Equal(t, "usage: /cc @user [@user ...]", ghc.Comments[0].Body)
}

func TestCC_HappyPath(t *testing.T) {
	sc, ghc := prContext("author")

	commands.Dispatch(context.Background(), sc, "/cc @carol", ccReg(), ghc, &config.Options{})

	require.Len(t, ghc.ReviewersRequested, 1)
	assert.Equal(t, []string{"carol"}, ghc.ReviewersRequested[0].Users)
	require.NotEmpty(t, ghc.Reactions)
	assert.Equal(t, "+1", ghc.Reactions[0].Content)
}

func TestCC_MultiUser(t *testing.T) {
	sc, ghc := prContext("author")

	commands.Dispatch(context.Background(), sc, "/cc @carol @dan", ccReg(), ghc, &config.Options{})

	assert.Equal(t, []string{"carol", "dan"}, ghc.ReviewersRequested[0].Users)
}

func TestCC_StripsAtAndDeduplicates(t *testing.T) {
	sc, ghc := prContext("author")

	commands.Dispatch(context.Background(), sc, "/cc @carol @CAROL @dan", ccReg(), ghc, &config.Options{})

	assert.Equal(t, []string{"carol", "dan"}, ghc.ReviewersRequested[0].Users)
}

func TestUncc_HappyPath(t *testing.T) {
	sc, ghc := prContext("author")
	ghc.ReviewRequests[1] = []string{"carol"}

	commands.Dispatch(context.Background(), sc, "/uncc @carol", ccReg(), ghc, &config.Options{})

	require.Len(t, ghc.ReviewersRemoved, 1)
	assert.Equal(t, []string{"carol"}, ghc.ReviewersRemoved[0].Users)
}

func TestCC_NotOnPR(t *testing.T) {
	sc := &event.Context{
		Org:         "o",
		Repo:        "r",
		Author:      "alice",
		IssueNumber: 5,
		PR:          nil,
	}
	ghc := github.NewMockClient()

	commands.Dispatch(context.Background(), sc, "/cc @carol", ccReg(), ghc, &config.Options{})

	assert.Empty(t, ghc.ReviewersRequested, "expected RequestReviewers NOT called when not on a PR")
	require.NotEmpty(t, ghc.Reactions)
	assert.Equal(t, "-1", ghc.Reactions[0].Content)
}

func TestCC_HandleError_SuppressesPost(t *testing.T) {
	sc, ghc := prContext("author")
	ghc.Errors["RequestReviewers"] = errors.New("boom")

	commands.Dispatch(context.Background(), sc, "/cc @carol", ccReg(), ghc, &config.Options{})

	assert.Empty(t, ghc.AutoMergeEnabled, "expected Post NOT to run when Handle errors")
	assert.Empty(t, ghc.AutoMergeDisabled, "expected Post NOT to run when Handle errors")
	require.NotEmpty(t, ghc.Reactions)
	assert.Equal(t, "confused", ghc.Reactions[0].Content, "expected confused reaction on internal error")
}
