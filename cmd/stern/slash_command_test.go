package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	gh "github.com/google/go-github/v72/github"

	"github.com/elevran/stern/internal/config"
	"github.com/elevran/stern/internal/event"
	"github.com/elevran/stern/internal/github"
)

// writeEvent writes a JSON-encoded CommentEvent payload to a temporary file
// and returns its path. The event is also returned for direct inspection in
// tests that want to verify the parsed fields. GITHUB_EVENT_PATH is set to
// the temp file so ParseCommentEvent picks it up.
func writeEvent(t *testing.T, evt gh.IssueCommentEvent) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "event.json")
	data, err := json.Marshal(evt)
	if err != nil {
		t.Fatalf("marshal event: %v", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write event: %v", err)
	}
	t.Setenv("GITHUB_EVENT_PATH", path)
	return path
}

func botEvent(login string) gh.IssueCommentEvent {
	return gh.IssueCommentEvent{
		Action: gh.Ptr("created"),
		Issue: &gh.Issue{
			Number: gh.Ptr(1),
			User:   &gh.User{Login: gh.Ptr(login)},
		},
		Comment: &gh.IssueComment{
			ID:   gh.Ptr(int64(12345)),
			Body: gh.Ptr("/ping"),
			User: &gh.User{Login: gh.Ptr(login)},
		},
		Sender: &gh.User{Login: gh.Ptr(login)},
	}
}

func prCommentEvent(login string) gh.IssueCommentEvent {
	evt := botEvent(login)
	evt.Issue.PullRequestLinks = &gh.PullRequestLinks{
		URL: gh.Ptr("https://api.github.com/repos/elevran/stern/pulls/1"),
	}
	return evt
}

func TestRunSlashCommand_BotCommentSkipped(t *testing.T) {
	writeEvent(t, botEvent("github-actions[bot]"))
	t.Setenv("GITHUB_REPOSITORY", "elevran/stern")

	origOpts := globalOpts
	globalOpts = &config.Options{BotLogin: "github-actions[bot]"}
	defer func() { globalOpts = origOpts }()

	ghc := github.NewMockClient()
	if err := runSlashCommand(ghc); err != nil {
		t.Fatalf("runSlashCommand() error = %v", err)
	}
	if len(ghc.Reactions) != 0 {
		t.Errorf("bot comments should be skipped, got reactions: %v", ghc.Reactions)
	}
	if len(ghc.Comments) != 0 {
		t.Errorf("bot comments should be skipped, got comments: %v", ghc.Comments)
	}
}

func TestRunSlashCommand_NonBotDispatched(t *testing.T) {
	writeEvent(t, botEvent("alice"))
	t.Setenv("GITHUB_REPOSITORY", "elevran/stern")

	origOpts := globalOpts
	globalOpts = &config.Options{BotLogin: "github-actions[bot]"}
	defer func() { globalOpts = origOpts }()

	ghc := github.NewMockClient()
	if err := runSlashCommand(ghc); err != nil {
		t.Fatalf("runSlashCommand() error = %v", err)
	}
	if len(ghc.Reactions) == 0 {
		t.Fatal("expected /ping to be dispatched and produce a +1 reaction")
	}
	if ghc.Reactions[0].Content != "+1" {
		t.Errorf("first reaction = %q, want +1", ghc.Reactions[0].Content)
	}
}

func TestRunSlashCommand_PRHydration(t *testing.T) {
	writeEvent(t, prCommentEvent("alice"))
	t.Setenv("GITHUB_REPOSITORY", "elevran/stern")

	origOpts := globalOpts
	globalOpts = &config.Options{BotLogin: "github-actions[bot]"}
	defer func() { globalOpts = origOpts }()

	ghc := github.NewMockClient()
	ghc.PullRequests[1] = &github.PullRequest{
		Number:  1,
		Author:  "alice",
		Title:   "Test PR",
		HeadSHA: "abc",
		NodeID:  "node-1",
		Labels:  []string{},
	}

	// We exercise the hydration step directly here because the
	// dispatch path would consume sc.PR, and we want to assert the
	// hydrated value before any handler logic runs.
	sc := &event.Context{
		Org:         "elevran",
		Repo:        "stern",
		BotLogin:    "github-actions[bot]",
		IssueNumber: 1,
	}
	evt, err := event.ParseCommentEvent()
	if err != nil {
		t.Fatalf("ParseCommentEvent() error = %v", err)
	}
	if err := hydratePR(context.Background(), sc, evt, ghc); err != nil {
		t.Fatalf("hydratePR() error = %v", err)
	}
	if sc.PR == nil {
		t.Fatal("expected sc.PR to be hydrated")
	}
	if sc.PR.Number != 1 {
		t.Errorf("sc.PR.Number = %d, want 1", sc.PR.Number)
	}
	if sc.PR.Title != "Test PR" {
		t.Errorf("sc.PR.Title = %q, want %q", sc.PR.Title, "Test PR")
	}
}

func TestRunSlashCommand_PRHydration_NotPullRequest(t *testing.T) {
	writeEvent(t, botEvent("alice"))
	t.Setenv("GITHUB_REPOSITORY", "elevran/stern")

	origOpts := globalOpts
	globalOpts = &config.Options{BotLogin: "github-actions[bot]"}
	defer func() { globalOpts = origOpts }()

	ghc := github.NewMockClient()
	sc := &event.Context{
		Org:         "elevran",
		Repo:        "stern",
		BotLogin:    "github-actions[bot]",
		IssueNumber: 1,
	}
	evt, err := event.ParseCommentEvent()
	if err != nil {
		t.Fatalf("ParseCommentEvent() error = %v", err)
	}
	if err := hydratePR(context.Background(), sc, evt, ghc); err != nil {
		t.Fatalf("hydratePR() error = %v", err)
	}
	if sc.PR != nil {
		t.Errorf("expected sc.PR to remain nil for a non-PR issue, got %+v", sc.PR)
	}
}

func TestRunSlashCommand_PRHydration_GetPullRequestError(t *testing.T) {
	writeEvent(t, prCommentEvent("alice"))
	t.Setenv("GITHUB_REPOSITORY", "elevran/stern")

	origOpts := globalOpts
	globalOpts = &config.Options{BotLogin: "github-actions[bot]"}
	defer func() { globalOpts = origOpts }()

	ghc := github.NewMockClient()
	ghc.Errors["GetPullRequest"] = os.ErrNotExist

	sc := &event.Context{
		Org:         "elevran",
		Repo:        "stern",
		BotLogin:    "github-actions[bot]",
		IssueNumber: 1,
	}
	evt, err := event.ParseCommentEvent()
	if err != nil {
		t.Fatalf("ParseCommentEvent() error = %v", err)
	}
	// Hydration failures are logged but must not propagate; the comment
	// still dispatches with sc.PR == nil.
	if err := hydratePR(context.Background(), sc, evt, ghc); err != nil {
		t.Fatalf("hydratePR() error = %v, want nil", err)
	}
	if sc.PR != nil {
		t.Errorf("expected sc.PR to be nil after GetPullRequest error, got %+v", sc.PR)
	}
}
