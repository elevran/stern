package pr_test

import (
	"context"
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

// TestPREventHandler_Handle_BotSuffixSkipped exercises the new
// (*PREventHandler).Handle entry point and confirms the bot-guard
// short-circuit is preserved when the method-based API is used.
func TestPREventHandler_Handle_BotSuffixSkipped(t *testing.T) {
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

	handler := pr.NewPREventHandler(ghc, &config.Options{})
	require.NoError(t, handler.Handle(context.Background(), "o", "r", evt))
	assert.Empty(t, ghc.IssueLabels, "expected no label mutations for bot-sender event via PREventHandler")
}

