package pr_test

import (
	"context"
	"errors"
	"net/http"
	"testing"

	gh "github.com/google/go-github/v72/github"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elevran/stern/internal/config"
	"github.com/elevran/stern/internal/github"
	"github.com/elevran/stern/internal/pr"
)

func wipOpts() *config.Options {
	return &config.Options{
		Merge: config.MergeOptions{
			Method:         "squash",
			BlockingLabels: []string{"do-not-merge/wip"},
		},
	}
}

func TestIsTitleWIP(t *testing.T) {
	cases := []struct {
		title string
		want  bool
	}{
		{"[WIP] fix something", true},
		{"wip: add feature", true},
		{"[Draft] new API", true},
		{"Draft: work in progress", true},
		{"Fix the bug", false},
		{"WIPpping something", false},
		{"", false},
	}
	for _, tc := range cases {
		got := pr.IsTitleWIP(tc.title)
		assert.Equal(t, tc.want, got, "IsTitleWIP(%q)", tc.title)
	}
}

func TestHandlePREventWIP_AddsOnWIPTitle(t *testing.T) {
	ghc := github.NewMockClient()
	p := github.PullRequest{
		Number: 1,
		Title:  "[WIP] test",
		Labels: []string{},
	}

	require.NoError(t, pr.HandlePREventWIP(context.Background(), ghc, "o", "r", p, wipOpts()))
	assert.True(t, ghc.IssueLabels[1]["do-not-merge/wip"], "expected wip label added for WIP title")
}

func TestHandlePREventWIP_RemovesWhenTitleClean(t *testing.T) {
	ghc := github.NewMockClient()
	ghc.IssueLabels[1] = map[string]bool{"do-not-merge/wip": true}
	p := github.PullRequest{
		Number: 1,
		Title:  "Fix the bug",
		Labels: []string{"do-not-merge/wip"},
	}
	ghc.PullRequests[1] = &p

	require.NoError(t, pr.HandlePREventWIP(context.Background(), ghc, "o", "r", p, wipOpts()))
	assert.False(t, ghc.IssueLabels[1]["do-not-merge/wip"], "expected wip label removed when title is clean")
}

func TestHandlePREventWIP_DraftAddsLabel(t *testing.T) {
	ghc := github.NewMockClient()
	p := github.PullRequest{
		Number:  1,
		Title:   "Normal PR",
		IsDraft: true,
		Labels:  []string{},
	}

	require.NoError(t, pr.HandlePREventWIP(context.Background(), ghc, "o", "r", p, wipOpts()))
	assert.True(t, ghc.IssueLabels[1]["do-not-merge/wip"], "expected wip label added for draft PR")
}

func TestInvalidateLGTMOnPush(t *testing.T) {
	ghc := github.NewMockClient()
	ghc.IssueLabels[1] = map[string]bool{"lgtm": true}
	p := github.PullRequest{
		Number: 1,
		Labels: []string{"lgtm"},
	}
	opts := &config.Options{
		LGTM: config.LGTMOptions{InvalidateOnPush: true},
	}

	require.NoError(t, pr.InvalidateLGTMOnPush(context.Background(), ghc, "o", "r", p, opts))
	assert.False(t, ghc.IssueLabels[1]["lgtm"], "expected lgtm label removed on push")
}

func TestInvalidateApproveOnPush(t *testing.T) {
	ghc := github.NewMockClient()
	ghc.IssueLabels[1] = map[string]bool{"approved": true}
	p := github.PullRequest{
		Number: 1,
		Labels: []string{"approved"},
	}
	opts := &config.Options{
		Approve: config.ApproveOptions{InvalidateOnPush: true},
	}

	require.NoError(t, pr.InvalidateApproveOnPush(context.Background(), ghc, "o", "r", p, opts))
	assert.False(t, ghc.IssueLabels[1]["approved"], "expected approved label removed on push")
}

