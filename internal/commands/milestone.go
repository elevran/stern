package commands

import (
	"context"
	"strconv"
	"strings"

	"github.com/elevran/stern/internal/config"
	"github.com/elevran/stern/internal/event"
	"github.com/elevran/stern/internal/github"
)

// milestoneClient is the minimum Client surface MilestoneHandler uses.
type milestoneClient interface {
	github.PermissionsClient
	github.MilestoneClient
}

// MilestoneHandler handles /milestone.
type MilestoneHandler struct {
	ghc milestoneClient
}

// NewMilestoneHandler constructs a MilestoneHandler with all dependencies injected.
func NewMilestoneHandler(_ *event.Context, ghc github.Client, _ *config.Options) Handler {
	return &MilestoneHandler{ghc: ghc}
}

// Pre enforces that /milestone requires repo write access.
func (h *MilestoneHandler) Pre(ctx context.Context, sc *event.Context, _ []string) error {
	ok, err := h.ghc.HasWriteAccess(ctx, sc.Org, sc.Repo, sc.Author)
	if err != nil {
		return err
	}
	if !ok {
		return PermissionError("%s does not have write access to set a milestone", sc.Author)
	}
	return nil
}

// Handle resolves the milestone argument: "clear" removes the milestone, a
// numeric ID is applied directly, otherwise the argument is matched
// case-insensitively against existing milestone titles.
func (h *MilestoneHandler) Handle(ctx context.Context, sc *event.Context, args []string) error {
	if len(args) == 0 {
		return PermissionError("usage: /milestone <title-or-id> | clear")
	}
	arg := args[0]
	if strings.EqualFold(arg, "clear") {
		return h.ghc.ClearMilestone(ctx, sc.Org, sc.Repo, sc.IssueNumber)
	}
	if id, err := strconv.Atoi(arg); err == nil {
		return h.ghc.SetMilestone(ctx, sc.Org, sc.Repo, sc.IssueNumber, id)
	}
	milestones, err := h.ghc.ListMilestones(ctx, sc.Org, sc.Repo)
	if err != nil {
		return err
	}
	want := strings.ToLower(arg)
	for _, m := range milestones {
		if strings.ToLower(m.Title) == want {
			return h.ghc.SetMilestone(ctx, sc.Org, sc.Repo, sc.IssueNumber, m.Number)
		}
	}
	return PermissionError("milestone not found: %s", arg)
}

// Post is a no-op for /milestone; it does not affect auto-merge.
func (h *MilestoneHandler) Post(_ context.Context, _ *event.Context, _ []string, _ error) error {
	return nil
}
