package commands

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/sirupsen/logrus"

	"github.com/elevran/stern/internal/config"
	"github.com/elevran/stern/internal/event"
	"github.com/elevran/stern/internal/github"
	"github.com/elevran/stern/internal/merge"
)

// Handler processes a single slash command.
type Handler interface {
	// Pre performs authorization and argument validation. A non-nil error
	// prevents Handle from running and triggers the appropriate reaction.
	Pre(ctx context.Context, sc *event.Context, args []string) error
	// Handle performs the command's main mutation logic.
	Handle(ctx context.Context, sc *event.Context, args []string) error
	// Post runs after Handle regardless of the result; receives the Handle error.
	Post(ctx context.Context, sc *event.Context, args []string, handleErr error) error
}

// HandlerFactory creates a configured Handler for the given call context.
type HandlerFactory func(sc *event.Context, ghc github.Client, opts *config.Options) Handler

// CommandInfo holds human-readable documentation for a command.
type CommandInfo struct {
	Short  string // one-line summary shown in /help
	Usage  string // "/verb [cancel]" style usage line
	Config string // brief config field, empty if none
}

// Registration bundles a handler factory with its documentation.
type Registration struct {
	Factory HandlerFactory
	Info    CommandInfo
}

// Registry maps command verbs to registrations.
type Registry map[string]Registration

// labelMutatingBase implements Post for handlers that mutate labels.
// It fetches a fresh PR and re-evaluates auto-merge eligibility after every
// successful Handle call.
type labelMutatingBase struct {
	mergeGHC github.PullRequestsClient
	opts     *config.Options
}

func (b *labelMutatingBase) Post(ctx context.Context, sc *event.Context, _ []string, handleErr error) error {
	if handleErr != nil {
		return nil
	}
	pr, err := b.mergeGHC.GetPullRequest(ctx, sc.Org, sc.Repo, sc.IssueNumber)
	if err != nil {
		return err
	}
	return merge.CheckAndApplyAutoMerge(ctx, b.mergeGHC, pr, b.opts)
}

// ErrPermission represents a permission-denied or validation error from a handler.
type ErrPermission struct {
	message string
}

func (e *ErrPermission) Error() string { return e.message }

// PermissionError wraps a permission or validation error for the dispatch loop.
func PermissionError(format string, args ...any) error {
	return &ErrPermission{message: fmt.Sprintf(format, args...)}
}

// Dispatch parses slash-commands from body and calls the matching handler for each.
// Pre errors: -1 reaction for ErrPermission, confused for internal errors.
// Handle errors: same routing. Post errors are logged but do not affect reactions.
// Unknown verb: logged only, no reaction.
func Dispatch(ctx context.Context, sc *event.Context, body string, reg Registry, ghc github.Client, opts *config.Options) {
	log := logrus.WithFields(logrus.Fields{
		"org":    sc.Org,
		"repo":   sc.Repo,
		"issue":  sc.IssueNumber,
		"author": sc.Author,
	})

	inFence := false
	for line := range strings.SplitSeq(body, "\n") {
		trimmed := strings.TrimSpace(line)
		// Skip lines inside fenced code blocks; commands there are
		// documentation/examples, not invocations.
		if strings.HasPrefix(trimmed, "```") {
			inFence = !inFence
			continue
		}
		if inFence {
			continue
		}
		// Skip blockquote lines; quoting an old command must not re-trigger it.
		if strings.HasPrefix(trimmed, ">") {
			continue
		}
		tokens := strings.Fields(line)
		// find the first slash-command token anywhere in the line
		cmdIdx := -1
		for i, t := range tokens {
			if strings.HasPrefix(t, "/") {
				cmdIdx = i
				break
			}
		}
		if cmdIdx == -1 {
			continue
		}
		// strip leading "/" and trailing punctuation from the verb
		verb := strings.TrimLeft(tokens[cmdIdx], "/")
		verb = strings.TrimRight(verb, ".,!?;:")
		if verb == "" {
			continue
		}
		args := tokens[cmdIdx+1:]

		registration, ok := reg[verb]
		if !ok {
			log.WithField("command", "/"+verb).Info("no handler registered")
			continue
		}

		if verb != "ping" && verb != "help" && opts != nil && len(opts.Plugins) > 0 && !opts.HasPlugin(verb) {
			log.WithField("command", "/"+verb).Info("plugin not enabled")
			continue
		}

		log.WithField("command", "/"+verb).Info("dispatching")
		h := registration.Factory(sc, ghc, opts)

		if preErr := h.Pre(ctx, sc, args); preErr != nil {
			dispatchErr(ctx, log, sc, ghc, verb, preErr)
			continue
		}

		handleErr := h.Handle(ctx, sc, args)
		_ = h.Post(ctx, sc, args, handleErr)
		if handleErr == nil {
			_ = ghc.CreateCommentReaction(ctx, sc.Org, sc.Repo, sc.CommentID, "+1")
			continue
		}
		dispatchErr(ctx, log, sc, ghc, verb, handleErr)
	}
}

