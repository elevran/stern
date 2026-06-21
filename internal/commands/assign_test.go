package commands_test

import (
	"context"
	"errors"
	"testing"

	"github.com/elevran/stern/internal/commands"
	"github.com/elevran/stern/internal/config"
	"github.com/elevran/stern/internal/event"
	"github.com/elevran/stern/internal/github"
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

	if len(ghc.AssigneesAdded) != 1 {
		t.Fatalf("expected 1 AddAssignees call, got %d", len(ghc.AssigneesAdded))
	}
	if got := ghc.AssigneesAdded[0].Users; len(got) != 1 || got[0] != "alice" {
		t.Errorf("expected AddAssignees([alice]), got %v", got)
	}
	if len(ghc.Reactions) == 0 || ghc.Reactions[0].Content != "+1" {
		t.Errorf("expected +1 reaction, got %v", ghc.Reactions)
	}
}

func TestAssign_SelfAssign_NotOrgMember_Denied(t *testing.T) {
	sc, ghc := prContext("author")
	sc.Author = "alice"
	ghc.OrgMembers["elevran/alice"] = false

	commands.Dispatch(context.Background(), sc, "/assign", assignReg(), ghc, assignOpts())

	if len(ghc.AssigneesAdded) != 0 {
		t.Errorf("expected AddAssignees NOT called for non-org-member, got %d", len(ghc.AssigneesAdded))
	}
	if len(ghc.Reactions) == 0 || ghc.Reactions[0].Content != "-1" {
		t.Errorf("expected -1 reaction, got %v", ghc.Reactions)
	}
}

func TestAssign_OtherUser_RequiresWriteAccess(t *testing.T) {
	sc, ghc := prContext("author")
	sc.Author = "bob"
	ghc.WriteAccess["elevran/stern/bob"] = false

	commands.Dispatch(context.Background(), sc, "/assign @carol", assignReg(), ghc, assignOpts())

	if len(ghc.AssigneesAdded) != 0 {
		t.Errorf("expected AddAssignees NOT called for non-writer, got %d", len(ghc.AssigneesAdded))
	}
	if len(ghc.Reactions) == 0 || ghc.Reactions[0].Content != "-1" {
		t.Errorf("expected -1 reaction, got %v", ghc.Reactions)
	}
}

func TestAssign_OtherUser_WithWriteAccess(t *testing.T) {
	sc, ghc := prContext("author")
	sc.Author = "maintainer"
	ghc.WriteAccess["elevran/stern/maintainer"] = true

	commands.Dispatch(context.Background(), sc, "/assign @carol", assignReg(), ghc, assignOpts())

	if len(ghc.AssigneesAdded) != 1 {
		t.Fatalf("expected 1 AddAssignees call, got %d", len(ghc.AssigneesAdded))
	}
	if got := ghc.AssigneesAdded[0].Users; len(got) != 1 || got[0] != "carol" {
		t.Errorf("expected AddAssignees([carol]), got %v", got)
	}
}

func TestAssign_MultiUser(t *testing.T) {
	sc, ghc := prContext("author")
	sc.Author = "maintainer"
	ghc.WriteAccess["elevran/stern/maintainer"] = true

	commands.Dispatch(context.Background(), sc, "/assign @carol @dan", assignReg(), ghc, assignOpts())

	if len(ghc.AssigneesAdded) != 1 {
		t.Fatalf("expected 1 AddAssignees call, got %d", len(ghc.AssigneesAdded))
	}
	got := ghc.AssigneesAdded[0].Users
	if len(got) != 2 || got[0] != "carol" || got[1] != "dan" {
		t.Errorf("expected [carol dan], got %v", got)
	}
}

func TestAssign_StripsAtAndDeduplicates(t *testing.T) {
	sc, ghc := prContext("author")
	sc.Author = "maintainer"
	ghc.WriteAccess["elevran/stern/maintainer"] = true

	commands.Dispatch(context.Background(), sc, "/assign @carol @CAROL @dan", assignReg(), ghc, assignOpts())

	got := ghc.AssigneesAdded[0].Users
	if len(got) != 2 || got[0] != "carol" || got[1] != "dan" {
		t.Errorf("expected [carol dan] after dedupe+lowercase, got %v", got)
	}
}

func TestUnassign_Self(t *testing.T) {
	sc, ghc := prContext("author")
	sc.Author = "alice"
	ghc.OrgMembers["elevran/alice"] = true
	ghc.Assignees[1] = []string{"alice"}

	commands.Dispatch(context.Background(), sc, "/unassign", assignReg(), ghc, assignOpts())

	if len(ghc.AssigneesRemoved) != 1 {
		t.Fatalf("expected 1 RemoveAssignees call, got %d", len(ghc.AssigneesRemoved))
	}
	if got := ghc.AssigneesRemoved[0].Users; len(got) != 1 || got[0] != "alice" {
		t.Errorf("expected RemoveAssignees([alice]), got %v", got)
	}
}

func TestUnassign_Other(t *testing.T) {
	sc, ghc := prContext("author")
	sc.Author = "maintainer"
	ghc.WriteAccess["elevran/stern/maintainer"] = true

	commands.Dispatch(context.Background(), sc, "/unassign @carol", assignReg(), ghc, assignOpts())

	if len(ghc.AssigneesRemoved) != 1 {
		t.Fatalf("expected 1 RemoveAssignees call, got %d", len(ghc.AssigneesRemoved))
	}
	if got := ghc.AssigneesRemoved[0].Users; len(got) != 1 || got[0] != "carol" {
		t.Errorf("expected RemoveAssignees([carol]), got %v", got)
	}
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

	if len(ghc.AssigneesAdded) != 0 {
		t.Errorf("expected AddAssignees NOT called when not on a PR")
	}
	if len(ghc.Reactions) == 0 || ghc.Reactions[0].Content != "-1" {
		t.Errorf("expected -1 reaction, got %v", ghc.Reactions)
	}
}

func TestAssign_HandleError_SuppressesPost(t *testing.T) {
	sc, ghc := prContext("author")
	sc.Author = "alice"
	ghc.OrgMembers["elevran/alice"] = true
	ghc.Errors["AddAssignees"] = errors.New("boom")

	commands.Dispatch(context.Background(), sc, "/assign", assignReg(), ghc, assignOpts())

	if len(ghc.AutoMergeEnabled) > 0 || len(ghc.AutoMergeDisabled) > 0 {
		t.Errorf("expected Post NOT to run when Handle errors, got enabled=%v disabled=%v",
			ghc.AutoMergeEnabled, ghc.AutoMergeDisabled)
	}
	if len(ghc.Reactions) == 0 || ghc.Reactions[0].Content != "confused" {
		t.Errorf("expected confused reaction on internal error, got %v", ghc.Reactions)
	}
}
