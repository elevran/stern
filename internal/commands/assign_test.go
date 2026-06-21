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

func assignReg() commands.Registry {
	return commands.Registry{
		"assign":   commands.NewAssignHandler("assign"),
		"unassign": commands.NewAssignHandler("unassign"),
	}
}

func assignOpts() *config.Options {
	return &config.Options{}
}

func TestAssign_SelfAssign(t *testing.T) {
	sc, ghc := prContext("author")
	sc.Author = "alice"
	ghc.OrgMembers["elevran/alice"] = true

	commands.Dispatch(context.Background(), sc, "/assign", assignReg(), ghc, assignOpts())

	require.Len(t, ghc.AssigneesAdded, 1)
	assert.Equal(t, []string{"alice"}, ghc.AssigneesAdded[0].Users)
	require.NotEmpty(t, ghc.Reactions)
	assert.Equal(t, "+1", ghc.Reactions[0].Content)
}

func TestAssign_SelfAssign_NotOrgMember_Denied(t *testing.T) {
	sc, ghc := prContext("author")
	sc.Author = "alice"
	ghc.OrgMembers["elevran/alice"] = false

	commands.Dispatch(context.Background(), sc, "/assign", assignReg(), ghc, assignOpts())

	assert.Empty(t, ghc.AssigneesAdded, "expected AddAssignees NOT called for non-org-member")
	require.NotEmpty(t, ghc.Reactions)
	assert.Equal(t, "-1", ghc.Reactions[0].Content)
}

func TestAssign_OtherUser_RequiresWriteAccess(t *testing.T) {
	sc, ghc := prContext("author")
	sc.Author = "bob"
	ghc.WriteAccess["elevran/stern/bob"] = false

	commands.Dispatch(context.Background(), sc, "/assign @carol", assignReg(), ghc, assignOpts())

	assert.Empty(t, ghc.AssigneesAdded, "expected AddAssignees NOT called for non-writer")
	require.NotEmpty(t, ghc.Reactions)
	assert.Equal(t, "-1", ghc.Reactions[0].Content)
}

func TestAssign_OtherUser_WithWriteAccess(t *testing.T) {
	sc, ghc := prContext("author")
	sc.Author = "maintainer"
	ghc.WriteAccess["elevran/stern/maintainer"] = true

	commands.Dispatch(context.Background(), sc, "/assign @carol", assignReg(), ghc, assignOpts())

	require.Len(t, ghc.AssigneesAdded, 1)
	assert.Equal(t, []string{"carol"}, ghc.AssigneesAdded[0].Users)
}

func TestAssign_MultiUser(t *testing.T) {
	sc, ghc := prContext("author")
	sc.Author = "maintainer"
	ghc.WriteAccess["elevran/stern/maintainer"] = true

	commands.Dispatch(context.Background(), sc, "/assign @carol @dan", assignReg(), ghc, assignOpts())

	require.Len(t, ghc.AssigneesAdded, 1)
	assert.Equal(t, []string{"carol", "dan"}, ghc.AssigneesAdded[0].Users)
}

func TestAssign_StripsAtAndDeduplicates(t *testing.T) {
	sc, ghc := prContext("author")
	sc.Author = "maintainer"
	ghc.WriteAccess["elevran/stern/maintainer"] = true

	commands.Dispatch(context.Background(), sc, "/assign @carol @CAROL @dan", assignReg(), ghc, assignOpts())

	assert.Equal(t, []string{"carol", "dan"}, ghc.AssigneesAdded[0].Users)
}

func TestUnassign_Self(t *testing.T) {
	sc, ghc := prContext("author")
	sc.Author = "alice"
	ghc.OrgMembers["elevran/alice"] = true
	ghc.Assignees[1] = []string{"alice"}

	commands.Dispatch(context.Background(), sc, "/unassign", assignReg(), ghc, assignOpts())

	require.Len(t, ghc.AssigneesRemoved, 1)
	assert.Equal(t, []string{"alice"}, ghc.AssigneesRemoved[0].Users)
}

func TestUnassign_Other(t *testing.T) {
	sc, ghc := prContext("author")
	sc.Author = "maintainer"
	ghc.WriteAccess["elevran/stern/maintainer"] = true

	commands.Dispatch(context.Background(), sc, "/unassign @carol", assignReg(), ghc, assignOpts())

	require.Len(t, ghc.AssigneesRemoved, 1)
	assert.Equal(t, []string{"carol"}, ghc.AssigneesRemoved[0].Users)
}

func TestAssign_NotOnPR(t *testing.T) {
	sc := &event.Context{
		Org:         "o",
		Repo:        "r",
		Author:      "alice",
		IssueNumber: 5,
		PR:          nil,
	}
	ghc := github.NewMockClient()
	ghc.OrgMembers["o/alice"] = true

	commands.Dispatch(context.Background(), sc, "/assign", assignReg(), ghc, assignOpts())

	assert.Empty(t, ghc.AssigneesAdded, "expected AddAssignees NOT called when not on a PR")
	require.NotEmpty(t, ghc.Reactions)
	assert.Equal(t, "-1", ghc.Reactions[0].Content)
}

func TestAssign_HandleError_SuppressesPost(t *testing.T) {
	sc, ghc := prContext("author")
	sc.Author = "alice"
	ghc.OrgMembers["elevran/alice"] = true
	ghc.Errors["AddAssignees"] = errors.New("boom")

	commands.Dispatch(context.Background(), sc, "/assign", assignReg(), ghc, assignOpts())

	assert.Empty(t, ghc.AutoMergeEnabled, "expected Post NOT to run when Handle errors")
	assert.Empty(t, ghc.AutoMergeDisabled, "expected Post NOT to run when Handle errors")
	require.NotEmpty(t, ghc.Reactions)
	assert.Equal(t, "confused", ghc.Reactions[0].Content, "expected confused reaction on internal error")
}