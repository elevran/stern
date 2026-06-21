package commands_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/elevran/stern/internal/commands"
	"github.com/elevran/stern/internal/config"
	"github.com/elevran/stern/internal/event"
	"github.com/elevran/stern/internal/github"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// withGitExec replaces the package-level gitExec stub for the duration of t.
// Returns a restore func.
func withGitExec(t *testing.T, stub func(args ...string) error) {
	t.Helper()
	orig := commands.SetGitExecForTest(stub)
	t.Cleanup(func() { commands.SetGitExecForTest(orig) })
}

// cherryPickReg returns a registry with the cherry-pick handler wired up.
func cherryPickReg() commands.Registry {
	return commands.Registry{"cherry-pick": {Factory: commands.NewCherryPickHandler}}
}

func cherryPickOpts(pattern string) *config.Options {
	return &config.Options{
		CherryPick: config.CherryPickOptions{
			AllowedBranchPattern: pattern,
		},
	}
}

// mergedPRContext returns an event.Context where sc.PR is a merged PR with
// a known MergeCommitSHA, mimicking the state after a PR has been merged.
func mergedPRContext(author string, mergedSHA string) *event.Context {
	p := &github.PullRequest{
		Number:         1,
		Author:         author,
		Title:          "Some merged PR",
		HeadSHA:        "head-sha",
		Merged:         true,
		MergeCommitSHA: mergedSHA,
	}
	return &event.Context{
		Org:         "elevran",
		Repo:        "stern",
		CommentID:   42,
		Author:      author,
		IssueNumber: 1,
		PR:          p,
	}
}

func TestCherryPick_Pre_RejectsNotMerged(t *testing.T) {
	sc, ghc := prContext("author")
	sc.Author = "alice"
	ghc.WriteAccess["elevran/stern/alice"] = true

	commands.Dispatch(context.Background(), sc, "/cherry-pick release-1.0", cherryPickReg(),
		ghc, cherryPickOpts("release-.*"))

	require.NotEmpty(t, ghc.Reactions)
	assert.Equal(t, "-1", ghc.Reactions[0].Content, "expected -1 reaction for non-merged PR")
	require.NotEmpty(t, ghc.Comments)
	assert.Contains(t, ghc.Comments[0].Body, "merged", "expected error to mention 'merged'")
}

func TestCherryPick_Pre_RejectsBranchNotMatchingPattern(t *testing.T) {
	ghc := github.NewMockClient()
	sc := mergedPRContext("alice", "merge-sha")
	ghc.WriteAccess["elevran/stern/alice"] = true

	commands.Dispatch(context.Background(), sc, "/cherry-pick main", cherryPickReg(),
		ghc, cherryPickOpts("release-.*"))

	require.NotEmpty(t, ghc.Reactions)
	assert.Equal(t, "-1", ghc.Reactions[0].Content)
	require.NotEmpty(t, ghc.Comments)
	assert.Contains(t, ghc.Comments[0].Body, "does not match", "expected pattern mismatch error")
}

func TestCherryPick_Pre_RejectsNonWriter(t *testing.T) {
	ghc := github.NewMockClient()
	sc := mergedPRContext("alice", "merge-sha")
	// alice has no write access in mock.

	commands.Dispatch(context.Background(), sc, "/cherry-pick release-1.0", cherryPickReg(),
		ghc, cherryPickOpts("release-.*"))

	require.NotEmpty(t, ghc.Reactions)
	assert.Equal(t, "-1", ghc.Reactions[0].Content, "expected -1 reaction for non-writer")
	require.NotEmpty(t, ghc.Comments)
	assert.Contains(t, ghc.Comments[0].Body, "write access", "expected 'write access' in error")
}

func TestCherryPick_Handle_HappyPath(t *testing.T) {
	ghc := github.NewMockClient()
	sc := mergedPRContext("alice", "merge-sha-123")
	ghc.WriteAccess["elevran/stern/alice"] = true

	// All git steps succeed.
	withGitExec(t, func(args ...string) error { return nil })

	commands.Dispatch(context.Background(), sc, "/cherry-pick release-1.0", cherryPickReg(),
		ghc, cherryPickOpts("release-.*"))

	// Expected git calls in order.
	require.NotEmpty(t, ghc.CreatedPRMeta, "expected CreatePullRequest to be called")
	pr := ghc.CreatedPRMeta[0]
	assert.Equal(t, "cherry-pick-1-release-1.0", pr.Head)
	assert.Equal(t, "release-1.0", pr.Base)
	assert.Contains(t, pr.Title, "[cherry-pick release-1.0]")
	assert.Contains(t, pr.Body, "Cherry-pick of #1")
	assert.Contains(t, pr.Body, "@alice")

	// Final comment mentions the new PR number.
	require.NotEmpty(t, ghc.Comments)
	last := ghc.Comments[len(ghc.Comments)-1]
	assert.True(t, strings.HasPrefix(last.Body, "Cherry-pick PR opened: #"),
		"expected final comment to announce the new PR, got %q", last.Body)

	require.NotEmpty(t, ghc.Reactions)
	assert.Equal(t, "+1", ghc.Reactions[0].Content, "expected +1 reaction on success")
}

func TestCherryPick_Handle_Conflict(t *testing.T) {
	ghc := github.NewMockClient()
	sc := mergedPRContext("alice", "merge-sha-123")
	ghc.WriteAccess["elevran/stern/alice"] = true

	// Cherry-pick fails: 3rd call (the cherry-pick itself) returns an error.
	// Calls: fetch, checkout, cherry-pick (fails), cherry-pick --abort (cleanup).
	withGitExec(t, func(args ...string) error {
		if len(args) >= 1 && args[0] == "cherry-pick" {
			return errors.New("conflict")
		}
		return nil
	})

	commands.Dispatch(context.Background(), sc, "/cherry-pick release-1.0", cherryPickReg(),
		ghc, cherryPickOpts("release-.*"))

	// Conflict must NOT create a PR.
	assert.Empty(t, ghc.CreatedPRMeta, "expected CreatePullRequest NOT called on conflict")

	// A conflict comment must be posted.
	require.NotEmpty(t, ghc.Comments)
	conflictComment := ghc.Comments[len(ghc.Comments)-1]
	assert.Contains(t, conflictComment.Body, "failed due to conflicts")
	assert.Contains(t, conflictComment.Body, "release-1.0")
	assert.Contains(t, conflictComment.Body, "git cherry-pick", "expected manual-recovery snippet")

	// +1 reaction still fires (conflict is not an internal error).
	require.NotEmpty(t, ghc.Reactions)
	assert.Equal(t, "+1", ghc.Reactions[0].Content, "expected +1 reaction even on conflict")
}
