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
	ghc := github.NewMockClient()
	opts := &config.Options{}
	sc := newSternContext()

	reg := commands.DefaultRegistry()
	commands.Dispatch(context.Background(), sc, "/ping", reg, ghc, opts)

	require.NotEmpty(t, ghc.Reactions, "expected +1 reaction from /ping")
	assert.Equal(t, "+1", ghc.Reactions[0].Content)
	assert.Equal(t, int64(123), ghc.Reactions[0].CommentID)
}

func TestDispatch_UnknownCommand(t *testing.T) {
	ghc := github.NewMockClient()
	opts := &config.Options{}
	sc := newSternContext()

	reg := commands.DefaultRegistry()
	commands.Dispatch(context.Background(), sc, "/hello there", reg, ghc, opts)

	assert.Empty(t, ghc.Reactions, "expected no reaction for unknown command")
	assert.Empty(t, ghc.Comments, "expected no comment for unknown command")
}

func TestDispatch_MultipleCommands(t *testing.T) {
	ghc := github.NewMockClient()
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
	assert.Equal(t, 2, pingCount, "expected 2 +1 reactions (one per /ping)")
}

func TestDispatch_NonCommandLinesIgnored(t *testing.T) {
	ghc := github.NewMockClient()
	opts := &config.Options{}
	sc := newSternContext()

	body := "This is a regular comment.\n/ping\nSome follow-up text."
	reg := commands.DefaultRegistry()
	commands.Dispatch(context.Background(), sc, body, reg, ghc, opts)

	require.Len(t, ghc.Reactions, 1)
	assert.Equal(t, "+1", ghc.Reactions[0].Content)
}

func TestDispatch_PermissionError(t *testing.T) {
	ghc := github.NewMockClient()
	opts := &config.Options{}
	sc := newSternContext()

	reg := commands.Registry{
		"deny": {Factory: newDenyHandler},
	}
	commands.Dispatch(context.Background(), sc, "/deny", reg, ghc, opts)

	require.NotEmpty(t, ghc.Reactions)
	assert.Equal(t, "-1", ghc.Reactions[0].Content)
	assert.NotEmpty(t, ghc.Comments, "expected error comment for permission error")
}

func TestDispatch_InternalError(t *testing.T) {
	ghc := github.NewMockClient()
	opts := &config.Options{}
	sc := newSternContext()

	reg := commands.Registry{
		"fail": {Factory: newFailHandler},
	}
	commands.Dispatch(context.Background(), sc, "/fail", reg, ghc, opts)

	require.NotEmpty(t, ghc.Reactions)
	assert.Equal(t, "confused", ghc.Reactions[0].Content)
	assert.NotEmpty(t, ghc.Comments, "expected error comment for internal error")
}

func TestDispatch_PluginNotEnabled(t *testing.T) {
	ghc := github.NewMockClient()
	opts := &config.Options{Plugins: []string{"lgtm"}}
	sc := newSternContext()

	called := false
	reg := commands.Registry{
		"approve": {Factory: func(_ *event.Context, _ github.Client, _ *config.Options) commands.Handler {
			return &spyHandler{onHandle: func() error { called = true; return nil }}
		}},
	}
	commands.Dispatch(context.Background(), sc, "/approve", reg, ghc, opts)

	assert.False(t, called, "handler should not be invoked when its plugin is not enabled")
	assert.Empty(t, ghc.Reactions, "expected no reaction for disabled plugin")
	assert.Empty(t, ghc.Comments, "expected no comment for disabled plugin")
}

func TestDispatch_BuiltinIgnoresPlugins(t *testing.T) {
	ghc := github.NewMockClient()
	opts := &config.Options{Plugins: []string{"lgtm"}}
	sc := newSternContext()

	reg := commands.DefaultRegistry()
	commands.Dispatch(context.Background(), sc, "/ping", reg, ghc, opts)

	require.NotEmpty(t, ghc.Reactions)
	assert.Equal(t, "+1", ghc.Reactions[0].Content)
}

// spyHandler records whether Handle was called.
type spyHandler struct {
	onHandle func() error
}

func (h *spyHandler) Pre(_ context.Context, _ *event.Context, _ []string) error { return nil }
func (h *spyHandler) Handle(_ context.Context, _ *event.Context, _ []string) error {
	return h.onHandle()
}
func (h *spyHandler) Post(_ context.Context, _ *event.Context, _ []string, _ error) error {
	return nil
}

// denyHandler always returns a permission error from Pre.
type denyHandler struct{}

func newDenyHandler(_ *event.Context, _ github.Client, _ *config.Options) commands.Handler {
	return denyHandler{}
}

func (denyHandler) Pre(_ context.Context, _ *event.Context, _ []string) error {
	return commands.PermissionError("you are not allowed to do that")
}
func (denyHandler) Handle(_ context.Context, _ *event.Context, _ []string) error { return nil }
func (denyHandler) Post(_ context.Context, _ *event.Context, _ []string, _ error) error {
	return nil
}

// failHandler always returns an internal error from Handle.
type failHandler struct{}

func newFailHandler(_ *event.Context, _ github.Client, _ *config.Options) commands.Handler {
	return failHandler{}
}

func (failHandler) Pre(_ context.Context, _ *event.Context, _ []string) error { return nil }
func (failHandler) Handle(_ context.Context, _ *event.Context, _ []string) error {
	return context.DeadlineExceeded
}
func (failHandler) Post(_ context.Context, _ *event.Context, _ []string, _ error) error {
	return nil
}
