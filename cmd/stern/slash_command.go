package main

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/elevran/stern/internal/commands"
	"github.com/elevran/stern/internal/event"
	"github.com/elevran/stern/internal/github"
)

func newSlashCommandCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "slash-command",
		Short: "Process slash commands from an issue_comment event",
		RunE: func(cmd *cobra.Command, args []string) error {
			ghc, err := buildClient()
			if err != nil {
				return fmt.Errorf("building GitHub client: %w", err)
			}
			return runSlashCommand(ghc)
		},
	}
}

// runSlashCommand processes the issue_comment event referenced by the
// GITHUB_EVENT_PATH env var. ghc is the GitHub client to use for any
// follow-up calls (PR hydration, dispatch mutations). It is injected so
// tests can pass a mock client.
func runSlashCommand(ghc github.Client) error {
	evt, err := event.ParseCommentEvent()
	if err != nil {
		return fmt.Errorf("parsing event: %w", err)
	}

	// Bot guard: skip comments from bot accounts to prevent loops.
	author := ""
	if evt.Comment != nil && evt.Comment.User != nil {
		author = evt.Comment.User.GetLogin()
	}
	if event.IsBot(author, globalOpts.BotLogin) {
		log.WithField("author", author).Info("skipping: comment from bot")
		return nil
	}

	org, repo, err := event.OrgRepoFromEnv()
	if err != nil {
		return err
	}

	sc := event.ContextFromComment(evt, org, repo, globalOpts.BotLogin)
	if err := hydratePR(context.Background(), sc, evt, ghc); err != nil {
		return err
	}

	body := ""
	if evt.Comment != nil {
		body = evt.Comment.GetBody()
	}

	reg := commands.DefaultRegistry()
	commands.Dispatch(context.Background(), sc, body, reg, ghc, globalOpts)
	return nil
}

// hydratePR populates sc.PR with the pull request data for the issue
// referenced by the comment, but only when the comment targets a pull
// request. Failures are logged and sc.PR is left nil; the function
// returns no error so that a hydration failure does not block the
// command dispatch.
func hydratePR(ctx context.Context, sc *event.Context, evt *event.CommentEvent, ghc github.Client) error {
	if evt == nil || evt.Issue == nil || !evt.Issue.IsPullRequest() {
		return nil
	}
	pullReq, err := ghc.GetPullRequest(ctx, sc.Org, sc.Repo, sc.IssueNumber)
	if err != nil {
		log.WithError(err).Warn("failed to hydrate PR")
		return nil
	}
	sc.PR = &pullReq
	return nil
}
