package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/elevran/stern/internal/commands"
	"github.com/elevran/stern/internal/config"
	"github.com/elevran/stern/internal/event"
	ghclient "github.com/elevran/stern/internal/github"
)

var (
	log        = logrus.New()
	configPath string
	dryRun     bool
)

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

var rootCmd = &cobra.Command{
	Use:   "stern",
	Short: "GitHub PR bot running via GitHub Actions",
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		if os.Getenv("STERN_DRY_RUN") == "true" {
			dryRun = true
		}
		if dryRun {
			log.Info("dry-run mode enabled")
		}
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

// --- slash-command ---

func newSlashCommandCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "slash-command",
		Short: "Process slash commands from an issue_comment event",
		RunE:  runSlashCommand,
	}
}

func runSlashCommand(cmd *cobra.Command, _ []string) error {
	opts, err := loadConfig()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	evt, err := event.ParseCommentEvent()
	if err != nil {
		return fmt.Errorf("parsing event: %w", err)
	}

	// Bot guard: skip comments from bot accounts to prevent loops.
	author := ""
	if evt.Comment != nil && evt.Comment.User != nil {
		author = evt.Comment.User.GetLogin()
	}
	if strings.HasSuffix(author, "[bot]") || author == opts.BotLogin {
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

	sc := event.ContextFromComment(evt, org, repo, opts.BotLogin)

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
	commands.Dispatch(context.Background(), sc, body, reg, ghc, opts)
	return nil
}

// --- pr-event ---

func newPREventCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "pr-event",
		Short: "Process a pull_request_target event",
		RunE:  runPREvent,
	}
}

func runPREvent(cmd *cobra.Command, _ []string) error {
	_, err := loadConfig()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}
	evt, err := event.ParsePREvent()
	if err != nil {
		return fmt.Errorf("parsing event: %w", err)
	}
	log.WithField("action", evt.GetAction()).Info("pr-event: no handlers registered yet")
	return nil
}

// --- issue-event ---

func newIssueEventCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "issue-event",
		Short: "Process an issues event",
		RunE:  runIssueEvent,
	}
}

func runIssueEvent(cmd *cobra.Command, _ []string) error {
	_, err := loadConfig()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}
	evt, err := event.ParseIssueEvent()
	if err != nil {
		return fmt.Errorf("parsing event: %w", err)
	}
	log.WithField("action", evt.GetAction()).Info("issue-event: no handlers registered yet")
	return nil
}

// --- merge-check ---

func newMergeCheckCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "merge-check",
		Short: "Process a check_suite event (optional, enable in workflow when ready)",
		RunE:  runMergeCheck,
	}
}

func runMergeCheck(cmd *cobra.Command, _ []string) error {
	log.Info("merge-check: not yet implemented")
	return nil
}

// --- lifecycle ---

func newLifecycleCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "lifecycle",
		Short: "Run scheduled lifecycle sweep (optional, enable in workflow when ready)",
		RunE:  runLifecycle,
	}
}

func runLifecycle(cmd *cobra.Command, _ []string) error {
	log.Info("lifecycle: not yet implemented")
	return nil
}

// --- config ---

func newConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Config management commands",
	}
	cmd.AddCommand(newConfigInitCmd())
	cmd.AddCommand(newConfigCheckCmd())
	cmd.AddCommand(newConfigSyncLabelsCmd())
	return cmd
}

func newConfigInitCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Generate a commented stern.yaml template (Task 0.4)",
		RunE: func(cmd *cobra.Command, args []string) error {
			log.Info("config init: not yet implemented (Task 0.4)")
			return nil
		},
	}
}

func newConfigCheckCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "check",
		Short: "Validate stern.yaml and report all issues (Task 0.5)",
		RunE: func(cmd *cobra.Command, args []string) error {
			opts, err := loadConfig()
			if err != nil {
				return err
			}
			errs := opts.Validate()
			if len(errs) == 0 {
				fmt.Println("No issues found")
				return nil
			}
			for _, e := range errs {
				fmt.Fprintf(os.Stderr, "ERROR  %v\n", e)
			}
			return fmt.Errorf("%d issue(s) found", len(errs))
		},
	}
}

func newConfigSyncLabelsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "sync-labels",
		Short: "Reconcile repo labels with label_definitions in stern.yaml (Task 0.6)",
		RunE: func(cmd *cobra.Command, args []string) error {
			log.Info("config sync-labels: not yet implemented (Task 0.6)")
			return nil
		},
	}
}

// --- helpers ---

func loadConfig() (*config.Options, error) {
	return config.LoadFromFile(configPath)
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
