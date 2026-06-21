package commands_test

import (
	"context"
	"errors"
	"testing"

	"github.com/elevran/stern/internal/commands"
	"github.com/elevran/stern/internal/config"
	"github.com/elevran/stern/internal/event"
	"github.com/elevran/stern/internal/github"
)

func ccReg() commands.Registry {
	return commands.Registry{
		"cc":   commands.NewCCHandler("cc"),
		"uncc": commands.NewCCHandler("uncc"),
	}
}

func TestCC_NoArgs_UsageComment(t *testing.T) {
	sc, ghc := prContext("author")

	commands.Dispatch(context.Background(), sc, "/cc", ccReg(), ghc, &config.Options{})

	if len(ghc.ReviewersRequested) != 0 {
		t.Errorf("expected RequestReviewers NOT called with no args, got %d", len(ghc.ReviewersRequested))
	}
	if len(ghc.Reactions) == 0 || ghc.Reactions[0].Content != "-1" {
		t.Errorf("expected -1 reaction, got %v", ghc.Reactions)
	}
	if len(ghc.Comments) == 0 {
		t.Fatalf("expected usage comment, got none")
	}
	if got := ghc.Comments[0].Body; got != "usage: /cc @user [@user ...]" {
		t.Errorf("unexpected usage comment: %q", got)
	}
}

func TestCC_HappyPath(t *testing.T) {
	sc, ghc := prContext("author")

	commands.Dispatch(context.Background(), sc, "/cc @carol", ccReg(), ghc, &config.Options{})

	if len(ghc.ReviewersRequested) != 1 {
		t.Fatalf("expected 1 RequestReviewers call, got %d", len(ghc.ReviewersRequested))
	}
	if got := ghc.ReviewersRequested[0].Users; len(got) != 1 || got[0] != "carol" {
		t.Errorf("expected RequestReviewers([carol]), got %v", got)
	}
	if len(ghc.Reactions) == 0 || ghc.Reactions[0].Content != "+1" {
		t.Errorf("expected +1 reaction, got %v", ghc.Reactions)
	}
}

func TestCC_MultiUser(t *testing.T) {
	sc, ghc := prContext("author")

	commands.Dispatch(context.Background(), sc, "/cc @carol @dan", ccReg(), ghc, &config.Options{})

	got := ghc.ReviewersRequested[0].Users
	if len(got) != 2 || got[0] != "carol" || got[1] != "dan" {
		t.Errorf("expected [carol dan], got %v", got)
	}
}

func TestCC_StripsAtAndDeduplicates(t *testing.T) {
	sc, ghc := prContext("author")

	commands.Dispatch(context.Background(), sc, "/cc @carol @CAROL @dan", ccReg(), ghc, &config.Options{})

	got := ghc.ReviewersRequested[0].Users
	if len(got) != 2 || got[0] != "carol" || got[1] != "dan" {
		t.Errorf("expected [carol dan] after dedupe+lowercase, got %v", got)
	}
}

func TestUncc_HappyPath(t *testing.T) {
	sc, ghc := prContext("author")
	ghc.ReviewRequests[1] = []string{"carol"}

	commands.Dispatch(context.Background(), sc, "/uncc @carol", ccReg(), ghc, &config.Options{})

	if len(ghc.ReviewersRemoved) != 1 {
		t.Fatalf("expected 1 RemoveReviewers call, got %d", len(ghc.ReviewersRemoved))
	}
	if got := ghc.ReviewersRemoved[0].Users; len(got) != 1 || got[0] != "carol" {
		t.Errorf("expected RemoveReviewers([carol]), got %v", got)
	}
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

	if len(ghc.ReviewersRequested) != 0 {
		t.Errorf("expected RequestReviewers NOT called when not on a PR")
	}
	if len(ghc.Reactions) == 0 || ghc.Reactions[0].Content != "-1" {
		t.Errorf("expected -1 reaction, got %v", ghc.Reactions)
	}
}

func TestCC_HandleError_SuppressesPost(t *testing.T) {
	sc, ghc := prContext("author")
	ghc.Errors["RequestReviewers"] = errors.New("boom")

	commands.Dispatch(context.Background(), sc, "/cc @carol", ccReg(), ghc, &config.Options{})

	if len(ghc.AutoMergeEnabled) > 0 || len(ghc.AutoMergeDisabled) > 0 {
		t.Errorf("expected Post NOT to run when Handle errors, got enabled=%v disabled=%v",
			ghc.AutoMergeEnabled, ghc.AutoMergeDisabled)
	}
	if len(ghc.Reactions) == 0 || ghc.Reactions[0].Content != "confused" {
		t.Errorf("expected confused reaction on internal error, got %v", ghc.Reactions)
	}
}
