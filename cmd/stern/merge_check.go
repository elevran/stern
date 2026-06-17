package main

import "github.com/spf13/cobra"

func newMergeCheckCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "merge-check",
		Short: "Process a check_suite event (optional, enable in workflow when ready)",
		RunE:  runMergeCheck,
	}
}

func runMergeCheck(_ *cobra.Command, _ []string) error {
	log.Info("merge-check: not yet implemented")
	return nil
}
