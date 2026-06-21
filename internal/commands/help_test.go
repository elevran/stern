package commands_test

import (
	"context"
	"strings"
	"testing"

	"github.com/elevran/stern/internal/commands"
	"github.com/elevran/stern/internal/config"
	"github.com/elevran/stern/internal/event"
	"github.com/elevran/stern/internal/github"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestHelp_ListsAllEnabledCommands verifies the rendered help body lists
// every command in DefaultRegistry except the always-hidden /ping and /help.
func TestHelp_ListsAllEnabledCommands(t *testing.T) {
	ghc := github.NewMockClient()
	sc := newSternContext()

	reg := commands.DefaultRegistry()
	commands.Dispatch(context.Background(), sc, "/help", reg, ghc, &config.Options{})

	require.Len(t, ghc.Comments, 1)
	body := ghc.Comments[0].Body

	// Spot-check a handful of verbs.
	for _, want := range []string{"/lgtm", "/approve", "/hold", "/close", "/assign", "/kind"} {
		assert.Contains(t, body, want, "expected /help body to mention %s", want)
	}
	// Self and ping are always hidden.
	assert.NotContains(t, body, "/ping", "expected /help to hide /ping")
	assert.NotContains(t, body, "- `/help`", "expected /help to hide itself")
}

// TestHelp_RespectsPluginFilter verifies that when an explicit plugin list is
// configured, disabled commands are omitted from the rendered help body.
func TestHelp_RespectsPluginFilter(t *testing.T) {
	ghc := github.NewMockClient()
	sc := newSternContext()

	reg := commands.DefaultRegistry()
	// Only enable lgtm and approve; everything else should be filtered out.
	opts := &config.Options{Plugins: []string{"lgtm", "approve"}}
	commands.Dispatch(context.Background(), sc, "/help", reg, ghc, opts)

	require.Len(t, ghc.Comments, 1)
	body := ghc.Comments[0].Body

	assert.Contains(t, body, "/lgtm")
	assert.Contains(t, body, "/approve")
	assert.NotContains(t, body, "/close", "expected /close filtered out by plugin list")
	assert.NotContains(t, body, "/retest", "expected /retest filtered out by plugin list")
}

// TestHelp_NoPRRequired verifies /help works on issues (sc.PR is nil).
func TestHelp_NoPRRequired(t *testing.T) {
	ghc := github.NewMockClient()
	sc := &event.Context{
		Org:         "elevran",
		Repo:        "stern",
		CommentID:   1,
		Author:      "testuser",
		IssueNumber: 7,
		// PR is nil — /help must still work on issues.
	}

	reg := commands.DefaultRegistry()
	commands.Dispatch(context.Background(), sc, "/help", reg, ghc, &config.Options{})

	require.Len(t, ghc.Comments, 1, "expected /help to post a comment")
	assert.True(t, strings.HasPrefix(ghc.Comments[0].Body, "## Available commands"),
		"expected help body to start with heading, got %q", ghc.Comments[0].Body)
}

// TestHelp_DeterministicOrdering verifies the verbs appear in sorted order.
func TestHelp_DeterministicOrdering(t *testing.T) {
	ghc := github.NewMockClient()
	sc := newSternContext()

	reg := commands.DefaultRegistry()
	commands.Dispatch(context.Background(), sc, "/help", reg, ghc, &config.Options{})

	require.Len(t, ghc.Comments, 1)
	body := ghc.Comments[0].Body

	// /approve should appear before /lgtm (alphabetical), since both are enabled.
	approveIdx := strings.Index(body, "/approve")
	lgtmIdx := strings.Index(body, "/lgtm")
	require.NotEqual(t, -1, approveIdx, "expected /approve in body")
	require.NotEqual(t, -1, lgtmIdx, "expected /lgtm in body")
	assert.Less(t, approveIdx, lgtmIdx, "expected /approve before /lgtm (sorted)")
}
