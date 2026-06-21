package commands_test

import (
	"context"
	"testing"

	"github.com/elevran/stern/internal/commands"
	"github.com/elevran/stern/internal/config"
	"github.com/elevran/stern/internal/github"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func retestOpts() *config.Options {
	return &config.Options{}
}

func TestRetest_NoFailedChecks_PostsComment(t *testing.T) {
	sc, ghc := prContext("author")
	sc.Author = "reviewer"
	ghc.WriteAccess["elevran/stern/reviewer"] = true
	// no CheckRuns entry → empty list

	reg := commands.Registry{"retest": commands.NewRetestHandler}
	commands.Dispatch(context.Background(), sc, "/retest", reg, ghc, retestOpts())

	assert.Empty(t, ghc.RerunCheckRuns, "expected no RerunCheckRun calls")
	found := false
	for _, c := range ghc.Comments {
		if c.Number == 1 && c.Body == "No failed checks to re-run." {
			found = true
			break
		}
	}
	assert.True(t, found, "expected 'no failed checks' comment, got %v", ghc.Comments)
	require.NotEmpty(t, ghc.Reactions)
	assert.Equal(t, "+1", ghc.Reactions[0].Content)
}

func TestRetest_SingleFailedCheck_RerunsIt(t *testing.T) {
	sc, ghc := prContext("author")
	sc.Author = "reviewer"
	ghc.WriteAccess["elevran/stern/reviewer"] = true
	ghc.CheckRuns["elevran/stern/abc123"] = []github.CheckRun{
		{ID: 42, Name: "ci/test", Conclusion: "failure"},
	}

	reg := commands.Registry{"retest": commands.NewRetestHandler}
	commands.Dispatch(context.Background(), sc, "/retest", reg, ghc, retestOpts())

	require.Len(t, ghc.RerunCheckRuns, 1)
	assert.Equal(t, int64(42), ghc.RerunCheckRuns[0])
	require.NotEmpty(t, ghc.Reactions)
	assert.Equal(t, "+1", ghc.Reactions[0].Content)
}

func TestRetest_MultipleFailedChecks_RerunsAll(t *testing.T) {
	sc, ghc := prContext("author")
	sc.Author = "reviewer"
	ghc.WriteAccess["elevran/stern/reviewer"] = true
	ghc.CheckRuns["elevran/stern/abc123"] = []github.CheckRun{
		{ID: 1, Name: "ci/test1", Conclusion: "failure"},
		{ID: 2, Name: "ci/test2", Conclusion: "timed_out"},
		{ID: 3, Name: "ci/test3", Conclusion: "cancelled"},
		{ID: 4, Name: "ci/test4", Conclusion: "action_required"},
	}

	reg := commands.Registry{"retest": commands.NewRetestHandler}
	commands.Dispatch(context.Background(), sc, "/retest", reg, ghc, retestOpts())

	require.Len(t, ghc.RerunCheckRuns, 4)
	want := map[int64]bool{1: true, 2: true, 3: true, 4: true}
	for _, id := range ghc.RerunCheckRuns {
		require.True(t, want[id], "unexpected RerunCheckRun id %d", id)
		delete(want, id)
	}
	assert.Empty(t, want, "missing RerunCheckRun calls for ids: %v", want)
}

func TestRetest_NonWriter_Denied(t *testing.T) {
	sc, ghc := prContext("author")
	sc.Author = "reader"
	ghc.WriteAccess["elevran/stern/reader"] = false
	ghc.CheckRuns["elevran/stern/abc123"] = []github.CheckRun{
		{ID: 42, Name: "ci/test", Conclusion: "failure"},
	}

	reg := commands.Registry{"retest": commands.NewRetestHandler}
	commands.Dispatch(context.Background(), sc, "/retest", reg, ghc, retestOpts())

	assert.Empty(t, ghc.RerunCheckRuns, "expected no RerunCheckRun calls for non-writer")
	require.NotEmpty(t, ghc.Reactions)
	assert.Equal(t, "-1", ghc.Reactions[0].Content)
}

func TestRetest_NotOnPR_Denied(t *testing.T) {
	sc, ghc := prContext("author")
	sc.PR = nil

	reg := commands.Registry{"retest": commands.NewRetestHandler}
	commands.Dispatch(context.Background(), sc, "/retest", reg, ghc, retestOpts())

	assert.Empty(t, ghc.RerunCheckRuns, "expected no RerunCheckRun calls on non-PR")
	require.NotEmpty(t, ghc.Reactions)
	assert.Equal(t, "-1", ghc.Reactions[0].Content, "expected -1 reaction for /retest on non-PR")
}