package commands_test

import (
	"context"
	"errors"
	"testing"

	"github.com/elevran/stern/internal/commands"
	"github.com/elevran/stern/internal/config"
	"github.com/elevran/stern/internal/github"
)

func approveOpts(allowSelf bool) *config.Options {
	return &config.Options{
		Approve: config.ApproveOptions{
			AllowSelfApproval: allowSelf,
			InvalidateOnPush:  false,
			RequireOwner:      true,
		},
		Merge: config.MergeOptions{
			Method:         "squash",
			BlockingLabels: []string{"do-not-merge/hold"},
		},
	}
}

func TestApprove_AddsLabel(t *testing.T) {
	sc, ghc := prContext("author")
	sc.Author = "approver"
	ghc.FileContent["OWNERS@abc123"] = []byte("approvers:\n  - approver\n")
	ghc.PRFiles[1] = []string{"main.go"}

	reg := commands.Registry{"approve": commands.NewApproveHandler}
	commands.Dispatch(context.Background(), sc, "/approve", reg, ghc, approveOpts(false))

	if !ghc.IssueLabels[1]["approved"] {
		t.Error("expected approved label to be added")
	}
	if len(ghc.Reactions) == 0 || ghc.Reactions[0].Content != "+1" {
		t.Errorf("expected +1 reaction after successful /approve, got %v", ghc.Reactions)
	}
}

func TestApprove_Cancel_RemovesLabel(t *testing.T) {
	sc, ghc := prContext("author")
	sc.Author = "approver"
	ghc.IssueLabels[1] = map[string]bool{"approved": true}

	reg := commands.Registry{"approve": commands.NewApproveHandler}
	commands.Dispatch(context.Background(), sc, "/approve cancel", reg, ghc, approveOpts(false))

	if ghc.IssueLabels[1]["approved"] {
		t.Error("expected approved label removed on cancel")
	}
	if len(ghc.Reactions) == 0 || ghc.Reactions[0].Content != "+1" {
		t.Errorf("expected +1 reaction after successful /approve cancel, got %v", ghc.Reactions)
	}
}

func TestApprove_SelfApprovalDenied(t *testing.T) {
	sc, ghc := prContext("approver")
	sc.Author = "approver" // PR author == commenter

	reg := commands.Registry{"approve": commands.NewApproveHandler}
	commands.Dispatch(context.Background(), sc, "/approve", reg, ghc, approveOpts(false))

	if ghc.IssueLabels[1]["approved"] {
		t.Error("expected approved NOT added for self-approval")
	}
	if len(ghc.Reactions) == 0 || ghc.Reactions[0].Content != "-1" {
		t.Errorf("expected -1 reaction, got %v", ghc.Reactions)
	}
}

func TestApprove_NonApproverDenied(t *testing.T) {
	sc, ghc := prContext("author")
	sc.Author = "outsider"
	ghc.FileContent["OWNERS@abc123"] = []byte("approvers:\n  - alice\n")
	ghc.PRFiles[1] = []string{"main.go"}

	reg := commands.Registry{"approve": commands.NewApproveHandler}
	commands.Dispatch(context.Background(), sc, "/approve", reg, ghc, approveOpts(false))

	if ghc.IssueLabels[1]["approved"] {
		t.Error("expected approved NOT added for non-approver")
	}
	if len(ghc.Reactions) == 0 || ghc.Reactions[0].Content != "-1" {
		t.Errorf("expected -1 reaction, got %v", ghc.Reactions)
	}
}

func TestApprove_BothLGTMAndApproved_TriggersAutoMerge(t *testing.T) {
	sc, ghc := prContext("author")
	sc.Author = "approver"
	// Pre-load the PR with lgtm already present.
	ghc.PullRequests[1].Labels = []string{"lgtm"}
	// After approve, GetPullRequest will return the updated PR.
	ghc.IssueLabels[1] = map[string]bool{"lgtm": true}
	// No OWNERS files: any commenter can approve.

	reg := commands.Registry{"approve": commands.NewApproveHandler}
	opts := &config.Options{
		Approve: config.ApproveOptions{RequireOwner: false},
		Merge:   config.MergeOptions{Method: "squash", BlockingLabels: []string{"do-not-merge/hold"}},
	}
	commands.Dispatch(context.Background(), sc, "/approve", reg, ghc, opts)

	if !ghc.IssueLabels[1]["approved"] {
		t.Error("expected approved label added")
	}
	if len(ghc.Reactions) == 0 || ghc.Reactions[0].Content != "+1" {
		t.Errorf("expected +1 reaction after successful /approve, got %v", ghc.Reactions)
	}
}

