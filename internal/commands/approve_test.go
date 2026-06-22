package commands_test

import (
	"context"
	"errors"
	"testing"

	"github.com/elevran/stern/internal/commands"
	"github.com/elevran/stern/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// default-strict path); kept as a parameter so future cases exercising
// allow_self_approval: true can override without touching the helper.
//
//nolint:unparam // allowSelf is fixed to false across current callers (the
func approveOpts(allowSelf bool) *config.Options {
	return &config.Options{
		Approve: config.ApproveOptions{
			AllowSelfApproval: allowSelf,
			InvalidateOnPush:  false,
		},
		Merge: config.MergeOptions{
			Method:         "squash",
			BlockingLabels: []string{"do-not-merge/hold"},
		},
	}
}

func TestApprove_AddsLabel(t *testing.T) {
	sc, ghc := prContext("author")
	sc.Author = "approver"
	ghc.FileContent["OWNERS@base456"] = []byte("approvers:\n  - approver\n")
	ghc.PRFiles[1] = []string{"main.go"}

	reg := commands.Registry{"approve": {Factory: commands.NewApproveHandler}}
	commands.Dispatch(context.Background(), sc, "/approve", reg, ghc, approveOpts(false))

	assert.True(t, ghc.IssueLabels[1]["approved"], "expected approved label to be added")
	require.NotEmpty(t, ghc.Reactions)
	assert.Equal(t, "+1", ghc.Reactions[0].Content, "expected +1 reaction after successful /approve")
}

func TestApprove_Cancel_RemovesLabel(t *testing.T) {
	sc, ghc := prContext("author")
	sc.Author = "approver"
	ghc.WriteAccess["elevran/stern/approver"] = true
	ghc.IssueLabels[1] = map[string]bool{"approved": true}

	reg := commands.Registry{"approve": {Factory: commands.NewApproveHandler}}
	commands.Dispatch(context.Background(), sc, "/approve cancel", reg, ghc, approveOpts(false))

	assert.False(t, ghc.IssueLabels[1]["approved"], "expected approved label removed on cancel")
	require.NotEmpty(t, ghc.Reactions)
	assert.Equal(t, "+1", ghc.Reactions[0].Content, "expected +1 reaction after successful /approve cancel")
}

func TestApprove_Cancel_RequiresWriteAccess(t *testing.T) {
	sc, ghc := prContext("author")
	sc.Author = "outsider"
	ghc.WriteAccess["elevran/stern/outsider"] = false
	ghc.IssueLabels[1] = map[string]bool{"approved": true}

	reg := commands.Registry{"approve": {Factory: commands.NewApproveHandler}}
	commands.Dispatch(context.Background(), sc, "/approve cancel", reg, ghc, approveOpts(false))

	assert.True(t, ghc.IssueLabels[1]["approved"], "expected approved label NOT removed without write access")
	require.NotEmpty(t, ghc.Reactions)
	assert.Equal(t, "-1", ghc.Reactions[0].Content, "expected -1 reaction")
}

func TestApprove_SelfApprovalDenied(t *testing.T) {
	sc, ghc := prContext("approver")
	sc.Author = "approver" // PR author == commenter

	reg := commands.Registry{"approve": {Factory: commands.NewApproveHandler}}
	commands.Dispatch(context.Background(), sc, "/approve", reg, ghc, approveOpts(false))

	assert.False(t, ghc.IssueLabels[1]["approved"], "expected approved NOT added for self-approval")
	require.NotEmpty(t, ghc.Reactions)
	assert.Equal(t, "-1", ghc.Reactions[0].Content, "expected -1 reaction")
}

func TestApprove_NonApproverDenied(t *testing.T) {
	sc, ghc := prContext("author")
	sc.Author = "outsider"
	ghc.FileContent["OWNERS@base456"] = []byte("approvers:\n  - alice\n")
	ghc.PRFiles[1] = []string{"main.go"}

	reg := commands.Registry{"approve": {Factory: commands.NewApproveHandler}}
	commands.Dispatch(context.Background(), sc, "/approve", reg, ghc, approveOpts(false))

	assert.False(t, ghc.IssueLabels[1]["approved"], "expected approved NOT added for non-approver")
	require.NotEmpty(t, ghc.Reactions)
	assert.Equal(t, "-1", ghc.Reactions[0].Content, "expected -1 reaction")
}

func TestApprove_OWNERSAtHeadSHA_NotTrusted(t *testing.T) {
	// An attacker can put an OWNERS file on their fork branch (HeadSHA). The
	// auth check must read OWNERS at the base ref instead — listings at the
	// head ref must be ignored.
	sc, ghc := prContext("author")
	sc.Author = "approver"
	ghc.FileContent["OWNERS@abc123"] = []byte("approvers:\n  - approver\n")
	// No OWNERS at base ref → fallback to write-access check, which fails.
	ghc.PRFiles[1] = []string{"main.go"}

	reg := commands.Registry{"approve": {Factory: commands.NewApproveHandler}}
	commands.Dispatch(context.Background(), sc, "/approve", reg, ghc, approveOpts(false))

	assert.False(t, ghc.IssueLabels[1]["approved"], "expected approved NOT added — OWNERS at attacker-controlled HeadSHA must be ignored")
	require.NotEmpty(t, ghc.Reactions)
	assert.Equal(t, "-1", ghc.Reactions[0].Content, "expected -1 reaction")
}

func TestApprove_BothLGTMAndApproved_TriggersAutoMerge(t *testing.T) {
	sc, ghc := prContext("author")
	sc.Author = "approver"
	ghc.FileContent["OWNERS@base456"] = []byte("approvers:\n  - approver\n")
	ghc.PRFiles[1] = []string{"main.go"}
	// Pre-load the PR with both labels so Post's re-check via GetPullRequest
	// sees an eligible PR and triggers EnableAutoMerge. The mock's AddLabels
	// does not sync back into PullRequests[1].Labels, so we set the final
	// state here.
	ghc.PullRequests[1].Labels = []string{"lgtm", "approved"}
	ghc.IssueLabels[1] = map[string]bool{"lgtm": true, "approved": true}

	reg := commands.Registry{"approve": {Factory: commands.NewApproveHandler}}
	commands.Dispatch(context.Background(), sc, "/approve", reg, ghc, approveOpts(false))

	assert.True(t, ghc.IssueLabels[1]["approved"], "expected approved label added")
	assert.Equal(t, 1, ghc.EnableAutoMergeCallCount, "expected EnableAutoMergeCallCount=1 after both labels present")
	require.NotEmpty(t, ghc.Reactions)
	assert.Equal(t, "+1", ghc.Reactions[0].Content, "expected +1 reaction after successful /approve")
}

func TestApprove_HandleError_SuppressesPost(t *testing.T) {
	sc, ghc := prContext("author")
	sc.Author = "approver"
	ghc.WriteAccess["elevran/stern/approver"] = true
	ghc.Errors["AddLabels"] = errors.New("boom")

	reg := commands.Registry{"approve": {Factory: commands.NewApproveHandler}}
	commands.Dispatch(context.Background(), sc, "/approve", reg, ghc, approveOpts(false))

	assert.Empty(t, ghc.AutoMergeEnabled, "expected Post NOT to run when Handle errors")
	assert.Empty(t, ghc.AutoMergeDisabled, "expected Post NOT to run when Handle errors")
	require.NotEmpty(t, ghc.Reactions)
	assert.Equal(t, "confused", ghc.Reactions[0].Content, "expected confused reaction on internal error")
}

// TestApprove_PartialOwnership_Rejected covers #100: when the commenter is
// only an approver for one of two directories touched by the PR, /approve
// must fail with a PermissionError listing the uncovered file.
func TestApprove_PartialOwnership_Rejected(t *testing.T) {
	sc, ghc := prContext("author")
	sc.Author = "alice"
	ghc.FileContent["dir-a/OWNERS@base456"] = []byte("approvers:\n  - alice\n")
	ghc.FileContent["dir-b/OWNERS@base456"] = []byte("approvers:\n  - bob\n")
	ghc.PRFiles[1] = []string{"dir-a/f.go", "dir-b/f.go"}

	reg := commands.Registry{"approve": {Factory: commands.NewApproveHandler}}
	commands.Dispatch(context.Background(), sc, "/approve", reg, ghc, approveOpts(false))

	assert.False(t, ghc.IssueLabels[1]["approved"], "expected approved NOT added for partial ownership")
	require.NotEmpty(t, ghc.Reactions)
	assert.Equal(t, "-1", ghc.Reactions[0].Content, "expected -1 reaction")
	require.NotEmpty(t, ghc.Comments, "expected permission error comment")
	assert.Contains(t, ghc.Comments[0].Body, "dir-b/f.go", "expected uncovered file listed in error comment")
}

// TestApprove_FullOwnership_Accepted: commenter is an approver for every
// changed file (via the root OWNERS) → /approve succeeds.
func TestApprove_FullOwnership_Accepted(t *testing.T) {
	sc, ghc := prContext("author")
	sc.Author = "alice"
	ghc.FileContent["OWNERS@base456"] = []byte("approvers:\n  - alice\n")
	ghc.PRFiles[1] = []string{"dir-a/f.go", "dir-b/f.go"}

	reg := commands.Registry{"approve": {Factory: commands.NewApproveHandler}}
	commands.Dispatch(context.Background(), sc, "/approve", reg, ghc, approveOpts(false))

	assert.True(t, ghc.IssueLabels[1]["approved"], "expected approved label added for full ownership")
	require.NotEmpty(t, ghc.Reactions)
	assert.Equal(t, "+1", ghc.Reactions[0].Content, "expected +1 reaction")
}

// TestApprove_NoOwners_NoWriteAccess_Denied: no OWNERS covers the changed
// paths and the commenter has no write access — must fail closed.
func TestApprove_NoOwners_NoWriteAccess_Denied(t *testing.T) {
	sc, ghc := prContext("author")
	sc.Author = "anyone"
	ghc.PRFiles[1] = []string{"some/path/f.go"}

	reg := commands.Registry{"approve": {Factory: commands.NewApproveHandler}}
	commands.Dispatch(context.Background(), sc, "/approve", reg, ghc, approveOpts(false))

	assert.False(t, ghc.IssueLabels[1]["approved"], "expected approved NOT added when no OWNERS and no write access")
	require.NotEmpty(t, ghc.Reactions)
	assert.Equal(t, "-1", ghc.Reactions[0].Content, "expected -1 reaction when no OWNERS and no write access")
}

// TestApprove_NoOwners_WriteAccess_Allowed: no OWNERS covers the changed
// paths but the commenter has write access — must be allowed (maintainer
// escape hatch).
func TestApprove_NoOwners_WriteAccess_Allowed(t *testing.T) {
	sc, ghc := prContext("author")
	sc.Author = "anyone"
	ghc.WriteAccess["elevran/stern/anyone"] = true
	ghc.PRFiles[1] = []string{"some/path/f.go"}

	reg := commands.Registry{"approve": {Factory: commands.NewApproveHandler}}
	commands.Dispatch(context.Background(), sc, "/approve", reg, ghc, approveOpts(false))

	assert.True(t, ghc.IssueLabels[1]["approved"], "expected approved added for write-access user when no OWNERS")
	require.NotEmpty(t, ghc.Reactions)
	assert.Equal(t, "+1", ghc.Reactions[0].Content, "expected +1 reaction for write-access user with no OWNERS")
}

// TestApprove_EmptyBaseSHAFailsClosed verifies that /approve fails when the
// PR's BaseSHA is unknown. The early-return lives in the shared checkOwners
// helper; checkApproveOwners delegates to it before running per-file checks.
func TestApprove_EmptyBaseSHAFailsClosed(t *testing.T) {
	sc, ghc := prContext("author")
	sc.Author = "outsider"
	sc.PR.BaseSHA = "" // unknown base — must fail closed

	reg := commands.Registry{"approve": {Factory: commands.NewApproveHandler}}
	commands.Dispatch(context.Background(), sc, "/approve", reg, ghc, approveOpts(false))

	assert.False(t, ghc.IssueLabels[1]["approved"], "expected approved NOT added when BaseSHA is unknown")
	require.NotEmpty(t, ghc.Reactions)
	assert.Equal(t, "-1", ghc.Reactions[0].Content, "expected -1 reaction (permission error)")
}
