package github

import (
	"context"
	"io"
	"testing"

	"github.com/sirupsen/logrus"
)

// TestDryRunClient_SuppressesMutations verifies that every mutating method
// on the wrapped Client is logged but not invoked. Each call should return
// nil and leave the inner MockClient's call-tracking slices/fields empty.
func TestDryRunClient_SuppressesMutations(t *testing.T) {
	inner := NewMockClient()
	inner.PullRequests[1] = &PullRequest{Number: 1, NodeID: "PR_node", HeadSHA: "abc"}

	logger := logrus.New()
	logger.SetOutput(io.Discard)
	dry := NewDryRun(inner, logger)

	ctx := context.Background()
	if err := dry.CreateLabel(ctx, "o", "r", Label{Name: "bug"}); err != nil {
		t.Errorf("CreateLabel: unexpected error %v", err)
	}
	if err := dry.UpdateLabel(ctx, "o", "r", "bug", Label{Name: "bug"}); err != nil {
		t.Errorf("UpdateLabel: unexpected error %v", err)
	}
	if err := dry.DeleteLabel(ctx, "o", "r", "bug"); err != nil {
		t.Errorf("DeleteLabel: unexpected error %v", err)
	}
	if err := dry.AddLabels(ctx, "o", "r", 1, []string{"foo"}); err != nil {
		t.Errorf("AddLabels: unexpected error %v", err)
	}
	if err := dry.RemoveLabel(ctx, "o", "r", 1, "foo"); err != nil {
		t.Errorf("RemoveLabel: unexpected error %v", err)
	}
	if err := dry.CreateCommentReaction(ctx, "o", "r", 99, "+1"); err != nil {
		t.Errorf("CreateCommentReaction: unexpected error %v", err)
	}
	if err := dry.CreateIssueComment(ctx, "o", "r", 1, "hi"); err != nil {
		t.Errorf("CreateIssueComment: unexpected error %v", err)
	}
	if err := dry.EnableAutoMerge(ctx, "PR_node", "squash"); err != nil {
		t.Errorf("EnableAutoMerge: unexpected error %v", err)
	}
	if err := dry.DisableAutoMerge(ctx, "PR_node"); err != nil {
		t.Errorf("DisableAutoMerge: unexpected error %v", err)
	}
	if err := dry.CloseIssue(ctx, "o", "r", 1); err != nil {
		t.Errorf("CloseIssue: unexpected error %v", err)
	}
	if err := dry.ReopenIssue(ctx, "o", "r", 1); err != nil {
		t.Errorf("ReopenIssue: unexpected error %v", err)
	}
	if err := dry.SetMilestone(ctx, "o", "r", 1, 2); err != nil {
		t.Errorf("SetMilestone: unexpected error %v", err)
	}
	if err := dry.ClearMilestone(ctx, "o", "r", 1); err != nil {
		t.Errorf("ClearMilestone: unexpected error %v", err)
	}
	if err := dry.AddAssignees(ctx, "o", "r", 1, []string{"alice"}); err != nil {
		t.Errorf("AddAssignees: unexpected error %v", err)
	}
	if err := dry.RemoveAssignees(ctx, "o", "r", 1, []string{"alice"}); err != nil {
		t.Errorf("RemoveAssignees: unexpected error %v", err)
	}
	if err := dry.RequestReviewers(ctx, "o", "r", 1, []string{"alice"}); err != nil {
		t.Errorf("RequestReviewers: unexpected error %v", err)
	}
	if err := dry.RemoveReviewers(ctx, "o", "r", 1, []string{"alice"}); err != nil {
		t.Errorf("RemoveReviewers: unexpected error %v", err)
	}
	if err := dry.RerunCheckRun(ctx, "o", "r", 42); err != nil {
		t.Errorf("RerunCheckRun: unexpected error %v", err)
	}

	// All mutating methods must NOT have reached the underlying mock.
	if _, ok := inner.RepoLabels["bug"]; ok {
		t.Error("CreateLabel reached inner mock")
	}
	if len(inner.IssueLabels) != 0 {
		t.Errorf("AddLabels reached inner mock: %v", inner.IssueLabels)
	}
	if len(inner.Reactions) != 0 {
		t.Errorf("CreateCommentReaction reached inner mock: %v", inner.Reactions)
	}
	if len(inner.Comments) != 0 {
		t.Errorf("CreateIssueComment reached inner mock: %v", inner.Comments)
	}
	if len(inner.AutoMergeEnabled) != 0 || len(inner.AutoMergeDisabled) != 0 {
		t.Errorf("Enable/DisableAutoMerge reached inner mock: enabled=%v disabled=%v",
			inner.AutoMergeEnabled, inner.AutoMergeDisabled)
	}
	if len(inner.IssueClosed) != 0 || len(inner.IssueReopened) != 0 {
		t.Errorf("CloseIssue/ReopenIssue reached inner mock: closed=%v reopened=%v",
			inner.IssueClosed, inner.IssueReopened)
	}
	if len(inner.MilestoneSet) != 0 || len(inner.MilestoneCleared) != 0 {
		t.Errorf("SetMilestone/ClearMilestone reached inner mock: set=%v cleared=%v",
			inner.MilestoneSet, inner.MilestoneCleared)
	}
	if len(inner.AssigneesAdded) != 0 || len(inner.AssigneesRemoved) != 0 {
		t.Errorf("AddAssignees/RemoveAssignees reached inner mock: added=%v removed=%v",
			inner.AssigneesAdded, inner.AssigneesRemoved)
	}
	if len(inner.ReviewersRequested) != 0 || len(inner.ReviewersRemoved) != 0 {
		t.Errorf("RequestReviewers/RemoveReviewers reached inner mock: req=%v rem=%v",
			inner.ReviewersRequested, inner.ReviewersRemoved)
	}
	if len(inner.RerunCheckRuns) != 0 {
		t.Errorf("RerunCheckRun reached inner mock: %v", inner.RerunCheckRuns)
	}
}

// TestDryRunClient_PassesThroughReads verifies that read methods continue
// to delegate to the inner Client and surface real errors / data.
func TestDryRunClient_PassesThroughReads(t *testing.T) {
	inner := NewMockClient()
	inner.RepoLabels["bug"] = Label{Name: "bug", Color: "f00"}
	inner.PullRequests[7] = &PullRequest{Number: 7, Title: "t"}

	logger := logrus.New()
	logger.SetOutput(io.Discard)
	dry := NewDryRun(inner, logger)

	labels, err := dry.ListRepoLabels(context.Background(), "o", "r")
	if err != nil {
		t.Fatalf("ListRepoLabels: %v", err)
	}
	if len(labels) != 1 || labels[0].Name != "bug" {
		t.Errorf("ListRepoLabels returned %v, want [bug]", labels)
	}

	pr, err := dry.GetPullRequest(context.Background(), "o", "r", 7)
	if err != nil {
		t.Fatalf("GetPullRequest: %v", err)
	}
	if pr.Title != "t" {
		t.Errorf("GetPullRequest title = %q, want %q", pr.Title, "t")
	}
}