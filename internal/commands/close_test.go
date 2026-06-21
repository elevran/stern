package commands_test

import (
	"context"
	"testing"

	"github.com/elevran/stern/internal/commands"
	"github.com/elevran/stern/internal/config"
	"github.com/elevran/stern/internal/event"
	"github.com/elevran/stern/internal/github"
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

	if len(ghc.IssueClosed) != 1 || ghc.IssueClosed[0] != 7 {
		t.Errorf("expected CloseIssue(7), got %v", ghc.IssueClosed)
	}
	if len(ghc.IssueReopened) != 0 {
		t.Errorf("expected ReopenIssue NOT called, got %v", ghc.IssueReopened)
	}
	if len(ghc.Reactions) == 0 || ghc.Reactions[0].Content != "+1" {
		t.Errorf("expected +1 reaction after successful /close, got %v", ghc.Reactions)
	}
}

func TestReopen_ReopensIssue(t *testing.T) {
	sc, ghc := issueContext()
	reg := commands.DefaultRegistry()
	commands.Dispatch(context.Background(), sc, "/reopen", reg, ghc, closeOpts())

	if len(ghc.IssueReopened) != 1 || ghc.IssueReopened[0] != 7 {
		t.Errorf("expected ReopenIssue(7), got %v", ghc.IssueReopened)
	}
	if len(ghc.IssueClosed) != 0 {
		t.Errorf("expected CloseIssue NOT called, got %v", ghc.IssueClosed)
	}
	if len(ghc.Reactions) == 0 || ghc.Reactions[0].Content != "+1" {
		t.Errorf("expected +1 reaction after successful /reopen, got %v", ghc.Reactions)
	}
}

func TestClose_PermissionDeniedForNonWriter(t *testing.T) {
	sc, ghc := issueContext()
	sc.Author = "reader"
	ghc.WriteAccess["elevran/stern/reader"] = false

	reg := commands.DefaultRegistry()
	commands.Dispatch(context.Background(), sc, "/close", reg, ghc, closeOpts())

	if len(ghc.IssueClosed) != 0 {
		t.Errorf("expected CloseIssue NOT called for non-writer, got %v", ghc.IssueClosed)
	}
	if len(ghc.Reactions) == 0 || ghc.Reactions[0].Content != "-1" {
		t.Errorf("expected -1 reaction for permission denied, got %v", ghc.Reactions)
	}
}
