package commands_test

import (
	"context"
	"testing"

	"github.com/elevran/stern/internal/commands"
	"github.com/elevran/stern/internal/config"
	"github.com/elevran/stern/internal/event"
	ghclient "github.com/elevran/stern/internal/github"
)

func newSternContext() *event.Context {
	return &event.Context{
		Org:         "elevran",
		Repo:        "stern",
		CommentID:   123,
		Author:      "testuser",
		IssueNumber: 1,
	}
}

func TestDispatch_Ping(t *testing.T) {
	ghc := ghclient.NewMockClient()
	opts := &config.Options{}
	sc := newSternContext()

	reg := commands.DefaultRegistry()
	commands.Dispatch(context.Background(), sc, "/ping", reg, ghc, opts)

	if len(ghc.Reactions) == 0 {
		t.Fatal("expected +1 reaction from /ping")
	}
	if ghc.Reactions[0].Content != "+1" {
		t.Errorf("reaction = %q, want +1", ghc.Reactions[0].Content)
	}
	if ghc.Reactions[0].CommentID != 123 {
		t.Errorf("reaction on comment %d, want 123", ghc.Reactions[0].CommentID)
	}
}

func TestDispatch_UnknownCommand(t *testing.T) {
	ghc := ghclient.NewMockClient()
	opts := &config.Options{}
	sc := newSternContext()

	reg := commands.DefaultRegistry()
	commands.Dispatch(context.Background(), sc, "/hello there", reg, ghc, opts)

	if len(ghc.Reactions) != 0 {
		t.Errorf("expected no reaction for unknown command, got %d", len(ghc.Reactions))
	}
	if len(ghc.Comments) != 0 {
		t.Errorf("expected no comment for unknown command, got %d", len(ghc.Comments))
	}
}

func TestDispatch_MultipleCommands(t *testing.T) {
	ghc := ghclient.NewMockClient()
	opts := &config.Options{}
	sc := newSternContext()

	body := "/ping\n/hello there\n/ping again"
	reg := commands.DefaultRegistry()
	commands.Dispatch(context.Background(), sc, body, reg, ghc, opts)

	// Two /ping calls both succeed; /hello is unknown (no reaction).
	// GitHub reactions are idempotent, mock records each attempt.
	pingCount := 0
	for _, r := range ghc.Reactions {
		if r.Content == "+1" {
			pingCount++
		}
	}
	if pingCount != 2 {
		t.Errorf("expected 2 +1 reactions (one per /ping), got %d", pingCount)
	}
}

func TestDispatch_NonCommandLinesIgnored(t *testing.T) {
	ghc := ghclient.NewMockClient()
	opts := &config.Options{}
	sc := newSternContext()

	body := "This is a regular comment.\n/ping\nSome follow-up text."
	reg := commands.DefaultRegistry()
	commands.Dispatch(context.Background(), sc, body, reg, ghc, opts)

	if len(ghc.Reactions) != 1 || ghc.Reactions[0].Content != "+1" {
		t.Errorf("expected exactly 1 +1 reaction, got %v", ghc.Reactions)
	}
}

func TestDispatch_PermissionError(t *testing.T) {
	ghc := ghclient.NewMockClient()
	opts := &config.Options{}
	sc := newSternContext()

	reg := commands.Registry{
		"deny": denyHandler{},
	}
	commands.Dispatch(context.Background(), sc, "/deny", reg, ghc, opts)

	if len(ghc.Reactions) == 0 || ghc.Reactions[0].Content != "-1" {
		t.Errorf("expected -1 reaction for permission error, got %v", ghc.Reactions)
	}
	if len(ghc.Comments) == 0 {
		t.Error("expected error comment for permission error")
	}
}

func TestDispatch_InternalError(t *testing.T) {
	ghc := ghclient.NewMockClient()
	opts := &config.Options{}
	sc := newSternContext()

	reg := commands.Registry{
		"fail": failHandler{},
	}
	commands.Dispatch(context.Background(), sc, "/fail", reg, ghc, opts)

	if len(ghc.Reactions) == 0 || ghc.Reactions[0].Content != "confused" {
		t.Errorf("expected confused reaction for internal error, got %v", ghc.Reactions)
	}
	if len(ghc.Comments) == 0 {
		t.Error("expected error comment for internal error")
	}
}

// denyHandler always returns a permission error.
type denyHandler struct{}

func (h denyHandler) Handle(ctx context.Context, sc *event.Context, args []string, ghc ghclient.Client, opts *config.Options) error {
	return commands.PermissionError("you are not allowed to do that")
}

// failHandler always returns an internal error.
type failHandler struct{}

func (h failHandler) Handle(ctx context.Context, sc *event.Context, args []string, ghc ghclient.Client, opts *config.Options) error {
	return context.DeadlineExceeded
}
