package commands

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/sirupsen/logrus"

	"github.com/elevran/stern/internal/config"
	"github.com/elevran/stern/internal/event"
	ghclient "github.com/elevran/stern/internal/github"
)

// Handler processes a single slash command.
type Handler interface {
	Handle(ctx context.Context, sc *event.Context, args []string, ghc ghclient.Client, opts *config.Options) error
}

// Registry maps command verbs to handlers.
type Registry map[string]Handler

// ErrPermission represents a permission-denied or validation error from a handler.
// The dispatch loop reacts with -1 and posts a comment.
type ErrPermission struct {
	message string
}

func (e *ErrPermission) Error() string { return e.message }

// PermissionError wraps a permission or validation error for the dispatch loop.
func PermissionError(format string, args ...interface{}) error {
	return &ErrPermission{message: fmt.Sprintf(format, args...)}
}

// Dispatch parses slash-commands from body and calls the matching handler for each.
// Success: +1 reaction on the triggering comment.
// Permission error: -1 reaction + comment.
// Internal error: confused reaction + comment with Actions run link.
// Unknown verb: logged only, no reaction.
func Dispatch(ctx context.Context, sc *event.Context, body string, reg Registry, ghc ghclient.Client, opts *config.Options) {
	log := logrus.WithFields(logrus.Fields{
		"org":    sc.Org,
		"repo":   sc.Repo,
		"issue":  sc.IssueNumber,
		"author": sc.Author,
	})

	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "/") {
			continue
		}
		parts := strings.Fields(strings.TrimPrefix(line, "/"))
		if len(parts) == 0 {
			continue
		}
		verb, args := parts[0], parts[1:]

		h, ok := reg[verb]
		if !ok {
			log.WithField("command", "/"+verb).Info("no handler registered")
			continue
		}

		log.WithField("command", "/"+verb).Info("dispatching")
		err := h.Handle(ctx, sc, args, ghc, opts)
		if err == nil {
			_ = ghc.CreateCommentReaction(ctx, sc.Org, sc.Repo, sc.CommentID, "+1")
			continue
		}

		var permErr *ErrPermission
		if errors.As(err, &permErr) {
			log.WithField("command", "/"+verb).WithError(err).Info("permission denied")
			_ = ghc.CreateCommentReaction(ctx, sc.Org, sc.Repo, sc.CommentID, "-1")
			_ = ghc.CreateIssueComment(ctx, sc.Org, sc.Repo, sc.IssueNumber, permErr.Error())
		} else {
			log.WithField("command", "/"+verb).WithError(err).Error("internal error")
			_ = ghc.CreateCommentReaction(ctx, sc.Org, sc.Repo, sc.CommentID, "confused")
			msg := fmt.Sprintf("Internal error handling `/%s`: %v", verb, err)
			if sc.ActionsRunURL != "" {
				msg += fmt.Sprintf("\n\nSee [Actions run](%s) for details.", sc.ActionsRunURL)
			}
			_ = ghc.CreateIssueComment(ctx, sc.Org, sc.Repo, sc.IssueNumber, msg)
		}
	}
}

// DefaultRegistry returns a Registry with all supported handlers pre-registered.
func DefaultRegistry() Registry {
	return Registry{
		"ping":    &pingHandler{},
		"lgtm":    &LGTMHandler{},
		"approve": &ApproveHandler{},
		"hold":    &HoldHandler{},
		"wip":     &WIPHandler{},
	}
}

// pingHandler handles /ping — confirms the bot is alive.
type pingHandler struct{}

func (h *pingHandler) Handle(ctx context.Context, sc *event.Context, args []string, ghc ghclient.Client, opts *config.Options) error {
	return nil // success: dispatch loop adds +1 reaction
}
