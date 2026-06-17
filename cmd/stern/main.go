package main

import (
	"fmt"
	"os"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/elevran/stern/internal/config"
	ghclient "github.com/elevran/stern/internal/github"
)

var (
	log        = logrus.New()
	configPath string
	dryRun     bool
	globalOpts *config.Options
)

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

var rootCmd = &cobra.Command{
	Use:   "stern",
	Short: "GitHub PR bot running via GitHub Actions",
	PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
		if os.Getenv("STERN_DRY_RUN") == "true" {
			dryRun = true
		}
		if dryRun {
			log.Info("dry-run mode enabled")
		}
		opts, err := config.LoadFromFile(configPath)
		if err != nil {
			return fmt.Errorf("loading config: %w", err)
		}
		globalOpts = opts
		return nil
	},
}

func init() {
	rootCmd.PersistentFlags().StringVar(&configPath, "config", ".github/stern.yaml", "path to stern config file")
	rootCmd.PersistentFlags().BoolVar(&dryRun, "dry-run", false, "log mutations without executing them")
	rootCmd.AddCommand(newSlashCommandCmd())
	rootCmd.AddCommand(newPREventCmd())
	rootCmd.AddCommand(newIssueEventCmd())
	rootCmd.AddCommand(newMergeCheckCmd())
	rootCmd.AddCommand(newLifecycleCmd())
	rootCmd.AddCommand(newConfigCmd())
}

func buildClient() (ghclient.Client, error) {
	ghc, err := ghclient.NewFromEnv()
	if err != nil {
		return nil, err
	}
	if dryRun {
		return ghclient.NewDryRun(ghc, log), nil
	}
	return ghc, nil
}