func TestHandlePREvent_BotSuffixSkipped(t *testing.T) {
	ghc := github.NewMockClient()
	ghc.PullRequests[1] = &github.PullRequest{
		Number: 1,
		Title:  "[WIP] bot PR",
		Labels: []string{},
	}
	evt := &gh.PullRequestEvent{
		Action: gh.Ptr("synchronize"),
		Sender: &gh.User{Login: gh.Ptr("some-bot[bot]")},
		PullRequest: &gh.PullRequest{
			Number: gh.Ptr(1),
			Title:  gh.Ptr("[WIP] bot PR"),
			Draft:  gh.Ptr(false),
			Labels: []*gh.Label{},
		},
	}
	opts := &config.Options{}

	require.NoError(t, pr.HandlePREvent(context.Background(), ghc, "o", "r", evt, opts))
	assert.Empty(t, ghc.IssueLabels, "expected no label mutations for bot-sender event")
}

func TestHandlePREvent_BotLoginSkipped(t *testing.T) {
	ghc := github.NewMockClient()
	ghc.PullRequests[1] = &github.PullRequest{
		Number: 1,
		Title:  "Normal PR",
		Labels: []string{},
	}
	evt := &gh.PullRequestEvent{
		Action: gh.Ptr("synchronize"),
		Sender: &gh.User{Login: gh.Ptr("stern-bot")},
		PullRequest: &gh.PullRequest{
			Number: gh.Ptr(1),
			Title:  gh.Ptr("Normal PR"),
			Draft:  gh.Ptr(false),
			Labels: []*gh.Label{},
		},
	}
	opts := &config.Options{BotLogin: "stern-bot"}

	require.NoError(t, pr.HandlePREvent(context.Background(), ghc, "o", "r", evt, opts))
	assert.Empty(t, ghc.IssueLabels, "expected no label mutations for configured bot login")
}

// TestEventHandler_Handle_BotSuffixSkipped exercises the new
// (*EventHandler).Handle entry point and confirms the bot-guard
// short-circuit is preserved when the method-based API is used.
func TestEventHandler_Handle_BotSuffixSkipped(t *testing.T) {
	ghc := github.NewMockClient()
	ghc.PullRequests[1] = &github.PullRequest{
		Number: 1,
		Title:  "[WIP] bot PR",
		Labels: []string{},
	}
	evt := &gh.PullRequestEvent{
		Action: gh.Ptr("synchronize"),
		Sender: &gh.User{Login: gh.Ptr("some-bot[bot]")},
		PullRequest: &gh.PullRequest{
			Number: gh.Ptr(1),
			Title:  gh.Ptr("[WIP] bot PR"),
			Draft:  gh.Ptr(false),
			Labels: []*gh.Label{},
		},
	}

	handler := pr.NewEventHandler(ghc, &config.Options{})
	require.NoError(t, handler.Handle(context.Background(), "o", "r", evt))
	assert.Empty(t, ghc.IssueLabels, "expected no label mutations for bot-sender event via EventHandler")
}

// ghLabel returns a go-github Label pointer for embedding in event payloads.
func ghLabel(name string) *gh.Label {
	return &gh.Label{Name: gh.Ptr(name)}
}

// ghPR builds a minimal go-github PullRequest payload for event tests.
// Labels are flattened to label names via PullRequestFromGH; size fields
// drive HandlePREventSize; HeadSHA is what HandlePREventReviewAssignment
// uses to resolve OWNERS.
func ghPR(title string, additions, deletions int, labels ...string) *gh.PullRequest {
	ghLabels := make([]*gh.Label, 0, len(labels))
	for _, l := range labels {
		ghLabels = append(ghLabels, ghLabel(l))
	}
	return &gh.PullRequest{
		Number:    gh.Ptr(1),
		Title:     gh.Ptr(title),
		Draft:     gh.Ptr(false),
		Additions: gh.Ptr(additions),
		Deletions: gh.Ptr(deletions),
		Head:      &gh.PullRequestBranch{SHA: gh.Ptr("sha1")},
		Labels:    ghLabels,
	}
}

// ghEvent wraps a ghPR into a PullRequestEvent with the given action.
func ghEvent(action string, pullReq *gh.PullRequest) *gh.PullRequestEvent {
	return &gh.PullRequestEvent{
		Action:      gh.Ptr(action),
		Sender:      &gh.User{Login: gh.Ptr("alice")},
		PullRequest: pullReq,
	}
}

