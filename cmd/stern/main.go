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
	"github.com/elevran/stern/internal/pr"
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
	opts, err := loadConfig()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
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

	return pr.HandlePREvent(context.Background(), ghc, org, repo, evt, opts)
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
	var (
		output string
		org    string
		repo   string
		force  bool
	)
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Generate a commented stern.yaml template",
		RunE: func(cmd *cobra.Command, args []string) error {
			org, repo = config.OrgRepoFromGitHubRepository(org, repo)
			if org == "" {
				org = "YOUR_ORG"
			}
			if repo == "" {
				repo = "YOUR_REPO"
			}

			data, err := config.Generate(org, repo)
			if err != nil {
				return err
			}

			if output == "-" {
				_, err = os.Stdout.Write(data)
				return err
			}

			if !force {
				if _, err := os.Stat(output); err == nil {
					return fmt.Errorf("%s already exists; use --force to overwrite", output)
				}
			}

			if err := os.MkdirAll(dirOf(output), 0o755); err != nil {
				return err
			}
			if err := os.WriteFile(output, data, 0o644); err != nil {
				return err
			}

			fmt.Printf("Wrote %s\n\n", output)
			fmt.Println("Next steps:")
			fmt.Printf("  1. Review and edit %s\n", output)
			fmt.Printf("  2. stern config check --config %s\n", output)
			fmt.Printf("  3. stern config sync-labels --dry-run --config %s\n", output)
			fmt.Printf("  4. stern config sync-labels --config %s\n", output)
			return nil
		},
	}
	cmd.Flags().StringVar(&output, "output", ".github/stern.yaml", "output file path (- for stdout)")
	cmd.Flags().StringVar(&org, "org", "", "GitHub organization")
	cmd.Flags().StringVar(&repo, "repo", "", "GitHub repository")
	cmd.Flags().BoolVar(&force, "force", false, "overwrite existing file")
	return cmd
}

func newConfigCheckCmd() *cobra.Command {
	c := &cobra.Command{
		Use:          "check",
		Short:        "Validate stern.yaml and report all issues",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			opts, err := loadConfig()
			if err != nil {
				return err
			}
			issues := opts.Validate()
			if len(issues) == 0 {
				fmt.Printf("%s — No issues found\n", configPath)
				return nil
			}
			fmt.Printf("%s — %d issue(s) found\n\n", configPath, len(issues))
			hasError := false
			for _, e := range issues {
				s := e.Error()
				fmt.Printf("  %s\n", s)
				if isErrorLevel(s) {
					hasError = true
				}
			}
			fmt.Println()
			if hasError {
				return fmt.Errorf("exit code 1")
			}
			return nil
		},
	}
	return c
}

func isErrorLevel(s string) bool {
	return len(s) >= 5 && s[:5] == "ERROR"
}

func newConfigSyncLabelsCmd() *cobra.Command {
	var (
		syncDryRun bool
		prune      bool
		yes        bool
	)
	cmd := &cobra.Command{
		Use:          "sync-labels",
		Short:        "Reconcile repo labels with label_definitions in stern.yaml",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			opts, err := loadConfig()
			if err != nil {
				return err
			}
			if opts.Org == "" || opts.Repo == "" {
				return fmt.Errorf("org and repo must be set in stern.yaml or --config")
			}

			ghc, err := buildClient()
			if err != nil {
				return err
			}

			ctx := context.Background()
			current, err := ghc.ListRepoLabels(ctx, opts.Org, opts.Repo)
			if err != nil {
				return fmt.Errorf("listing repo labels: %w", err)
			}

			diffs := config.DiffLabels(opts.LabelDefinitions, current)

			counts := map[string]int{}
			for _, d := range diffs {
				switch d.Action {
				case config.LabelCreate:
					counts["create"]++
				case config.LabelUpdate:
					counts["update"]++
				case config.LabelOK:
					counts["ok"]++
				case config.LabelExtra:
					counts["extra"]++
				}
			}
			fmt.Printf("Label plan for %s/%s:\n", opts.Org, opts.Repo)
			config.PrintLabelPlan(os.Stdout, diffs)
			fmt.Printf("\nSummary: %d create, %d update, %d ok, %d extra\n",
				counts["create"], counts["update"], counts["ok"], counts["extra"])

			if syncDryRun {
				fmt.Println("\n[dry-run] No changes applied.")
				return nil
			}

			if counts["extra"] > 0 && prune && !yes {
				fmt.Printf("\nDelete %d extra label(s)? [y/N] ", counts["extra"])
				var resp string
				fmt.Scanln(&resp)
				if resp != "y" && resp != "Y" {
					fmt.Println("Aborted.")
					return nil
				}
			}

			if err := config.ApplyLabelDiffs(ctx, ghc, opts.Org, opts.Repo, diffs, prune); err != nil {
				return err
			}
			fmt.Println("\nDone.")
			return nil
		},
	}
	cmd.Flags().BoolVar(&syncDryRun, "dry-run", false, "print plan without making changes")
	cmd.Flags().BoolVar(&prune, "prune", false, "delete labels not in label_definitions")
	cmd.Flags().BoolVar(&yes, "yes", false, "skip interactive confirmation when pruning")
	return cmd
}

func dirOf(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' {
			return path[:i]
		}
	}
	return "."
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
