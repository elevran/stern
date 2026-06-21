package pr

import (
	"context"
	"testing"

	"github.com/elevran/stern/internal/config"
	"github.com/elevran/stern/internal/github"
)

func handlerWIPOpts() *config.Options {
	return &config.Options{
		Merge: config.MergeOptions{
			Method:         "squash",
			BlockingLabels: []string{"do-not-merge/wip"},
		},
	}
}

// TestPREventHandler_handleWIP_AddsOnWIPTitle verifies the unexported
// handleWIP method adds the WIP label when the PR title contains a WIP
// marker.
func TestPREventHandler_handleWIP_AddsOnWIPTitle(t *testing.T) {
	ghc := github.NewMockClient()
	p := github.PullRequest{
		Number: 1,
		Title:  "[WIP] test",
		Labels: []string{},
	}

	h := &PREventHandler{ghc: ghc, opts: handlerWIPOpts()}
	if err := h.handleWIP(context.Background(), "o", "r", p); err != nil {
		t.Fatalf("handleWIP() error = %v", err)
	}
	if !ghc.IssueLabels[1]["do-not-merge/wip"] {
		t.Error("expected wip label added for WIP title")
	}
}

// TestPREventHandler_handleWIP_RemovesWhenTitleClean verifies the
// unexported handleWIP method removes the WIP label once the title no
// longer indicates WIP.
func TestPREventHandler_handleWIP_RemovesWhenTitleClean(t *testing.T) {
	ghc := github.NewMockClient()
	ghc.IssueLabels[1] = map[string]bool{"do-not-merge/wip": true}
	p := github.PullRequest{
		Number: 1,
		Title:  "Fix the bug",
		Labels: []string{"do-not-merge/wip"},
	}
	ghc.PullRequests[1] = &p

	h := &PREventHandler{ghc: ghc, opts: handlerWIPOpts()}
	if err := h.handleWIP(context.Background(), "o", "r", p); err != nil {
		t.Fatalf("handleWIP() error = %v", err)
	}
	if ghc.IssueLabels[1]["do-not-merge/wip"] {
		t.Error("expected wip label removed when title is clean")
	}
}

// TestPREventHandler_invalidateLGTMOnPush verifies the unexported method
// removes the lgtm label when InvalidateOnPush is enabled.
func TestPREventHandler_invalidateLGTMOnPush(t *testing.T) {
	ghc := github.NewMockClient()
	ghc.IssueLabels[1] = map[string]bool{"lgtm": true}
	p := github.PullRequest{
		Number: 1,
		Labels: []string{"lgtm"},
	}
	opts := &config.Options{
		LGTM: config.LGTMOptions{InvalidateOnPush: true},
	}

	h := &PREventHandler{ghc: ghc, opts: opts}
	if err := h.invalidateLGTMOnPush(context.Background(), "o", "r", p); err != nil {
		t.Fatalf("invalidateLGTMOnPush() error = %v", err)
	}
	if ghc.IssueLabels[1]["lgtm"] {
		t.Error("expected lgtm label removed on push")
	}
}

// TestPREventHandler_invalidateApproveOnPush verifies the unexported
// method removes the approved label when InvalidateOnPush is enabled.
func TestPREventHandler_invalidateApproveOnPush(t *testing.T) {
	ghc := github.NewMockClient()
	ghc.IssueLabels[1] = map[string]bool{"approved": true}
	p := github.PullRequest{
		Number: 1,
		Labels: []string{"approved"},
	}
	opts := &config.Options{
		Approve: config.ApproveOptions{InvalidateOnPush: true},
	}

	h := &PREventHandler{ghc: ghc, opts: opts}
	if err := h.invalidateApproveOnPush(context.Background(), "o", "r", p); err != nil {
		t.Fatalf("invalidateApproveOnPush() error = %v", err)
	}
	if ghc.IssueLabels[1]["approved"] {
		t.Error("expected approved label removed on push")
	}
}
