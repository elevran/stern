package commands

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/elevran/stern/internal/config"
	"github.com/elevran/stern/internal/event"
	"github.com/elevran/stern/internal/github"
)

// HelpHandler handles /help — posts a comment listing available commands.
type HelpHandler struct {
	opts *config.Options
	reg  Registry
	ghc  github.Client
}

// NewHelpHandler returns a HandlerFactory that wires a HelpHandler with a
// snapshot of the registry to render against. regFn is called once at
// construction time so that DefaultRegistry can be wired at package init
// without an import cycle.
func NewHelpHandler(regFn func() Registry) HandlerFactory {
	return func(_ *event.Context, ghc github.Client, opts *config.Options) Handler {
		return &HelpHandler{opts: opts, reg: regFn(), ghc: ghc}
	}
}

func (h *HelpHandler) Pre(_ context.Context, _ *event.Context, _ []string) error { return nil }

func (h *HelpHandler) Handle(ctx context.Context, sc *event.Context, _ []string) error {
	var sb strings.Builder
	sb.WriteString("## Available commands\n\n")
	for _, verb := range sortedVerbs(h.reg) {
		// Always-hidden: /ping (health-check) and /help (self).
		if verb == "ping" || verb == "help" {
			continue
		}
		// Plugin gating: when an explicit plugin list is configured,
		// skip commands whose plugin is not enabled.
		if h.opts != nil && len(h.opts.Plugins) > 0 && !h.opts.HasPlugin(verb) {
			continue
		}
		r := h.reg[verb]
		fmt.Fprintf(&sb, "- `%s` — %s\n", r.Info.Usage, r.Info.Short)
		if r.Info.Config != "" {
			fmt.Fprintf(&sb, "  _Config: `%s`_\n", r.Info.Config)
		}
	}
	return h.ghc.CreateIssueComment(ctx, sc.Org, sc.Repo, sc.IssueNumber, sb.String())
}

func (h *HelpHandler) Post(_ context.Context, _ *event.Context, _ []string, _ error) error {
	return nil
}

// sortedVerbs returns the registry's verbs in sorted order for deterministic
// /help output.
func sortedVerbs(reg Registry) []string {
	verbs := make([]string, 0, len(reg))
	for v := range reg {
		verbs = append(verbs, v)
	}
	sort.Strings(verbs)
	return verbs
}
