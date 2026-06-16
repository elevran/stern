package commands_test

import (
	"context"
	"testing"

	gh "github.com/google/go-github/v72/github"

	"github.com/elevran/stern/internal/commands"
	"github.com/elevran/stern/internal/config"
	"github.com/elevran/stern/internal/event"
	ghclient "github.com/elevran/stern/internal/github"
)

func prContext(prAuthor string) (*event.Context, *ghclient.MockClient) {
	ghc := ghclient.NewMockClient()
	pr := &gh.PullRequest{
		Number: gh.Ptr(1),
		User:   &gh.User{Login: gh.Ptr(prAuthor)},
		Head:   &gh.PullRequestBranch{SHA: gh.Ptr("abc123")},
		Labels: []*gh.Label{},
	}
	ghc.PullRequests[1] = pr

	sc := &event.Context{
		Org:         "elevran",
		Repo:        "stern",
		CommentID:   10,
		Author:      "reviewer",
		IssueNumber: 1,
		PR:          pr,
	}
	return sc, ghc
}

func lgtmOpts(allowSelf bool) *config.Options {
	return &config.Options{
		LGTM: config.LGTMOptions{
			AllowSelfLGTM:    allowSelf,
			InvalidateOnPush: true,
		},
		Merge: config.MergeOptions{
			Method:         "squash",
			BlockingLabels: []string{"do-not-merge/hold"},
		},
	}
}

func TestLGTM_AddsLabel(t *testing.T) {
	sc, ghc := prContext("author")
	reg := commands.Registry{"lgtm": &commands.LGTMHandler{}}
	commands.Dispatch(context.Background(), sc, "/lgtm", reg, ghc, lgtmOpts(false))

	if !ghc.IssueLabels[1]["lgtm"] {
		t.Error("expected lgtm label to be added")
	}
}

func TestLGTM_Cancel_RemovesLabel(t *testing.T) {
	sc, ghc := prContext("author")
	ghc.IssueLabels[1] = map[string]bool{"lgtm": true}
	reg := commands.Registry{"lgtm": &commands.LGTMHandler{}}
	commands.Dispatch(context.Background(), sc, "/lgtm cancel", reg, ghc, lgtmOpts(false))

	if ghc.IssueLabels[1]["lgtm"] {
		t.Error("expected lgtm label to be removed on cancel")
	}
}

func TestLGTM_SelfLGTMDenied(t *testing.T) {
	sc, ghc := prContext("reviewer") // PR author == commenter
	sc.Author = "reviewer"
	reg := commands.Registry{"lgtm": &commands.LGTMHandler{}}
	commands.Dispatch(context.Background(), sc, "/lgtm", reg, ghc, lgtmOpts(false))

	if ghc.IssueLabels[1]["lgtm"] {
		t.Error("expected lgtm NOT added when author lgtms own PR")
	}
	if len(ghc.Reactions) == 0 || ghc.Reactions[0].Content != "-1" {
		t.Errorf("expected -1 reaction for self-lgtm, got %v", ghc.Reactions)
	}
}

func TestLGTM_SelfLGTMAllowed(t *testing.T) {
	sc, ghc := prContext("reviewer")
	sc.Author = "reviewer"
	reg := commands.Registry{"lgtm": &commands.LGTMHandler{}}
	commands.Dispatch(context.Background(), sc, "/lgtm", reg, ghc, lgtmOpts(true))

	if !ghc.IssueLabels[1]["lgtm"] {
		t.Error("expected lgtm added when allow_self_lgtm=true")
	}
}

func TestLGTM_NonReviewerDeniedByOwners(t *testing.T) {
	sc, ghc := prContext("author")
	sc.Author = "outsider"
	ghc.FileContent["OWNERS@abc123"] = []byte("reviewers:\n  - alice\n  - bob\n")
	ghc.PRFiles[1] = []*gh.CommitFile{{Filename: gh.Ptr("README.md")}}

	reg := commands.Registry{"lgtm": &commands.LGTMHandler{}}
	commands.Dispatch(context.Background(), sc, "/lgtm", reg, ghc, lgtmOpts(false))

	if ghc.IssueLabels[1]["lgtm"] {
		t.Error("expected lgtm NOT added for non-reviewer when OWNERS present")
	}
	if len(ghc.Reactions) == 0 || ghc.Reactions[0].Content != "-1" {
		t.Errorf("expected -1 reaction, got %v", ghc.Reactions)
	}
}

func TestLGTM_NoOwnersAllowsAnyCommenter(t *testing.T) {
	sc, ghc := prContext("author")
	sc.Author = "anyone"
	// No OWNERS files loaded in mock
	ghc.PRFiles[1] = []*gh.CommitFile{{Filename: gh.Ptr("README.md")}}

	reg := commands.Registry{"lgtm": &commands.LGTMHandler{}}
	commands.Dispatch(context.Background(), sc, "/lgtm", reg, ghc, lgtmOpts(false))

	if !ghc.IssueLabels[1]["lgtm"] {
		t.Error("expected lgtm added when no OWNERS files present")
	}
}

func TestLGTM_NotOnPR_Denied(t *testing.T) {
	sc := &event.Context{
		Org:         "o",
		Repo:        "r",
		Author:      "alice",
		IssueNumber: 5,
		PR:          nil, // not a PR
	}
	ghc := ghclient.NewMockClient()
	reg := commands.Registry{"lgtm": &commands.LGTMHandler{}}
	commands.Dispatch(context.Background(), sc, "/lgtm", reg, ghc, lgtmOpts(false))

	if len(ghc.Reactions) == 0 || ghc.Reactions[0].Content != "-1" {
		t.Errorf("expected -1 for /lgtm on non-PR, got %v", ghc.Reactions)
	}
}