func TestApprove_HandleError_SuppressesPost(t *testing.T) {
	sc, ghc := prContext("author")
	sc.Author = "approver"
	ghc.Errors["AddLabels"] = errors.New("boom")

	reg := commands.Registry{"approve": commands.NewApproveHandler}
	commands.Dispatch(context.Background(), sc, "/approve", reg, ghc, approveOpts(false))

	if len(ghc.AutoMergeEnabled) > 0 || len(ghc.AutoMergeDisabled) > 0 {
		t.Errorf("expected Post NOT to run when Handle errors, got enabled=%v disabled=%v",
			ghc.AutoMergeEnabled, ghc.AutoMergeDisabled)
	}
	if len(ghc.Reactions) == 0 || ghc.Reactions[0].Content != "confused" {
		t.Errorf("expected confused reaction on internal error, got %v", ghc.Reactions)
	}
}

func TestApprove_PostsReview(t *testing.T) {
	sc, ghc := prContext("author")
	sc.Author = "approver"
	ghc.FileContent["OWNERS@abc123"] = []byte("approvers:\n  - approver\n")
	ghc.PRFiles[1] = []string{"main.go"}
	opts := approveOpts(false)
	opts.BotLogin = "stern-bot"

	reg := commands.Registry{"approve": commands.NewApproveHandler}
	commands.Dispatch(context.Background(), sc, "/approve", reg, ghc, opts)

	if len(ghc.ReviewsPosted) != 1 {
		t.Fatalf("expected exactly one CreatePullRequestReview call, got %d", len(ghc.ReviewsPosted))
	}
	got := ghc.ReviewsPosted[0]
	if got.Number != 1 || got.Event != "APPROVE" {
		t.Errorf("unexpected review record: %+v", got)
	}
}

func TestApprove_Idempotent_WhenBotReviewExists(t *testing.T) {
	sc, ghc := prContext("author")
	sc.Author = "approver"
	ghc.FileContent["OWNERS@abc123"] = []byte("approvers:\n  - approver\n")
	ghc.PRFiles[1] = []string{"main.go"}
	opts := approveOpts(false)
	opts.BotLogin = "stern-bot"
	ghc.Reviews[1] = []github.Review{{ID: 42, State: "APPROVED", Login: "stern-bot"}}

	reg := commands.Registry{"approve": commands.NewApproveHandler}
	commands.Dispatch(context.Background(), sc, "/approve", reg, ghc, opts)

	if len(ghc.ReviewsPosted) != 0 {
		t.Errorf("expected NO CreatePullRequestReview call when bot review exists, got %v", ghc.ReviewsPosted)
	}
}

func TestApprove_Cancel_DismissesReview(t *testing.T) {
	sc, ghc := prContext("author")
	sc.Author = "approver"
	ghc.IssueLabels[1] = map[string]bool{"approved": true}
	ghc.Reviews[1] = []github.Review{{ID: 99, State: "APPROVED", Login: "stern-bot"}}
	opts := approveOpts(false)
	opts.BotLogin = "stern-bot"

	reg := commands.Registry{"approve": commands.NewApproveHandler}
	commands.Dispatch(context.Background(), sc, "/approve cancel", reg, ghc, opts)

	if len(ghc.ReviewsDismissed) != 1 || ghc.ReviewsDismissed[0] != 99 {
		t.Errorf("expected DismissPullRequestReview called with id 99, got %v", ghc.ReviewsDismissed)
	}
	if got := ghc.Reviews[1][0].State; got != "DISMISSED" {
		t.Errorf("expected review state DISMISSED, got %q", got)
	}
}

func TestApprove_Cancel_NoReviewIsNoOp(t *testing.T) {
	sc, ghc := prContext("author")
	sc.Author = "approver"
	ghc.IssueLabels[1] = map[string]bool{"approved": true}
	opts := approveOpts(false)
	opts.BotLogin = "stern-bot"

	reg := commands.Registry{"approve": commands.NewApproveHandler}
	commands.Dispatch(context.Background(), sc, "/approve cancel", reg, ghc, opts)

	if len(ghc.ReviewsDismissed) != 0 {
		t.Errorf("expected NO DismissPullRequestReview when bot has no review, got %v", ghc.ReviewsDismissed)
	}
}
