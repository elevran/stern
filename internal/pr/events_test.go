package pr_test

import (
	"context"
	"testing"

	gh "github.com/google/go-github/v72/github"

	"github.com/elevran/stern/internal/config"
	ghclient "github.com/elevran/stern/internal/github"
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
		{"WIPping something", false},
		{"", false},
	}
	for _, tc := range cases {
		got := pr.IsTitleWIP(tc.title)
		if got != tc.want {
			t.Errorf("IsTitleWIP(%q) = %v, want %v", tc.title, got, tc.want)
		}
	}
}

func TestHandlePREventWIP_AddsOnWIPTitle(t *testing.T) {
	ghc := ghclient.NewMockClient()
	p := &gh.PullRequest{
		Number: gh.Ptr(1),
		Title:  gh.Ptr("[WIP] test"),
		Draft:  gh.Ptr(false),
		Labels: []*gh.Label{},
	}

	if err := pr.HandlePREventWIP(context.Background(), ghc, "o", "r", p, wipOpts()); err != nil {
		t.Fatalf("HandlePREventWIP() error = %v", err)
	}
	if !ghc.IssueLabels[1]["do-not-merge/wip"] {
		t.Error("expected wip label added for WIP title")
	}
}

func TestHandlePREventWIP_RemovesWhenTitleClean(t *testing.T) {
	ghc := ghclient.NewMockClient()
	ghc.IssueLabels[1] = map[string]bool{"do-not-merge/wip": true}
	p := &gh.PullRequest{
		Number: gh.Ptr(1),
		Title:  gh.Ptr("Fix the bug"),
		Draft:  gh.Ptr(false),
		Labels: []*gh.Label{{Name: gh.Ptr("do-not-merge/wip")}},
	}
	ghc.PullRequests[1] = p

	if err := pr.HandlePREventWIP(context.Background(), ghc, "o", "r", p, wipOpts()); err != nil {
		t.Fatalf("HandlePREventWIP() error = %v", err)
	}
	if ghc.IssueLabels[1]["do-not-merge/wip"] {
		t.Error("expected wip label removed when title is clean")
	}
}

func TestHandlePREventWIP_DraftAddsLabel(t *testing.T) {
	ghc := ghclient.NewMockClient()
	p := &gh.PullRequest{
		Number: gh.Ptr(1),
		Title:  gh.Ptr("Normal PR"),
		Draft:  gh.Ptr(true),
		Labels: []*gh.Label{},
	}

	if err := pr.HandlePREventWIP(context.Background(), ghc, "o", "r", p, wipOpts()); err != nil {
		t.Fatalf("HandlePREventWIP() error = %v", err)
	}
	if !ghc.IssueLabels[1]["do-not-merge/wip"] {
		t.Error("expected wip label added for draft PR")
	}
}

func TestInvalidateLGTMOnPush(t *testing.T) {
	ghc := ghclient.NewMockClient()
	ghc.IssueLabels[1] = map[string]bool{"lgtm": true}
	p := &gh.PullRequest{
		Number: gh.Ptr(1),
		Labels: []*gh.Label{{Name: gh.Ptr("lgtm")}},
	}
	opts := &config.Options{
		LGTM: config.LGTMOptions{InvalidateOnPush: true},
	}

	if err := pr.InvalidateLGTMOnPush(context.Background(), ghc, "o", "r", p, opts); err != nil {
		t.Fatalf("InvalidateLGTMOnPush() error = %v", err)
	}
	if ghc.IssueLabels[1]["lgtm"] {
		t.Error("expected lgtm label removed on push")
	}
}

func TestInvalidateApproveOnPush(t *testing.T) {
	ghc := ghclient.NewMockClient()
	ghc.IssueLabels[1] = map[string]bool{"approved": true}
	p := &gh.PullRequest{
		Number: gh.Ptr(1),
		Labels: []*gh.Label{{Name: gh.Ptr("approved")}},
	}
	opts := &config.Options{
		Approve: config.ApproveOptions{InvalidateOnPush: true},
	}

	if err := pr.InvalidateApproveOnPush(context.Background(), ghc, "o", "r", p, opts); err != nil {
		t.Fatalf("InvalidateApproveOnPush() error = %v", err)
	}
	if ghc.IssueLabels[1]["approved"] {
		t.Error("expected approved label removed on push")
	}
}
