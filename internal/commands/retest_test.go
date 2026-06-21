package commands_test

import (
	"context"
	"testing"

	"github.com/elevran/stern/internal/commands"
	"github.com/elevran/stern/internal/config"
	"github.com/elevran/stern/internal/github"
)

func retestOpts() *config.Options {
	return &config.Options{}
}

func TestRetest_NoFailedChecks_PostsComment(t *testing.T) {
	sc, ghc := prContext("author")
	sc.Author = "reviewer"
	ghc.WriteAccess["elevran/stern/reviewer"] = true
	// no FailedCheckRuns entry → empty list

	reg := commands.Registry{"retest": commands.NewRetestHandler}
	commands.Dispatch(context.Background(), sc, "/retest", reg, ghc, retestOpts())

	if len(ghc.RerunCheckRuns) != 0 {
		t.Errorf("expected no RerunCheckRun calls, got %v", ghc.RerunCheckRuns)
	}
	found := false
	for _, c := range ghc.Comments {
		if c.Number == 1 && c.Body == "No failed checks to re-run." {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 'no failed checks' comment, got %v", ghc.Comments)
	}
	if len(ghc.Reactions) == 0 || ghc.Reactions[0].Content != "+1" {
		t.Errorf("expected +1 reaction after successful /retest, got %v", ghc.Reactions)
	}
}

func TestRetest_SingleFailedCheck_RerunsIt(t *testing.T) {
	sc, ghc := prContext("author")
	sc.Author = "reviewer"
	ghc.WriteAccess["elevran/stern/reviewer"] = true
	ghc.FailedCheckRuns["elevran/stern/abc123"] = []github.CheckRun{
		{ID: 42, Name: "ci/test", Conclusion: "failure"},
	}

	reg := commands.Registry{"retest": commands.NewRetestHandler}
	commands.Dispatch(context.Background(), sc, "/retest", reg, ghc, retestOpts())

	if len(ghc.RerunCheckRuns) != 1 || ghc.RerunCheckRuns[0] != 42 {
		t.Errorf("expected one RerunCheckRun(42), got %v", ghc.RerunCheckRuns)
	}
	if len(ghc.Reactions) == 0 || ghc.Reactions[0].Content != "+1" {
		t.Errorf("expected +1 reaction after successful /retest, got %v", ghc.Reactions)
	}
}

func TestRetest_MultipleFailedChecks_RerunsAll(t *testing.T) {
	sc, ghc := prContext("author")
	sc.Author = "reviewer"
	ghc.WriteAccess["elevran/stern/reviewer"] = true
	ghc.FailedCheckRuns["elevran/stern/abc123"] = []github.CheckRun{
		{ID: 1, Name: "ci/test1", Conclusion: "failure"},
		{ID: 2, Name: "ci/test2", Conclusion: "timed_out"},
		{ID: 3, Name: "ci/test3", Conclusion: "cancelled"},
		{ID: 4, Name: "ci/test4", Conclusion: "action_required"},
	}

	reg := commands.Registry{"retest": commands.NewRetestHandler}
	commands.Dispatch(context.Background(), sc, "/retest", reg, ghc, retestOpts())

	if len(ghc.RerunCheckRuns) != 4 {
		t.Fatalf("expected 4 RerunCheckRun calls, got %d (%v)", len(ghc.RerunCheckRuns), ghc.RerunCheckRuns)
	}
	want := map[int64]bool{1: true, 2: true, 3: true, 4: true}
	for _, id := range ghc.RerunCheckRuns {
		if !want[id] {
			t.Errorf("unexpected RerunCheckRun id %d", id)
		}
		delete(want, id)
	}
	if len(want) > 0 {
		t.Errorf("missing RerunCheckRun calls for ids: %v", want)
	}
}

func TestRetest_NonWriter_Denied(t *testing.T) {
	sc, ghc := prContext("author")
	sc.Author = "reader"
	ghc.WriteAccess["elevran/stern/reader"] = false
	ghc.FailedCheckRuns["elevran/stern/abc123"] = []github.CheckRun{
		{ID: 42, Name: "ci/test", Conclusion: "failure"},
	}

	reg := commands.Registry{"retest": commands.NewRetestHandler}
	commands.Dispatch(context.Background(), sc, "/retest", reg, ghc, retestOpts())

	if len(ghc.RerunCheckRuns) != 0 {
		t.Errorf("expected no RerunCheckRun calls for non-writer, got %v", ghc.RerunCheckRuns)
	}
	if len(ghc.Reactions) == 0 || ghc.Reactions[0].Content != "-1" {
		t.Errorf("expected -1 reaction for non-writer, got %v", ghc.Reactions)
	}
}

func TestRetest_NotOnPR_Denied(t *testing.T) {
	sc, ghc := prContext("author")
	sc.PR = nil

	reg := commands.Registry{"retest": commands.NewRetestHandler}
	commands.Dispatch(context.Background(), sc, "/retest", reg, ghc, retestOpts())

	if len(ghc.RerunCheckRuns) != 0 {
		t.Errorf("expected no RerunCheckRun calls on non-PR, got %v", ghc.RerunCheckRuns)
	}
	if len(ghc.Reactions) == 0 || ghc.Reactions[0].Content != "-1" {
		t.Errorf("expected -1 reaction for /retest on non-PR, got %v", ghc.Reactions)
	}
}
