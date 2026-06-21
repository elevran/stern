package main

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/elevran/stern/internal/config"
	"github.com/elevran/stern/internal/lifecycle"
)

func newLifecycleCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "lifecycle",
		Short: "Run scheduled lifecycle sweep over open issues and pull requests",
		RunE:  runLifecycle,
	}
}

func runLifecycle(_ *cobra.Command, _ []string) error {
	if !globalOpts.Lifecycle.Enabled {
		log.Info("lifecycle: plugin not enabled, nothing to do")
		return nil
	}
	ghc, err := buildClient()
	if err != nil {
		return fmt.Errorf("building github client: %w", err)
	}
	org, repo := config.OrgRepoFromGitHubRepository(globalOpts.Org, globalOpts.Repo)
	if org == "" || repo == "" {
		return fmt.Errorf("lifecycle: org/repo not set (provide via config or GITHUB_REPOSITORY)")
	}
	return lifecycle.Sweep(context.Background(), ghc, org, repo, globalOpts, time.Now())
}