// TestEventHandler_Handle_Opened_DispatchesAllThree verifies the "opened"
// branch: handleWIP, HandlePREventSize, HandlePREventReviewAssignment all run.
func TestEventHandler_Handle_Opened_DispatchesAllThree(t *testing.T) {
	ghc := github.NewMockClient()
	ghc.PRFiles[1] = []string{"main.go"}
	ghc.FileContent["OWNERS@sha1"] = []byte("approvers:\n  - alice\n")

	opts := &config.Options{
		Size: config.SizeOptions{Buckets: []config.SizeBucket{{Name: "XS", Max: 10}}},
		ReviewAssignment: config.ReviewAssignmentOptions{
			Enabled:       true,
			Count:         1,
			LoadBalancing: "round-robin",
		},
	}

	handler := pr.NewEventHandler(ghc, opts)
	evt := ghEvent("opened", ghPR("fix bug", 5, 2))

	require.NoError(t, handler.Handle(context.Background(), "o", "r", evt))

	assert.True(t, ghc.IssueLabels[1]["size/XS"], "expected size/XS applied by HandlePREventSize")
	assert.Len(t, ghc.ReviewersRequested, 1, "expected HandlePREventReviewAssignment to request one reviewer")
}

// TestEventHandler_Handle_Reopened_NoReviewAssignment verifies "reopened"
// runs handleWIP + HandlePREventSize but NOT HandlePREventReviewAssignment.
func TestEventHandler_Handle_Reopened_NoReviewAssignment(t *testing.T) {
	ghc := github.NewMockClient()
	// Intentionally do NOT seed OWNERS — review assignment would fail if invoked.

	opts := &config.Options{
		Size:             config.SizeOptions{Buckets: []config.SizeBucket{{Name: "XS", Max: 10}}},
		ReviewAssignment: config.ReviewAssignmentOptions{Enabled: true, Count: 1},
	}
	handler := pr.NewEventHandler(ghc, opts)
	evt := ghEvent("reopened", ghPR("fix bug", 5, 2))

	require.NoError(t, handler.Handle(context.Background(), "o", "r", evt))

	assert.True(t, ghc.IssueLabels[1]["size/XS"], "expected HandlePREventSize to run on reopened")
	assert.Empty(t, ghc.ReviewersRequested, "expected review assignment NOT to run on reopened (opened-only)")
}

// TestEventHandler_Handle_Synchronize_InvalidatesLabels verifies "synchronize"
// runs invalidateLGTMOnPush + invalidateApproveOnPush and removes the labels.
func TestEventHandler_Handle_Synchronize_InvalidatesLabels(t *testing.T) {
	ghc := github.NewMockClient()
	ghc.IssueLabels[1] = map[string]bool{"lgtm": true, "approved": true}

	opts := &config.Options{
		LGTM:    config.LGTMOptions{InvalidateOnPush: true},
		Approve: config.ApproveOptions{InvalidateOnPush: true},
	}
	handler := pr.NewEventHandler(ghc, opts)
	evt := ghEvent("synchronize", ghPR("fix bug", 5, 2, "lgtm", "approved"))

	require.NoError(t, handler.Handle(context.Background(), "o", "r", evt))

	assert.False(t, ghc.IssueLabels[1]["lgtm"], "expected lgtm label removed on synchronize")
	assert.False(t, ghc.IssueLabels[1]["approved"], "expected approved label removed on synchronize")
}

// TestEventHandler_Handle_Edited_WithTitleChange verifies "edited" with
// Changes.Title set triggers handleWIP.
func TestEventHandler_Handle_Edited_WithTitleChange(t *testing.T) {
	ghc := github.NewMockClient()
	handler := pr.NewEventHandler(ghc, &config.Options{})
	evt := ghEvent("edited", ghPR("[WIP] in progress", 0, 0))
	evt.Changes = &gh.EditChange{
		Title: &gh.EditTitle{From: gh.Ptr("fix bug")},
	}

	require.NoError(t, handler.Handle(context.Background(), "o", "r", evt))

	assert.True(t, ghc.IssueLabels[1]["do-not-merge/wip"], "expected handleWIP to add WIP on title edit")
}

// TestEventHandler_Handle_Edited_NoTitleChange verifies "edited" without
// Changes.Title does NOT trigger handleWIP.
func TestEventHandler_Handle_Edited_NoTitleChange(t *testing.T) {
	ghc := github.NewMockClient()
	handler := pr.NewEventHandler(ghc, &config.Options{})
	evt := ghEvent("edited", ghPR("fix bug", 0, 0))
	// Changes present but Title is nil (e.g. body edit).
	evt.Changes = &gh.EditChange{Body: &gh.EditBody{From: gh.Ptr("old body")}}

	require.NoError(t, handler.Handle(context.Background(), "o", "r", evt))

	assert.False(t, ghc.IssueLabels[1]["do-not-merge/wip"], "expected handleWIP NOT to run on body-only edit")
}

