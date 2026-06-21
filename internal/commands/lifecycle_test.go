package commands_test

import (
	"context"
	"errors"
	"net/http"
	"testing"

	gh "github.com/google/go-github/v72/github"

	"github.com/elevran/stern/internal/commands"
	"github.com/elevran/stern/internal/config"
	"github.com/elevran/stern/internal/event"
	"github.com/elevran/stern/internal/github"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func lifecycleOpts(enabled bool) *config.Options {
	return &config.Options{
		Plugins: []string{"lifecycle"},
		Lifecycle: config.LifecycleOptions{
			Enabled:    enabled,
			StaleDays:  90,
			RottenDays: 30,
			CloseAfter: 30,
		},
	}
}

// lifecycleContext returns an issue-style context (no PR). /lifecycle works
// on both issues and PRs; tests do not need the PR fields.
func lifecycleContext() (*event.Context, *github.MockClient) {
	ghc := github.NewMockClient()
	return &event.Context{
		Org:         "elevran",
		Repo:        "stern",
		CommentID:   10,
		Author:      "maintainer",
		IssueNumber: 42,
	}, ghc
}

func TestLifecycle_Stale_NoExistingLabels(t *testing.T) {
	sc, ghc := lifecycleContext()
	reg := commands.Registry{"lifecycle": {Factory: commands.NewLifecycleHandler}}
	commands.Dispatch(context.Background(), sc, "/lifecycle stale", reg, ghc, lifecycleOpts(true))

	assert.True(t, ghc.IssueLabels[42]["lifecycle/stale"], "expected lifecycle/stale added")
	assert.False(t, ghc.IssueLabels[42]["lifecycle/rotten"])
	assert.False(t, ghc.IssueLabels[42]["lifecycle/frozen"])
	require.NotEmpty(t, ghc.Reactions)
	assert.Equal(t, "+1", ghc.Reactions[0].Content)
}

func TestLifecycle_Frozen_RemovesStale(t *testing.T) {
	sc, ghc := lifecycleContext()
	ghc.IssueLabels[42] = map[string]bool{"lifecycle/stale": true}

	reg := commands.Registry{"lifecycle": {Factory: commands.NewLifecycleHandler}}
	commands.Dispatch(context.Background(), sc, "/lifecycle frozen", reg, ghc, lifecycleOpts(true))

	assert.False(t, ghc.IssueLabels[42]["lifecycle/stale"], "expected lifecycle/stale removed")
	assert.True(t, ghc.IssueLabels[42]["lifecycle/frozen"], "expected lifecycle/frozen added")
	require.NotEmpty(t, ghc.Reactions)
	assert.Equal(t, "+1", ghc.Reactions[0].Content)
}

func TestLifecycle_Rotten_RemovesStaleAndFrozen(t *testing.T) {
	sc, ghc := lifecycleContext()
	ghc.IssueLabels[42] = map[string]bool{
		"lifecycle/stale":  true,
		"lifecycle/frozen": true,
	}

	reg := commands.Registry{"lifecycle": {Factory: commands.NewLifecycleHandler}}
	commands.Dispatch(context.Background(), sc, "/lifecycle rotten", reg, ghc, lifecycleOpts(true))

	assert.False(t, ghc.IssueLabels[42]["lifecycle/stale"])
	assert.False(t, ghc.IssueLabels[42]["lifecycle/frozen"])
	assert.True(t, ghc.IssueLabels[42]["lifecycle/rotten"])
}

func TestLifecycle_Active_RemovesAll(t *testing.T) {
	sc, ghc := lifecycleContext()
	ghc.IssueLabels[42] = map[string]bool{
		"lifecycle/stale":  true,
		"lifecycle/rotten": true,
		"lifecycle/frozen": true,
	}

	reg := commands.Registry{"lifecycle": {Factory: commands.NewLifecycleHandler}}
	commands.Dispatch(context.Background(), sc, "/lifecycle active", reg, ghc, lifecycleOpts(true))

	assert.False(t, ghc.IssueLabels[42]["lifecycle/stale"])
	assert.False(t, ghc.IssueLabels[42]["lifecycle/rotten"])
	assert.False(t, ghc.IssueLabels[42]["lifecycle/frozen"])
	require.NotEmpty(t, ghc.Reactions)
	assert.Equal(t, "+1", ghc.Reactions[0].Content)
}

func TestLifecycle_Active_NoLabels_NoError(t *testing.T) {
	// IsNotFoundError from RemoveLabel must be tolerated, so /lifecycle active
	// succeeds even when no lifecycle labels are present.
	sc, ghc := lifecycleContext()
	ghc.Errors["RemoveLabel"] = &gh.ErrorResponse{
		Response: &http.Response{StatusCode: http.StatusNotFound},
	}

	reg := commands.Registry{"lifecycle": {Factory: commands.NewLifecycleHandler}}
	commands.Dispatch(context.Background(), sc, "/lifecycle active", reg, ghc, lifecycleOpts(true))

	require.NotEmpty(t, ghc.Reactions)
	assert.Equal(t, "+1", ghc.Reactions[0].Content, "404 from RemoveLabel should be tolerated")
}

func TestLifecycle_NoArgs(t *testing.T) {
	sc, ghc := lifecycleContext()
	reg := commands.Registry{"lifecycle": {Factory: commands.NewLifecycleHandler}}
	commands.Dispatch(context.Background(), sc, "/lifecycle", reg, ghc, lifecycleOpts(true))

	require.NotEmpty(t, ghc.Reactions)
	assert.Equal(t, "-1", ghc.Reactions[0].Content)
	require.NotEmpty(t, ghc.Comments, "expected usage comment on missing subcommand")
	assert.Contains(t, ghc.Comments[0].Body, "usage:")
}

func TestLifecycle_UnknownSubcommand(t *testing.T) {
	sc, ghc := lifecycleContext()
	reg := commands.Registry{"lifecycle": {Factory: commands.NewLifecycleHandler}}
	commands.Dispatch(context.Background(), sc, "/lifecycle bogus", reg, ghc, lifecycleOpts(true))

	require.NotEmpty(t, ghc.Reactions)
	assert.Equal(t, "-1", ghc.Reactions[0].Content)
	require.NotEmpty(t, ghc.Comments)
	assert.Contains(t, ghc.Comments[0].Body, "unknown lifecycle subcommand")
	// No label mutations on rejected subcommand.
	assert.Empty(t, ghc.IssueLabels[42])
}

func TestLifecycle_NotEnabled(t *testing.T) {
	// Plugin is in opts.Plugins (Dispatch lets it through), but
	// Lifecycle.Enabled is false, so Pre returns a PermissionError.
	sc, ghc := lifecycleContext()
	reg := commands.Registry{"lifecycle": {Factory: commands.NewLifecycleHandler}}
	commands.Dispatch(context.Background(), sc, "/lifecycle stale", reg, ghc, lifecycleOpts(false))

	require.NotEmpty(t, ghc.Reactions)
	assert.Equal(t, "-1", ghc.Reactions[0].Content)
	require.NotEmpty(t, ghc.Comments)
	assert.Contains(t, ghc.Comments[0].Body, "lifecycle plugin is not enabled")
	assert.Empty(t, ghc.IssueLabels[42], "no labels touched when plugin disabled")
}

func TestLifecycle_UnknownSubcommand_PreFailsBeforeHandle(t *testing.T) {
	// Verify Handle is never reached when Pre rejects: AddLabels must not be called.
	sc, ghc := lifecycleContext()
	ghc.Errors["AddLabels"] = errors.New("Handle should not run")

	reg := commands.Registry{"lifecycle": {Factory: commands.NewLifecycleHandler}}
	commands.Dispatch(context.Background(), sc, "/lifecycle bogus", reg, ghc, lifecycleOpts(true))

	require.NotEmpty(t, ghc.Reactions)
	assert.Equal(t, "-1", ghc.Reactions[0].Content)
}

func TestLifecycle_RemoveLabel_NonNotFoundError_PropagatesAsInternal(t *testing.T) {
	// A non-404 error from RemoveLabel surfaces as a confused reaction.
	sc, ghc := lifecycleContext()
	ghc.Errors["RemoveLabel"] = errors.New("boom")

	reg := commands.Registry{"lifecycle": {Factory: commands.NewLifecycleHandler}}
	commands.Dispatch(context.Background(), sc, "/lifecycle stale", reg, ghc, lifecycleOpts(true))

	require.NotEmpty(t, ghc.Reactions)
	assert.Equal(t, "confused", ghc.Reactions[0].Content)
}
