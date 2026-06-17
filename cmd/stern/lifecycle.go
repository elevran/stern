package main

import "github.com/spf13/cobra"

func newLifecycleCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "lifecycle",
		Short: "Run scheduled lifecycle sweep (optional, enable in workflow when ready)",
		RunE:  runLifecycle,
	}
}

func runLifecycle(_ *cobra.Command, _ []string) error {
	log.Info("lifecycle: not yet implemented")
	return nil
}
