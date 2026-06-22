package main

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/elevran/stern/internal/event"
	"github.com/elevran/stern/internal/owners"
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
	// OWNERS cache: same wiring as runSlashCommand. The pr-event job
	// triggers review_assignment which calls owners.LoadForPaths; without
	// this, the actions/cache step in the workflow would warm a file
	// the binary never reads.
	if p := os.Getenv("OWNERS_CACHE_FILE"); p != "" {
		cache, err := owners.LoadCacheFile(p)
		if err != nil {
			log.WithError(err).WithField("path", p).Warn("loading owners cache; continuing without cache")
		} else {
			owners.SetAmbientCache(cache)
			defer func() {
				if err := cache.Save(); err != nil {
					log.WithError(err).WithField("path", p).Warn("saving owners cache")
				}
			}()
		}
	}

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

	handler := pr.NewEventHandler(ghc, globalOpts)
	return handler.Handle(context.Background(), org, repo, evt)
}
