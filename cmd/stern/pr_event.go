package main

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/elevran/stern/internal/event"
	"github.com/elevran/stern/internal/pr"
)

func newPREventCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "pr-event",
		Short: "Process a pull_request_target event",
		RunE:  runPREvent,
	}
}

func runPREvent(_ *cobra.Command, _ []string) error {
	evt, err := event.ParsePREvent()
	if err != nil {
		return fmt.Errorf("parsing event: %w", err)
	}

	ghc, err := buildClient()
	if err != nil {
		return fmt.Errorf("building GitHub client: %w", err)
	}

	org, repo, err := event.OrgRepoFromEnv()
	if err != nil {
		return err
	}

	handler := pr.NewPREventHandler(ghc, globalOpts)
	return handler.Handle(context.Background(), org, repo, evt)
}
