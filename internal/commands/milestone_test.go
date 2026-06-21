package commands_test

import (
	"context"
	"testing"

	"github.com/elevran/stern/internal/commands"
	"github.com/elevran/stern/internal/config"
	"github.com/elevran/stern/internal/event"
	"github.com/elevran/stern/internal/github"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func milestoneOpts() *config.Options {
	return &config.Options{
		Plugins: []string{"milestone"},
	}
}

func TestMilestone_SetByID(t *testing.T) {
	sc, ghc := prContext("author")
	sc.Author = "maintainer"
	ghc.WriteAccess["elevran/stern/maintainer"] = true

	reg := commands.Registry{"milestone": {Factory: commands.NewMilestoneHandler}}
	commands.Dispatch(context.Background(), sc, "/milestone 7", reg, ghc, milestoneOpts())

	require.Len(t, ghc.MilestoneSet, 1)
	assert.Equal(t, 1, ghc.MilestoneSet[0].Number)
	assert.Equal(t, 7, ghc.MilestoneSet[0].MilestoneID)
	assert.Equal(t, 7, ghc.IssueMilestone[1])
	require.NotEmpty(t, ghc.Reactions)
	assert.Equal(t, "+1", ghc.Reactions[0].Content)
}

func TestMilestone_SetByTitle_Exact(t *testing.T) {
	sc, ghc := prContext("author")
	sc.Author = "maintainer"
	ghc.WriteAccess["elevran/stern/maintainer"] = true
	ghc.Milestones[5] = github.Milestone{Number: 5, Title: "v1.0"}

	reg := commands.Registry{"milestone": {Factory: commands.NewMilestoneHandler}}
	commands.Dispatch(context.Background(), sc, "/milestone v1.0", reg, ghc, milestoneOpts())

	require.Len(t, ghc.MilestoneSet, 1)
	assert.Equal(t, 5, ghc.MilestoneSet[0].MilestoneID)
}

func TestMilestone_SetByTitle_CaseInsensitive(t *testing.T) {
	sc, ghc := prContext("author")
	sc.Author = "maintainer"
	ghc.WriteAccess["elevran/stern/maintainer"] = true
	ghc.Milestones[5] = github.Milestone{Number: 5, Title: "v1.0"}

	reg := commands.Registry{"milestone": {Factory: commands.NewMilestoneHandler}}
	commands.Dispatch(context.Background(), sc, "/milestone V1.0", reg, ghc, milestoneOpts())

	require.Len(t, ghc.MilestoneSet, 1)
	assert.Equal(t, 5, ghc.MilestoneSet[0].MilestoneID)
}

func TestMilestone_Clear(t *testing.T) {
	sc, ghc := prContext("author")
	sc.Author = "maintainer"
	ghc.WriteAccess["elevran/stern/maintainer"] = true
	ghc.IssueMilestone[1] = 5

	reg := commands.Registry{"milestone": {Factory: commands.NewMilestoneHandler}}
	commands.Dispatch(context.Background(), sc, "/milestone clear", reg, ghc, milestoneOpts())

	require.Len(t, ghc.MilestoneCleared, 1)
	assert.Equal(t, 1, ghc.MilestoneCleared[0])
	_, ok := ghc.IssueMilestone[1]
	assert.False(t, ok, "expected IssueMilestone[1] deleted")
	require.NotEmpty(t, ghc.Reactions)
	assert.Equal(t, "+1", ghc.Reactions[0].Content)
}

func TestMilestone_Clear_CaseInsensitive(t *testing.T) {
	sc, ghc := prContext("author")
	sc.Author = "maintainer"
	ghc.WriteAccess["elevran/stern/maintainer"] = true
	ghc.IssueMilestone[1] = 5

	reg := commands.Registry{"milestone": {Factory: commands.NewMilestoneHandler}}
	commands.Dispatch(context.Background(), sc, "/milestone CLEAR", reg, ghc, milestoneOpts())

	assert.Len(t, ghc.MilestoneCleared, 1)
}

func TestMilestone_TitleNotFound(t *testing.T) {
	sc, ghc := prContext("author")
	sc.Author = "maintainer"
	ghc.WriteAccess["elevran/stern/maintainer"] = true
	ghc.Milestones[5] = github.Milestone{Number: 5, Title: "v1.0"}

	reg := commands.Registry{"milestone": {Factory: commands.NewMilestoneHandler}}
	commands.Dispatch(context.Background(), sc, "/milestone v2.0", reg, ghc, milestoneOpts())

	assert.Empty(t, ghc.MilestoneSet, "expected no SetMilestone call for unknown title")
	require.NotEmpty(t, ghc.Reactions)
	assert.Equal(t, "-1", ghc.Reactions[0].Content)
	foundUsage := false
	for _, c := range ghc.Comments {
		if c.Number == 1 && c.Body == "milestone not found: v2.0" {
			foundUsage = true
		}
	}
	assert.True(t, foundUsage, "expected comment 'milestone not found: v2.0', got %+v", ghc.Comments)
}

func TestMilestone_MissingArg(t *testing.T) {
	sc, ghc := prContext("author")
	sc.Author = "maintainer"
	ghc.WriteAccess["elevran/stern/maintainer"] = true

	reg := commands.Registry{"milestone": {Factory: commands.NewMilestoneHandler}}
	commands.Dispatch(context.Background(), sc, "/milestone", reg, ghc, milestoneOpts())

	assert.Empty(t, ghc.MilestoneSet, "expected no mutation when arg missing")
	assert.Empty(t, ghc.MilestoneCleared, "expected no mutation when arg missing")
	require.NotEmpty(t, ghc.Reactions)
	assert.Equal(t, "-1", ghc.Reactions[0].Content)
}

func TestMilestone_RequiresWriteAccess(t *testing.T) {
	sc, ghc := prContext("author")
	sc.Author = "outsider"
	ghc.WriteAccess["elevran/stern/outsider"] = false

	reg := commands.Registry{"milestone": {Factory: commands.NewMilestoneHandler}}
	commands.Dispatch(context.Background(), sc, "/milestone 7", reg, ghc, milestoneOpts())

	assert.Empty(t, ghc.MilestoneSet, "expected no SetMilestone without write access")
	require.NotEmpty(t, ghc.Reactions)
	assert.Equal(t, "-1", ghc.Reactions[0].Content)
}

func TestMilestone_WorksOnIssue(t *testing.T) {
	sc := &event.Context{
		Org:         "elevran",
		Repo:        "stern",
		CommentID:   42,
		Author:      "maintainer",
		IssueNumber: 99,
		PR:          nil,
	}
	ghc := github.NewMockClient()
	ghc.WriteAccess["elevran/stern/maintainer"] = true

	reg := commands.Registry{"milestone": {Factory: commands.NewMilestoneHandler}}
	commands.Dispatch(context.Background(), sc, "/milestone 3", reg, ghc, milestoneOpts())

	require.Len(t, ghc.MilestoneSet, 1)
	assert.Equal(t, 99, ghc.MilestoneSet[0].Number)
	assert.Equal(t, 3, ghc.MilestoneSet[0].MilestoneID)
	require.NotEmpty(t, ghc.Reactions)
	assert.Equal(t, "+1", ghc.Reactions[0].Content)
}
