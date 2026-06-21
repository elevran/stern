package commands_test

import (
	"context"
	"testing"

	"github.com/elevran/stern/internal/commands"
	"github.com/elevran/stern/internal/config"
	"github.com/elevran/stern/internal/event"
	"github.com/elevran/stern/internal/github"
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

	reg := commands.Registry{"milestone": commands.NewMilestoneHandler}
	commands.Dispatch(context.Background(), sc, "/milestone 7", reg, ghc, milestoneOpts())

	if len(ghc.MilestoneSet) != 1 || ghc.MilestoneSet[0].Number != 1 || ghc.MilestoneSet[0].MilestoneID != 7 {
		t.Errorf("expected SetMilestone(number=1, milestoneID=7), got %+v", ghc.MilestoneSet)
	}
	if ghc.IssueMilestone[1] != 7 {
		t.Errorf("expected IssueMilestone[1]==7, got %d", ghc.IssueMilestone[1])
	}
	if len(ghc.Reactions) == 0 || ghc.Reactions[0].Content != "+1" {
		t.Errorf("expected +1 reaction, got %v", ghc.Reactions)
	}
}

func TestMilestone_SetByTitle_Exact(t *testing.T) {
	sc, ghc := prContext("author")
	sc.Author = "maintainer"
	ghc.WriteAccess["elevran/stern/maintainer"] = true
	ghc.Milestones[5] = github.Milestone{Number: 5, Title: "v1.0"}

	reg := commands.Registry{"milestone": commands.NewMilestoneHandler}
	commands.Dispatch(context.Background(), sc, "/milestone v1.0", reg, ghc, milestoneOpts())

	if len(ghc.MilestoneSet) != 1 || ghc.MilestoneSet[0].MilestoneID != 5 {
		t.Errorf("expected SetMilestone(milestoneID=5), got %+v", ghc.MilestoneSet)
	}
}

func TestMilestone_SetByTitle_CaseInsensitive(t *testing.T) {
	sc, ghc := prContext("author")
	sc.Author = "maintainer"
	ghc.WriteAccess["elevran/stern/maintainer"] = true
	ghc.Milestones[5] = github.Milestone{Number: 5, Title: "v1.0"}

	reg := commands.Registry{"milestone": commands.NewMilestoneHandler}
	commands.Dispatch(context.Background(), sc, "/milestone V1.0", reg, ghc, milestoneOpts())

	if len(ghc.MilestoneSet) != 1 || ghc.MilestoneSet[0].MilestoneID != 5 {
		t.Errorf("expected SetMilestone(milestoneID=5), got %+v", ghc.MilestoneSet)
	}
}

func TestMilestone_Clear(t *testing.T) {
	sc, ghc := prContext("author")
	sc.Author = "maintainer"
	ghc.WriteAccess["elevran/stern/maintainer"] = true
	ghc.IssueMilestone[1] = 5

	reg := commands.Registry{"milestone": commands.NewMilestoneHandler}
	commands.Dispatch(context.Background(), sc, "/milestone clear", reg, ghc, milestoneOpts())

	if len(ghc.MilestoneCleared) != 1 || ghc.MilestoneCleared[0] != 1 {
		t.Errorf("expected ClearMilestone(number=1), got %+v", ghc.MilestoneCleared)
	}
	if _, ok := ghc.IssueMilestone[1]; ok {
		t.Errorf("expected IssueMilestone[1] deleted, got %d", ghc.IssueMilestone[1])
	}
	if len(ghc.Reactions) == 0 || ghc.Reactions[0].Content != "+1" {
		t.Errorf("expected +1 reaction, got %v", ghc.Reactions)
	}
}

func TestMilestone_Clear_CaseInsensitive(t *testing.T) {
	sc, ghc := prContext("author")
	sc.Author = "maintainer"
	ghc.WriteAccess["elevran/stern/maintainer"] = true
	ghc.IssueMilestone[1] = 5

	reg := commands.Registry{"milestone": commands.NewMilestoneHandler}
	commands.Dispatch(context.Background(), sc, "/milestone CLEAR", reg, ghc, milestoneOpts())

	if len(ghc.MilestoneCleared) != 1 {
		t.Errorf("expected ClearMilestone called once, got %+v", ghc.MilestoneCleared)
	}
}

func TestMilestone_TitleNotFound(t *testing.T) {
	sc, ghc := prContext("author")
	sc.Author = "maintainer"
	ghc.WriteAccess["elevran/stern/maintainer"] = true
	ghc.Milestones[5] = github.Milestone{Number: 5, Title: "v1.0"}

	reg := commands.Registry{"milestone": commands.NewMilestoneHandler}
	commands.Dispatch(context.Background(), sc, "/milestone v2.0", reg, ghc, milestoneOpts())

	if len(ghc.MilestoneSet) > 0 {
		t.Errorf("expected no SetMilestone call for unknown title, got %+v", ghc.MilestoneSet)
	}
	if len(ghc.Reactions) == 0 || ghc.Reactions[0].Content != "-1" {
		t.Errorf("expected -1 reaction, got %v", ghc.Reactions)
	}
	foundUsage := false
	for _, c := range ghc.Comments {
		if c.Number == 1 && c.Body == "milestone not found: v2.0" {
			foundUsage = true
		}
	}
	if !foundUsage {
		t.Errorf("expected comment 'milestone not found: v2.0', got %+v", ghc.Comments)
	}
}

func TestMilestone_MissingArg(t *testing.T) {
	sc, ghc := prContext("author")
	sc.Author = "maintainer"
	ghc.WriteAccess["elevran/stern/maintainer"] = true

	reg := commands.Registry{"milestone": commands.NewMilestoneHandler}
	commands.Dispatch(context.Background(), sc, "/milestone", reg, ghc, milestoneOpts())

	if len(ghc.MilestoneSet) > 0 || len(ghc.MilestoneCleared) > 0 {
		t.Errorf("expected no mutation when arg missing, got set=%v cleared=%v",
			ghc.MilestoneSet, ghc.MilestoneCleared)
	}
	if len(ghc.Reactions) == 0 || ghc.Reactions[0].Content != "-1" {
		t.Errorf("expected -1 reaction, got %v", ghc.Reactions)
	}
}

func TestMilestone_RequiresWriteAccess(t *testing.T) {
	sc, ghc := prContext("author")
	sc.Author = "outsider"
	ghc.WriteAccess["elevran/stern/outsider"] = false

	reg := commands.Registry{"milestone": commands.NewMilestoneHandler}
	commands.Dispatch(context.Background(), sc, "/milestone 7", reg, ghc, milestoneOpts())

	if len(ghc.MilestoneSet) > 0 {
		t.Errorf("expected no SetMilestone without write access, got %+v", ghc.MilestoneSet)
	}
	if len(ghc.Reactions) == 0 || ghc.Reactions[0].Content != "-1" {
		t.Errorf("expected -1 reaction, got %v", ghc.Reactions)
	}
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

	reg := commands.Registry{"milestone": commands.NewMilestoneHandler}
	commands.Dispatch(context.Background(), sc, "/milestone 3", reg, ghc, milestoneOpts())

	if len(ghc.MilestoneSet) != 1 || ghc.MilestoneSet[0].Number != 99 || ghc.MilestoneSet[0].MilestoneID != 3 {
		t.Errorf("expected SetMilestone on issue 99, got %+v", ghc.MilestoneSet)
	}
	if len(ghc.Reactions) == 0 || ghc.Reactions[0].Content != "+1" {
		t.Errorf("expected +1 reaction, got %v", ghc.Reactions)
	}
}
