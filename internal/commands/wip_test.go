package commands_test

import (
	"context"
	"testing"

	gh "github.com/google/go-github/v72/github"

	"github.com/elevran/stern/internal/commands"
	"github.com/elevran/stern/internal/config"
	ghclient "github.com/elevran/stern/internal/github"
)

func wipOpts() *config.Options {
	return &config.Options{
		Merge: config.MergeOptions{
			Method:         "squash",
			BlockingLabels: []string{"do-not-merge/work-in-progress"},
		},
	}
}

func TestWIP_AddsLabel(t *testing.T) {
	sc, ghc := prContext("author")
	reg := commands.Registry{"wip": &commands.WIPHandler{}}
	commands.Dispatch(context.Background(), sc, "/wip", reg, ghc, wipOpts())

	if !ghc.IssueLabels[1]["do-not-merge/work-in-progress"] {
		t.Error("expected wip label added")
	}
}

func TestWIP_RemovesLabel(t *testing.T) {
	sc, ghc := prContext("author")
	sc.PR.Labels = []*gh.Label{{Name: gh.Ptr("do-not-merge/work-in-progress")}}
	ghc.IssueLabels[1] = map[string]bool{"do-not-merge/work-in-progress": true}

	reg := commands.Registry{"wip": &commands.WIPHandler{}}
	commands.Dispatch(context.Background(), sc, "/wip", reg, ghc, wipOpts())

	if ghc.IssueLabels[1]["do-not-merge/work-in-progress"] {
		t.Error("expected wip label removed")
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
		{"WIPping something", false}, // no colon
		{"", false},
	}
	for _, tc := range cases {
		got := commands.IsTitleWIP(tc.title)
		if got != tc.want {
			t.Errorf("IsTitleWIP(%q) = %v, want %v", tc.title, got, tc.want)
		}
	}
}

func TestHandlePREventWIP_AddsOnWIPTitle(t *testing.T) {
	ghc := ghclient.NewMockClient()
	pr := &gh.PullRequest{
		Number: gh.Ptr(1),
		Title:  gh.Ptr("[WIP] test"),
		Draft:  gh.Ptr(false),
		Labels: []*gh.Label{},
	}

	err := commands.HandlePREventWIP(context.Background(), ghc, "o", "r", pr, wipOpts(), false)
	if err != nil {
		t.Fatalf("HandlePREventWIP() error = %v", err)
	}
	if !ghc.IssueLabels[1]["do-not-merge/work-in-progress"] {
		t.Error("expected wip label added for WIP title")
	}
}

func TestHandlePREventWIP_RemovesWhenTitleClean(t *testing.T) {
	ghc := ghclient.NewMockClient()
	ghc.IssueLabels[1] = map[string]bool{"do-not-merge/work-in-progress": true}
	pr := &gh.PullRequest{
		Number: gh.Ptr(1),
		Title:  gh.Ptr("Fix the bug"),
		Draft:  gh.Ptr(false),
		Labels: []*gh.Label{{Name: gh.Ptr("do-not-merge/work-in-progress")}},
	}
	ghc.PullRequests[1] = pr

	err := commands.HandlePREventWIP(context.Background(), ghc, "o", "r", pr, wipOpts(), true)
	if err != nil {
		t.Fatalf("HandlePREventWIP() error = %v", err)
	}
	if ghc.IssueLabels[1]["do-not-merge/work-in-progress"] {
		t.Error("expected wip label removed when title is clean")
	}
}

func TestHandlePREventWIP_DraftAddsLabel(t *testing.T) {
	ghc := ghclient.NewMockClient()
	pr := &gh.PullRequest{
		Number: gh.Ptr(1),
		Title:  gh.Ptr("Normal PR"),
		Draft:  gh.Ptr(true),
		Labels: []*gh.Label{},
	}

	err := commands.HandlePREventWIP(context.Background(), ghc, "o", "r", pr, wipOpts(), false)
	if err != nil {
		t.Fatalf("HandlePREventWIP() error = %v", err)
	}
	if !ghc.IssueLabels[1]["do-not-merge/work-in-progress"] {
		t.Error("expected wip label added for draft PR")
	}
}

func TestWIP_NotOnPR(t *testing.T) {
	sc, ghc := prContext("author")
	sc.PR = nil

	reg := commands.Registry{"wip": &commands.WIPHandler{}}
	commands.Dispatch(context.Background(), sc, "/wip", reg, ghc, wipOpts())

	if len(ghc.Reactions) == 0 || ghc.Reactions[0].Content != "-1" {
		t.Errorf("expected -1 for /wip on non-PR, got %v", ghc.Reactions)
	}
}