// TestEventHandler_Handle_UnknownAction verifies actions outside the switch
// fall through silently with no sub-handler calls.
func TestEventHandler_Handle_UnknownAction(t *testing.T) {
	ghc := github.NewMockClient()
	ghc.IssueLabels[1] = map[string]bool{"lgtm": true, "approved": true}
	handler := pr.NewEventHandler(ghc, &config.Options{
		LGTM:    config.LGTMOptions{InvalidateOnPush: true},
		Approve: config.ApproveOptions{InvalidateOnPush: true},
	})
	evt := ghEvent("closed", ghPR("fix bug", 0, 0, "lgtm", "approved"))

	require.NoError(t, handler.Handle(context.Background(), "o", "r", evt))

	assert.True(t, ghc.IssueLabels[1]["lgtm"], "expected lgtm NOT removed on closed action")
	assert.True(t, ghc.IssueLabels[1]["approved"], "expected approved NOT removed on closed action")
}

// TestEventHandler_Handle_NilPR verifies the rawPR == nil early return.
func TestEventHandler_Handle_NilPR(t *testing.T) {
	ghc := github.NewMockClient()
	handler := pr.NewEventHandler(ghc, &config.Options{})
	evt := ghEvent("opened", nil)
	evt.PullRequest = nil

	require.NoError(t, handler.Handle(context.Background(), "o", "r", evt))

	assert.Empty(t, ghc.IssueLabels, "expected no label mutations when PR payload is nil")
}

// TestEventHandler_Handle_SubHandlerErrorSwallowed verifies that an error
// from a sub-handler is logged and Handle still returns nil.
func TestEventHandler_Handle_SubHandlerErrorSwallowed(t *testing.T) {
	ghc := github.NewMockClient()
	ghc.Errors["AddLabels"] = errors.New("boom")

	opts := &config.Options{
		Size: config.SizeOptions{Buckets: []config.SizeBucket{{Name: "XS", Max: 10}}},
	}
	handler := pr.NewEventHandler(ghc, opts)
	evt := ghEvent("opened", ghPR("fix bug", 5, 2))

	require.NoError(t, handler.Handle(context.Background(), "o", "r", evt),
		"Handle should swallow sub-handler errors and return nil")
}

// TestEventHandler_Handle_LGTMInvalidateDisabled verifies the short-circuit
// when InvalidateOnPush is false: RemoveLabel is never called.
func TestEventHandler_Handle_LGTMInvalidateDisabled(t *testing.T) {
	ghc := github.NewMockClient()
	ghc.IssueLabels[1] = map[string]bool{"lgtm": true}

	opts := &config.Options{
		LGTM: config.LGTMOptions{InvalidateOnPush: false},
	}
	handler := pr.NewEventHandler(ghc, opts)
	evt := ghEvent("synchronize", ghPR("fix bug", 0, 0, "lgtm"))

	require.NoError(t, handler.Handle(context.Background(), "o", "r", evt))

	assert.True(t, ghc.IssueLabels[1]["lgtm"], "expected lgtm NOT removed when InvalidateOnPush=false")
}

// TestEventHandler_Handle_LGTMNotFoundSwallowed verifies IsNotFoundError from
// RemoveLabel is swallowed, not propagated.
func TestEventHandler_Handle_LGTMNotFoundSwallowed(t *testing.T) {
	ghc := github.NewMockClient()
	ghc.IssueLabels[1] = map[string]bool{"lgtm": true}
	ghc.Errors["RemoveLabel"] = &gh.ErrorResponse{
		Response: &http.Response{StatusCode: http.StatusNotFound},
		Message:  "label not found",
	}

	opts := &config.Options{
		LGTM: config.LGTMOptions{InvalidateOnPush: true},
	}
	handler := pr.NewEventHandler(ghc, opts)
	evt := ghEvent("synchronize", ghPR("fix bug", 0, 0, "lgtm"))

	require.NoError(t, handler.Handle(context.Background(), "o", "r", evt),
		"Handle should swallow IsNotFoundError from RemoveLabel")
}
