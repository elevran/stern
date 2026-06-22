package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/elevran/stern/internal/config"
)

var (
	errorColor = color.New(color.FgRed)
	warnColor  = color.New(color.FgYellow)
)

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
		// Override root PersistentPreRunE — config init runs before a config file exists.
		PersistentPreRunE: func(_ *cobra.Command, _ []string) error {
			if os.Getenv("STERN_DRY_RUN") == "true" {
				dryRun = true
			}
			return nil
		},
		RunE: func(_ *cobra.Command, _ []string) error {
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

			if err := os.MkdirAll(filepath.Dir(output), 0o755); err != nil { // #nosec G301 -- config dir, not sensitive
				return err
			}
			if err := os.WriteFile(output, data, 0o644); err != nil { // #nosec G306 -- generated config, not sensitive
				return err
			}

			fmt.Printf("Wrote %s\n\n", output)
			fmt.Println("Next steps:")
			fmt.Printf("  1. Review and edit %s\n", output)
			fmt.Printf("  2. stern config check --config %s\n", output)
			fmt.Printf("  3. stern config sync-labels --check --config %s\n", output)
			fmt.Printf("  4. stern config sync-labels --dry-run --config %s\n", output)
			fmt.Printf("  5. stern config sync-labels --config %s\n", output)
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
	return &cobra.Command{
		Use:           "check",
		Short:         "Validate stern.yaml and report all issues",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			out := cmd.OutOrStdout()
			issues := globalOpts.Validate()
			if len(issues) == 0 {
				_, _ = fmt.Fprintf(out, "%s — No issues found\n", configPath)
				return nil
			}
			_, _ = fmt.Fprintf(out, "%s — %d issue(s) found\n\n", configPath, len(issues))
			hasError := false
			for _, e := range issues {
				switch e.Level {
				case "ERROR":
					_, _ = errorColor.Fprintf(out, "  ERROR  %s: %s\n", e.Field, e.Message)
					hasError = true
				case "WARN":
					_, _ = warnColor.Fprintf(out, "  WARN   %s: %s\n", e.Field, e.Message)
				default:
					_, _ = fmt.Fprintf(out, "  %s\n", e.Error())
				}
			}
			_, _ = fmt.Fprintln(out)
			if hasError {
				// Use os.Exit(1) rather than returning an error so Cobra does
				// not print its own "Error: ..." line after our validation output
				// (#18). SilenceErrors also suppresses that line, but exiting
				// directly avoids the RunE error path entirely.
				os.Exit(1)
			}
			return nil
		},
	}
}

func newConfigSyncLabelsCmd() *cobra.Command {
	var (
		syncDryRun bool
		check      bool
		prune      bool
		yes        bool
	)
	cmd := &cobra.Command{
		Use:          "sync-labels",
		Short:        "Reconcile repo labels with label_definitions in stern.yaml",
		SilenceUsage: true,
		RunE: func(_ *cobra.Command, _ []string) error {
			if globalOpts.Org == "" || globalOpts.Repo == "" {
				return fmt.Errorf("org and repo must be set in stern.yaml or --config")
			}

			ghc, err := buildClient()
			if err != nil {
				return err
			}

			ctx := context.Background()
			current, err := ghc.ListRepoLabels(ctx, globalOpts.Org, globalOpts.Repo)
			if err != nil {
				return fmt.Errorf("listing repo labels: %w", err)
			}

			plan := config.DiffLabels(globalOpts.LabelDefinitions, current)

			fmt.Printf("Label plan for %s/%s:\n", globalOpts.Org, globalOpts.Repo)
			plan.Print(os.Stdout)
			fmt.Printf("\nSummary: %d create, %d update, %d ok, %d extra\n",
				len(plan.Creates), len(plan.Updates), len(plan.Unchanged), len(plan.Extras))

			// --check is for CI gates: print plan, exit 1 on drift, 0 otherwise.
			// Implies --dry-run: never applies changes, even with --prune.
			if check {
				if plan.HasDrift() {
					fmt.Fprintln(os.Stderr, "\nDrift detected: repo labels do not match label_definitions.")
					// Exit directly so Cobra does not print "Error: ..." after our
					// output, matching the pattern used by `stern config check` (#18).
					os.Exit(1)
				}
				fmt.Println("\n[check] No drift detected.")
				return nil
			}

			if syncDryRun {
				fmt.Println("\n[dry-run] No changes applied.")
				return nil
			}

			if len(plan.Extras) > 0 && prune && !yes {
				fmt.Printf("\nDelete %d extra label(s)? [y/N] ", len(plan.Extras))
				var resp string
				_, _ = fmt.Scanln(&resp)
				if resp != "y" && resp != "Y" {
					fmt.Println("Aborted.")
					return nil
				}
			}

			if err := plan.Apply(ctx, ghc, globalOpts.Org, globalOpts.Repo, prune); err != nil {
				return err
			}
			fmt.Println("\nDone.")
			return nil
		},
	}
	cmd.Flags().BoolVar(&syncDryRun, "dry-run", false, "print plan without making changes")
	cmd.Flags().BoolVar(&check, "check", false, "print plan and exit non-zero if drift is detected (implies --dry-run; for CI gates)")
	cmd.Flags().BoolVar(&prune, "prune", false, "delete labels not in label_definitions")
	cmd.Flags().BoolVar(&yes, "yes", false, "skip interactive confirmation when pruning")
	return cmd
}
