package commands

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"

	"github.com/elevran/stern/internal/config"
	"github.com/elevran/stern/internal/event"
	"github.com/elevran/stern/internal/github"
)

// cherryPickClient is the minimum Client surface CherryPickHandler uses.
type cherryPickClient interface {
	github.PermissionsClient
	github.PRCreatorClient
	github.CommentsClient
}

// CherryPickHandler handles /cherry-pick <branch>.
type CherryPickHandler struct {
	ghc            cherryPickClient
	opts           *config.Options
	allowedPattern *regexp.Regexp
}

// NewCherryPickHandler constructs a CherryPickHandler with all dependencies
// injected. When allowed_branch_pattern is unset, the handler rejects every
// target branch (matches the config validation: cherry-pick plugin should
// not be enabled without an explicit pattern).
func NewCherryPickHandler(_ *event.Context, ghc github.Client, opts *config.Options) Handler {
	// Pattern that never matches when allowed_branch_pattern is empty.
	// \b\B is a word boundary followed by a non-word boundary — no string
	// can satisfy both, so every target is rejected.
	pattern := regexp.MustCompile(`\b\B`)
	if opts.CherryPick.AllowedBranchPattern != "" {
		pattern = regexp.MustCompile(opts.CherryPick.AllowedBranchPattern)
	}
	return &CherryPickHandler{
		ghc:            ghc,
		opts:           opts,
		allowedPattern: pattern,
	}
}

// gitExec runs git with the given args. It is a package-level var so tests
// can replace it with a stub. Stdout/stderr are forwarded so failures are
// visible in the Actions log.
var gitExec = func(args ...string) error {
	cmd := exec.Command("git", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// SetGitExecForTest swaps gitExec for a stub and returns the previous value.
// Tests should restore via t.Cleanup.
func SetGitExecForTest(fn func(...string) error) func(...string) error {
	orig := gitExec
	gitExec = fn
	return orig
}

// commandVerb returns the configured verb for /cherry-pick: defaults to
// "cherry-pick" but configurable per config.CherryPickOptions.Command.
func (h *CherryPickHandler) commandVerb() string {
	switch h.opts.CherryPick.Command {
	case config.CherryPickCommandCherrypick:
		return "cherrypick"
	case config.CherryPickCommandCP:
		return "cp"
	default:
		return "cherry-pick"
	}
}

func (h *CherryPickHandler) Pre(_ context.Context, sc *event.Context, args []string) error {
	if sc.PR == nil {
		return PermissionError("/%s may only be used on pull requests", h.commandVerb())
	}
	if !sc.PR.Merged {
		return PermissionError("/%s may only be used on merged pull requests", h.commandVerb())
	}
	if len(args) == 0 || strings.TrimSpace(args[0]) == "" {
		return PermissionError("/%s requires a target branch as the first argument", h.commandVerb())
	}
	target := args[0]
	writer, err := h.ghc.HasWriteAccess(context.Background(), sc.Org, sc.Repo, sc.Author)
	if err != nil {
		return err
	}
	if !writer {
		return PermissionError("you do not have write access to %s/%s", sc.Org, sc.Repo)
	}
	if !h.allowedPattern.MatchString(target) {
		return PermissionError("target branch %q does not match allowed_branch_pattern %q",
			target, h.allowedPattern.String())
	}
	return nil
}

func (h *CherryPickHandler) Handle(ctx context.Context, sc *event.Context, args []string) error {
	target := args[0]
	newBranch := fmt.Sprintf("cherry-pick-%d-%s", sc.IssueNumber, target)
	newTitle := fmt.Sprintf("[cherry-pick %s] %s", target, sc.PR.Title)

	if err := gitExec("fetch", "origin", target); err != nil {
		return fmt.Errorf("fetching %s: %w", target, err)
	}
	if err := gitExec("checkout", "-b", newBranch, "origin/"+target); err != nil {
		return fmt.Errorf("creating branch: %w", err)
	}
	if err := gitExec(h.commandVerb(), sc.PR.MergeCommitSHA); err != nil {
		_ = gitExec(h.commandVerb(), "--abort")
		msg := fmt.Sprintf(
			"Cherry-pick of #%d onto `%s` failed due to conflicts.\n\nTo resolve manually:\n```\ngit fetch origin\ngit checkout -b %s origin/%s\ngit %s %s\n# resolve conflicts, then:\ngit push origin %s\n```",
			sc.IssueNumber, target, newBranch, target, h.commandVerb(), sc.PR.MergeCommitSHA, newBranch,
		)
		// Conflict is not an internal error — comment and return nil so the
		// +1 reaction still fires.
		return h.ghc.CreateIssueComment(ctx, sc.Org, sc.Repo, sc.IssueNumber, msg)
	}
	if err := gitExec("push", "origin", newBranch); err != nil {
		return fmt.Errorf("pushing branch: %w", err)
	}
	prNum, err := h.ghc.CreatePullRequest(ctx, sc.Org, sc.Repo, newTitle, newBranch, target,
		fmt.Sprintf("Cherry-pick of #%d.\n\nRequested by @%s.", sc.IssueNumber, sc.Author))
	if err != nil {
		return err
	}
	return h.ghc.CreateIssueComment(ctx, sc.Org, sc.Repo, sc.IssueNumber,
		fmt.Sprintf("Cherry-pick PR opened: #%d", prNum))
}

func (h *CherryPickHandler) Post(_ context.Context, _ *event.Context, _ []string, _ error) error {
	return nil
}