func dispatchErr(ctx context.Context, log *logrus.Entry, sc *event.Context, ghc github.Client, verb string, err error) {
	var permErr *ErrPermission
	if errors.As(err, &permErr) {
		log.WithField("command", "/"+verb).WithError(err).Info("permission denied")
		_ = ghc.CreateCommentReaction(ctx, sc.Org, sc.Repo, sc.CommentID, "-1")
		_ = ghc.CreateIssueComment(ctx, sc.Org, sc.Repo, sc.IssueNumber, permErr.Error())
	} else {
		log.WithField("command", "/"+verb).WithError(err).Error("internal error")
		_ = ghc.CreateCommentReaction(ctx, sc.Org, sc.Repo, sc.CommentID, "confused")
		msg := fmt.Sprintf("Internal error handling `/%s`. See the [Actions run](%s) for details.", verb, sc.ActionsRunURL)
		if sc.ActionsRunURL == "" {
			msg = fmt.Sprintf("Internal error handling `/%s`.", verb)
		}
		_ = ghc.CreateIssueComment(ctx, sc.Org, sc.Repo, sc.IssueNumber, msg)
	}
}

// DefaultRegistry returns a Registry with all supported handlers pre-registered.
func DefaultRegistry() Registry {
	return Registry{
		"ping": {Factory: newPingHandler, Info: CommandInfo{
			Short: "Check that the bot is alive",
			Usage: "/ping",
		}},
		"help": {Factory: NewHelpHandler(DefaultRegistry), Info: CommandInfo{
			Short: "List available commands",
			Usage: "/help",
		}},
		"lgtm": {Factory: NewLGTMHandler, Info: CommandInfo{
			Short:  "Approve a PR as a reviewer (LGTM)",
			Usage:  "/lgtm [cancel]",
			Config: "lgtm.allow_self_lgtm",
		}},
		"approve": {Factory: NewApproveHandler, Info: CommandInfo{
			Short:  "Approve a PR (auto-merges when both /lgtm and /approve are present)",
			Usage:  "/approve [cancel]",
			Config: "approve.allow_self_approval, approve.require_owner",
		}},
		"hold": {Factory: NewHoldHandler, Info: CommandInfo{
			Short: "Block merging of a PR",
			Usage: "/hold [cancel]",
		}},
		"wip": {Factory: NewWIPHandler, Info: CommandInfo{
			Short: "Mark a PR as work-in-progress",
			Usage: "/wip [cancel]",
		}},
		"close": {Factory: newCloseHandler("close"), Info: CommandInfo{
			Short: "Close an issue or PR",
			Usage: "/close",
		}},
		"reopen": {Factory: newCloseHandler("reopen"), Info: CommandInfo{
			Short: "Reopen a closed issue or PR",
			Usage: "/reopen",
		}},
		"milestone": {Factory: NewMilestoneHandler, Info: CommandInfo{
			Short: "Set the milestone on an issue or PR",
			Usage: "/milestone <name> [cancel]",
		}},
		"assign": {Factory: NewAssignHandler("assign"), Info: CommandInfo{
			Short: "Assign a user to an issue or PR",
			Usage: "/assign <user>",
		}},
		"unassign": {Factory: NewAssignHandler("unassign"), Info: CommandInfo{
			Short: "Remove an assignee from an issue or PR",
			Usage: "/unassign <user>",
		}},
		"cc": {Factory: NewCCHandler("cc"), Info: CommandInfo{
			Short: "Request a review from a user",
			Usage: "/cc <user>",
		}},
		"uncc": {Factory: NewCCHandler("uncc"), Info: CommandInfo{
			Short: "Remove a review request from a user",
			Usage: "/uncc <user>",
		}},
		"retest": {Factory: NewRetestHandler, Info: CommandInfo{
			Short: "Re-run failed CI checks",
			Usage: "/retest",
		}},
		"kind": {Factory: NewKindHandler, Info: CommandInfo{
			Short: "Set the `kind/*` label on a PR",
			Usage: "/kind <name>",
		}},
		"area": {Factory: NewAreaHandler, Info: CommandInfo{
			Short: "Set the `area/*` label on a PR",
			Usage: "/area <name>",
		}},
		"priority": {Factory: NewPriorityHandler, Info: CommandInfo{
			Short: "Set the `priority/*` label on a PR",
			Usage: "/priority <name>",
		}},
		"lifecycle": {Factory: NewLifecycleHandler, Info: CommandInfo{
			Short:  "Manage stale/rotten lifecycle labels on issues and PRs",
			Usage:  "/lifecycle <set|clear>",
			Config: "lifecycle.enabled",
		}},
	}
}

func newPingHandler(_ *event.Context, _ github.Client, _ *config.Options) Handler {
	return &pingHandler{}
}

// pingHandler handles /ping — confirms the bot is alive.
type pingHandler struct{}

func (h *pingHandler) Pre(_ context.Context, _ *event.Context, _ []string) error    { return nil }
func (h *pingHandler) Handle(_ context.Context, _ *event.Context, _ []string) error { return nil }
func (h *pingHandler) Post(_ context.Context, _ *event.Context, _ []string, _ error) error {
	return nil
}
