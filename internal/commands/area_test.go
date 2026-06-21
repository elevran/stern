package commands_test

import (
	"context"
	"errors"
	"testing"

	"github.com/elevran/stern/internal/commands"
	"github.com/elevran/stern/internal/config"
)

func areaOpts() *config.Options {
	return &config.Options{
		Area: config.AreaOptions{
			Values: []string{"api", "cli", "ui"},
		},
	}
}

func TestArea_AddsLabel(t *testing.T) {
	sc, ghc := prContext("author")
	reg := commands.Registry{"area": commands.NewAreaHandler}
	commands.Dispatch(context.Background(), sc, "/area api", reg, ghc, areaOpts())

	if !ghc.IssueLabels[1]["area/api"] {
		t.Error("expected area/api label to be added")
	}
	if len(ghc.Reactions) == 0 || ghc.Reactions[0].Content != "+1" {
		t.Errorf("expected +1 reaction after successful /area, got %v", ghc.Reactions)
	}
}

func TestArea_AllowsMultipleValues(t *testing.T) {
	sc, ghc := prContext("author")
	sc.PR.Labels = []string{"area/api"}
	ghc.IssueLabels[1] = map[string]bool{"area/api": true}

	reg := commands.Registry{"area": commands.NewAreaHandler}
	commands.Dispatch(context.Background(), sc, "/area cli", reg, ghc, areaOpts())

	if !ghc.IssueLabels[1]["area/api"] {
		t.Error("expected pre-existing area/api label to remain (no mutual exclusion)")
	}
	if !ghc.IssueLabels[1]["area/cli"] {
		t.Error("expected new area/cli label to be added")
	}
}

func TestArea_InvalidValue(t *testing.T) {
	sc, ghc := prContext("author")
	reg := commands.Registry{"area": commands.NewAreaHandler}
	commands.Dispatch(context.Background(), sc, "/area unknown", reg, ghc, areaOpts())

	if len(ghc.IssueLabels[1]) > 0 {
		t.Errorf("expected no labels added for invalid area, got %v", ghc.IssueLabels[1])
	}
	if len(ghc.Reactions) == 0 || ghc.Reactions[0].Content != "-1" {
		t.Errorf("expected -1 reaction for invalid area, got %v", ghc.Reactions)
	}
}

func TestArea_NoArg(t *testing.T) {
	sc, ghc := prContext("author")
	reg := commands.Registry{"area": commands.NewAreaHandler}
	commands.Dispatch(context.Background(), sc, "/area", reg, ghc, areaOpts())

	if len(ghc.Reactions) == 0 || ghc.Reactions[0].Content != "-1" {
		t.Errorf("expected -1 reaction for /area with no arg, got %v", ghc.Reactions)
	}
}

func TestArea_NotOnPR(t *testing.T) {
	sc, ghc := prContext("author")
	sc.PR = nil

	reg := commands.Registry{"area": commands.NewAreaHandler}
	commands.Dispatch(context.Background(), sc, "/area api", reg, ghc, areaOpts())

	if len(ghc.Reactions) == 0 || ghc.Reactions[0].Content != "-1" {
		t.Errorf("expected -1 for /area on non-PR, got %v", ghc.Reactions)
	}
}

func TestArea_HandleError_SuppressesPost(t *testing.T) {
	sc, ghc := prContext("author")
	ghc.Errors["AddLabels"] = errors.New("boom")

	reg := commands.Registry{"area": commands.NewAreaHandler}
	commands.Dispatch(context.Background(), sc, "/area api", reg, ghc, areaOpts())

	if len(ghc.AutoMergeEnabled) > 0 || len(ghc.AutoMergeDisabled) > 0 {
		t.Errorf("expected Post NOT to run when Handle errors, got enabled=%v disabled=%v",
			ghc.AutoMergeEnabled, ghc.AutoMergeDisabled)
	}
	if len(ghc.Reactions) == 0 || ghc.Reactions[0].Content != "confused" {
		t.Errorf("expected confused reaction on internal error, got %v", ghc.Reactions)
	}
}
