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

// Registry maps command verbs to handler factories.
type Registry map[string]HandlerFactory

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

	for line := range strings.SplitSeq(body, "\n") {
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

		factory, ok := reg[verb]
		if !ok {
			log.WithField("command", "/"+verb).Info("no handler registered")
			continue
		}

		log.WithField("command", "/"+verb).Info("dispatching")
		h := factory(sc, ghc, opts)

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
		"ping":    newPingHandler,
		"lgtm":    NewLGTMHandler,
		"approve": NewApproveHandler,
		"hold":    NewHoldHandler,
		"wip":     NewWIPHandler,
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
