package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/elevran/stern/internal/commands"
	"github.com/elevran/stern/internal/event"
)

func newSlashCommandCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "slash-command",
		Short: "Process slash commands from an issue_comment event",
		RunE:  runSlashCommand,
	}
}

func runSlashCommand(_ *cobra.Command, _ []string) error {
	evt, err := event.ParseCommentEvent()
	if err != nil {
		return fmt.Errorf("parsing event: %w", err)
	}

	// Bot guard: skip comments from bot accounts to prevent loops.
	author := ""
	if evt.Comment != nil && evt.Comment.User != nil {
		author = evt.Comment.User.GetLogin()
	}
	if strings.HasSuffix(author, "[bot]") || author == globalOpts.BotLogin {
		log.WithField("author", author).Info("skipping: comment from bot")
		return nil
	}

	ghc, err := buildClient()
	if err != nil {
		return fmt.Errorf("building GitHub client: %w", err)
	}

	org, repo, err := event.OrgRepoFromEnv()
	if err != nil {
		return err
	}

	sc := event.ContextFromComment(evt, org, repo, globalOpts.BotLogin)

	// Hydrate PR data when the comment is on a pull request.
	if evt.Issue != nil && evt.Issue.IsPullRequest() {
		pr, err := ghc.GetPullRequest(context.Background(), org, repo, sc.IssueNumber)
		if err != nil {
			log.WithError(err).Warn("failed to hydrate PR")
		} else {
			sc.PR = pr
		}
	}

	body := ""
	if evt.Comment != nil {
		body = evt.Comment.GetBody()
	}

	reg := commands.DefaultRegistry()
	commands.Dispatch(context.Background(), sc, body, reg, ghc, globalOpts)
	return nil
}
