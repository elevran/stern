package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/elevran/stern/internal/event"
)

func newIssueEventCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "issue-event",
		Short: "Process an issues event",
		RunE:  runIssueEvent,
	}
}

func runIssueEvent(_ *cobra.Command, _ []string) error {
	evt, err := event.ParseIssueEvent()
	if err != nil {
		return fmt.Errorf("parsing event: %w", err)
	}
	log.WithField("action", evt.GetAction()).Info("issue-event: no handlers registered yet")
	return nil
}
