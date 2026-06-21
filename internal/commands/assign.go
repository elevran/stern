package commands

import (
	"context"

	"github.com/elevran/stern/internal/config"
	"github.com/elevran/stern/internal/event"
	"github.com/elevran/stern/internal/github"
)

// assignClient is the minimum Client surface AssignHandler uses.
type assignClient interface {
	github.PermissionsClient
	github.UsersClient
}

// AssignHandler handles /assign and /unassign.
type AssignHandler struct {
	ghc  assignClient
	verb string // "assign" or "unassign"
}

// NewAssignHandler constructs an AssignHandler bound to a single verb
// ("assign" or "unassign"). Each verb is registered as its own factory
// in DefaultRegistry.
func NewAssignHandler(verb string) HandlerFactory {
	return func(_ *event.Context, ghc github.Client, _ *config.Options) Handler {
		return &AssignHandler{ghc: ghc, verb: verb}
	}
}

func (h *AssignHandler) Pre(ctx context.Context, sc *event.Context, args []string) error {
	if sc.PR == nil {
		return PermissionError("/%s may only be used on pull requests", h.verb)
	}
	if len(args) == 0 {
		// Self-assign/unassign: any org member may do this.
		ok, err := h.ghc.IsOrgMember(ctx, sc.Org, sc.Author)
		if err != nil {
			return err
		}
		if !ok {
			return PermissionError("%s is not a member of %s", sc.Author, sc.Org)
		}
		return nil
	}
	// Assigning/unassigning others requires write access.
	ok, err := h.ghc.HasWriteAccess(ctx, sc.Org, sc.Repo, sc.Author)
	if err != nil {
		return err
	}
	if !ok {
		return PermissionError("%s does not have write access to %s others", sc.Author, h.verb)
	}
	return nil
}

func (h *AssignHandler) Handle(ctx context.Context, sc *event.Context, args []string) error {
	users, err := parseUsers(args, sc.Author)
	if err != nil {
		return err
	}
	if h.verb == "unassign" {
		return h.ghc.RemoveAssignees(ctx, sc.Org, sc.Repo, sc.IssueNumber, users)
	}
	return h.ghc.AddAssignees(ctx, sc.Org, sc.Repo, sc.IssueNumber, users)
}

// Post is a no-op: assign/unassign do not affect auto-merge eligibility.
func (h *AssignHandler) Post(_ context.Context, _ *event.Context, _ []string, _ error) error {
	return nil
}
