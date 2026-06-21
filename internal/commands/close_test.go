package commands_test

import (
	"context"
	"testing"

	"github.com/elevran/stern/internal/commands"
	"github.com/elevran/stern/internal/config"
	"github.com/elevran/stern/internal/event"
	"github.com/elevran/stern/internal/github"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func closeOpts() *config.Options {
	return &config.Options{
		Merge: config.MergeOptions{
			Method:         "squash",
			BlockingLabels: []string{"do-not-merge/hold"},
		},
	}
}

func issueContext() (*event.Context, *github.MockClient) {
	ghc := github.NewMockClient()
	sc := &event.Context{
		Org:         "elevran",
		Repo:        "stern",
		CommentID:   10,
		Author:      "maintainer",
		IssueNumber: 7,
		PR:          nil, // plain issue
	}
	ghc.WriteAccess["elevran/stern/maintainer"] = true
	return sc, ghc
}

func TestClose_ClosesIssue(t *testing.T) {
	sc, ghc := issueContext()
	reg := commands.DefaultRegistry()
	commands.Dispatch(context.Background(), sc, "/close", reg, ghc, closeOpts())

	assert.Equal(t, []int{7}, ghc.IssueClosed, "expected CloseIssue(7)")
	assert.Empty(t, ghc.IssueReopened, "expected ReopenIssue NOT called")
	require.NotEmpty(t, ghc.Reactions)
	assert.Equal(t, "+1", ghc.Reactions[0].Content)
}

func TestReopen_ReopensIssue(t *testing.T) {
	sc, ghc := issueContext()
	reg := commands.DefaultRegistry()
	commands.Dispatch(context.Background(), sc, "/reopen", reg, ghc, closeOpts())

	assert.Equal(t, []int{7}, ghc.IssueReopened, "expected ReopenIssue(7)")
	assert.Empty(t, ghc.IssueClosed, "expected CloseIssue NOT called")
	require.NotEmpty(t, ghc.Reactions)
	assert.Equal(t, "+1", ghc.Reactions[0].Content)
}

func TestClose_PermissionDeniedForNonWriter(t *testing.T) {
	sc, ghc := issueContext()
	sc.Author = "reader"
	ghc.WriteAccess["elevran/stern/reader"] = false

	reg := commands.DefaultRegistry()
	commands.Dispatch(context.Background(), sc, "/close", reg, ghc, closeOpts())

	assert.Empty(t, ghc.IssueClosed, "expected CloseIssue NOT called for non-writer")
	require.NotEmpty(t, ghc.Reactions)
	assert.Equal(t, "-1", ghc.Reactions[0].Content)
}
