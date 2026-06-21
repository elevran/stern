package commands_test

import (
	"context"
	"errors"
	"testing"

	"github.com/elevran/stern/internal/commands"
	"github.com/elevran/stern/internal/config"
	"github.com/elevran/stern/internal/event"
	"github.com/elevran/stern/internal/github"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func prContext(prAuthor string) (*event.Context, *github.MockClient) {
	ghc := github.NewMockClient()
	p := &github.PullRequest{
		Number:  1,
		Author:  prAuthor,
		HeadSHA: "abc123",
		NodeID:  "test-node-id",
		Labels:  []string{},
	}
	ghc.PullRequests[1] = p

	sc := &event.Context{
		Org:         "elevran",
		Repo:        "stern",
		CommentID:   10,
		Author:      "reviewer",
		IssueNumber: 1,
		PR:          p,
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
	reg := commands.Registry{"lgtm": {Factory: commands.NewLGTMHandler}}
	commands.Dispatch(context.Background(), sc, "/lgtm", reg, ghc, lgtmOpts(false))

	assert.True(t, ghc.IssueLabels[1]["lgtm"], "expected lgtm label to be added")
	require.NotEmpty(t, ghc.Reactions)
	assert.Equal(t, "+1", ghc.Reactions[0].Content, "expected +1 reaction after successful /lgtm")
}

func TestLGTM_Cancel_RemovesLabel(t *testing.T) {
	sc, ghc := prContext("author")
	ghc.IssueLabels[1] = map[string]bool{"lgtm": true}
	reg := commands.Registry{"lgtm": {Factory: commands.NewLGTMHandler}}
	commands.Dispatch(context.Background(), sc, "/lgtm cancel", reg, ghc, lgtmOpts(false))

	assert.False(t, ghc.IssueLabels[1]["lgtm"], "expected lgtm label to be removed on cancel")
	require.NotEmpty(t, ghc.Reactions)
	assert.Equal(t, "+1", ghc.Reactions[0].Content, "expected +1 reaction after successful /lgtm cancel")
}

func TestLGTM_SelfLGTMDenied(t *testing.T) {
	sc, ghc := prContext("reviewer") // PR author == commenter
	sc.Author = "reviewer"
	reg := commands.Registry{"lgtm": {Factory: commands.NewLGTMHandler}}
	commands.Dispatch(context.Background(), sc, "/lgtm", reg, ghc, lgtmOpts(false))

	assert.False(t, ghc.IssueLabels[1]["lgtm"], "expected lgtm NOT added when author lgtms own PR")
	require.NotEmpty(t, ghc.Reactions)
	assert.Equal(t, "-1", ghc.Reactions[0].Content, "expected -1 reaction for self-lgtm")
}

func TestLGTM_SelfLGTMAllowed(t *testing.T) {
	sc, ghc := prContext("reviewer")
	sc.Author = "reviewer"
	reg := commands.Registry{"lgtm": {Factory: commands.NewLGTMHandler}}
	commands.Dispatch(context.Background(), sc, "/lgtm", reg, ghc, lgtmOpts(true))

	assert.True(t, ghc.IssueLabels[1]["lgtm"], "expected lgtm added when allow_self_lgtm=true")
	require.NotEmpty(t, ghc.Reactions)
	assert.Equal(t, "+1", ghc.Reactions[0].Content, "expected +1 reaction after successful self-/lgtm")
}

func TestLGTM_NonReviewerDeniedByOwners(t *testing.T) {
	sc, ghc := prContext("author")
	sc.Author = "outsider"
	ghc.FileContent["OWNERS@abc123"] = []byte("reviewers:\n  - alice\n  - bob\n")
	ghc.PRFiles[1] = []string{"README.md"}

	reg := commands.Registry{"lgtm": {Factory: commands.NewLGTMHandler}}
	commands.Dispatch(context.Background(), sc, "/lgtm", reg, ghc, lgtmOpts(false))

	assert.False(t, ghc.IssueLabels[1]["lgtm"], "expected lgtm NOT added for non-reviewer when OWNERS present")
	require.NotEmpty(t, ghc.Reactions)
	assert.Equal(t, "-1", ghc.Reactions[0].Content, "expected -1 reaction")
}

func TestLGTM_NoOwnersAllowsAnyCommenter(t *testing.T) {
	sc, ghc := prContext("author")
	sc.Author = "anyone"
	// No OWNERS files loaded in mock
	ghc.PRFiles[1] = []string{"README.md"}

	reg := commands.Registry{"lgtm": {Factory: commands.NewLGTMHandler}}
	commands.Dispatch(context.Background(), sc, "/lgtm", reg, ghc, lgtmOpts(false))

	assert.True(t, ghc.IssueLabels[1]["lgtm"], "expected lgtm added when no OWNERS files present")
	require.NotEmpty(t, ghc.Reactions)
	assert.Equal(t, "+1", ghc.Reactions[0].Content, "expected +1 reaction after successful /lgtm")
}

func TestLGTM_NotOnPR_Denied(t *testing.T) {
	sc := &event.Context{
		Org:         "o",
		Repo:        "r",
		Author:      "alice",
		IssueNumber: 5,
		PR:          nil, // not a PR
	}
	ghc := github.NewMockClient()
	reg := commands.Registry{"lgtm": {Factory: commands.NewLGTMHandler}}
	commands.Dispatch(context.Background(), sc, "/lgtm", reg, ghc, lgtmOpts(false))

	require.NotEmpty(t, ghc.Reactions)
	assert.Equal(t, "-1", ghc.Reactions[0].Content, "expected -1 for /lgtm on non-PR")
}

func TestLGTM_HandleError_SuppressesPost(t *testing.T) {
	sc, ghc := prContext("author")
	ghc.Errors["AddLabels"] = errors.New("boom")

	reg := commands.Registry{"lgtm": {Factory: commands.NewLGTMHandler}}
	commands.Dispatch(context.Background(), sc, "/lgtm", reg, ghc, lgtmOpts(false))

	assert.Empty(t, ghc.AutoMergeEnabled, "expected Post NOT to run when Handle errors")
	assert.Empty(t, ghc.AutoMergeDisabled, "expected Post NOT to run when Handle errors")
	require.NotEmpty(t, ghc.Reactions)
	assert.Equal(t, "confused", ghc.Reactions[0].Content, "expected confused reaction on internal error")
}
