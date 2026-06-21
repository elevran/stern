package pr_test

import (
	"bytes"
	"context"
	"io"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elevran/stern/internal/config"
	"github.com/elevran/stern/internal/github"
	"github.com/elevran/stern/internal/pr"
)

// captureLogger swaps logrus.StandardLogger().Out with a buffer for the
// duration of fn, restoring the original on return. This lets tests assert
// that an INFO message was logged (e.g. the least-busy fallback note).
func captureLogger(fn func()) string {
	var buf bytes.Buffer
	orig := logrus.StandardLogger().Out
	logrus.StandardLogger().SetOutput(&buf)
	defer logrus.StandardLogger().SetOutput(orig)
	fn()
	return buf.String()
}

// quietLogger silences logrus output during tests that intentionally
// exercise code paths which log info/warn messages.
func quietLogger() {
	logrus.SetOutput(io.Discard)
}

func reviewAssignmentOpts(enabled bool, count int, loadBalancing string) *config.Options {
	return &config.Options{
		ReviewAssignment: config.ReviewAssignmentOptions{
			Enabled:       enabled,
			LoadBalancing: loadBalancing,
			Count:         count,
		},
	}
}

func seedOwners(ghc *github.MockClient, sha string, content []byte) {
	ghc.FileContent["OWNERS@"+sha] = content
}

func TestHandlePREventReviewAssignment_DisabledNoOp(t *testing.T) {
	quietLogger()
	ghc := github.NewMockClient()
	ghc.PRFiles[1] = []string{"foo.go"}
	seedOwners(ghc, "sha1", []byte("approvers:\n  - alice\n"))
	p := github.PullRequest{Number: 1, Author: "bob", HeadSHA: "sha1"}

	require.NoError(t, pr.HandlePREventReviewAssignment(context.Background(), ghc, "o", "r", p, reviewAssignmentOpts(false, 1, "round-robin")))
	assert.Empty(t, ghc.ReviewersRequested, "expected no reviewer requests when disabled")
	assert.Empty(t, ghc.Comments, "expected no comments when disabled")
}

func TestHandlePREventReviewAssignment_NoOwnersNoOp(t *testing.T) {
	quietLogger()
	ghc := github.NewMockClient()
	ghc.PRFiles[1] = []string{"foo.go"}
	// No FileContent seeded: every OWNERS lookup will fail.
	p := github.PullRequest{Number: 1, Author: "bob", HeadSHA: "sha1"}

	require.NoError(t, pr.HandlePREventReviewAssignment(context.Background(), ghc, "o", "r", p, reviewAssignmentOpts(true, 1, "round-robin")))
	assert.Empty(t, ghc.ReviewersRequested, "expected no reviewer requests when no OWNERS found")
	assert.Empty(t, ghc.Comments, "expected no comments when no OWNERS found")
}

func TestHandlePREventReviewAssignment_NoFilesNoOp(t *testing.T) {
	quietLogger()
	ghc := github.NewMockClient()
	// No PRFiles seeded.
	seedOwners(ghc, "sha1", []byte("approvers:\n  - alice\n"))
	p := github.PullRequest{Number: 1, Author: "bob", HeadSHA: "sha1"}

	require.NoError(t, pr.HandlePREventReviewAssignment(context.Background(), ghc, "o", "r", p, reviewAssignmentOpts(true, 1, "round-robin")))
	assert.Empty(t, ghc.ReviewersRequested, "expected no reviewer requests when PR has no files")
}

func TestHandlePREventReviewAssignment_SingleApproverAssigned(t *testing.T) {
	quietLogger()
	ghc := github.NewMockClient()
	ghc.PRFiles[1] = []string{"foo.go"}
	seedOwners(ghc, "sha1", []byte("approvers:\n  - alice\n"))
	p := github.PullRequest{Number: 1, Author: "bob", HeadSHA: "sha1"}

	require.NoError(t, pr.HandlePREventReviewAssignment(context.Background(), ghc, "o", "r", p, reviewAssignmentOpts(true, 1, "round-robin")))
	require.Len(t, ghc.ReviewersRequested, 1)
	got := ghc.ReviewersRequested[0]
	assert.Equal(t, 1, got.Number)
	assert.Equal(t, []string{"alice"}, got.Users)
	require.Len(t, ghc.Comments, 1)
	assert.Equal(t, "Assigned reviewers: @alice", ghc.Comments[0].Body)
	assert.Equal(t, 1, ghc.Comments[0].Number)
}

func TestHandlePREventReviewAssignment_MultipleApproversPicksFirstCountSorted(t *testing.T) {
	quietLogger()
	ghc := github.NewMockClient()
	ghc.PRFiles[1] = []string{"foo.go"}
	seedOwners(ghc, "sha1", []byte("approvers:\n  - charlie\n  - alice\n  - bob\n"))
	p := github.PullRequest{Number: 1, Author: "dave", HeadSHA: "sha1"}

	require.NoError(t, pr.HandlePREventReviewAssignment(context.Background(), ghc, "o", "r", p, reviewAssignmentOpts(true, 2, "round-robin")))
	require.Len(t, ghc.ReviewersRequested, 1)
	got := ghc.ReviewersRequested[0]
	assert.Equal(t, []string{"alice", "bob"}, got.Users, "sorted, first Count")
	assert.Equal(t, "Assigned reviewers: @alice, @bob", ghc.Comments[0].Body)
}

func TestHandlePREventReviewAssignment_AuthorExcluded(t *testing.T) {
	quietLogger()
	ghc := github.NewMockClient()
	ghc.PRFiles[1] = []string{"foo.go"}
	// alice and bob are approvers, but alice is the PR author.
	seedOwners(ghc, "sha1", []byte("approvers:\n  - alice\n  - bob\n"))
	p := github.PullRequest{Number: 1, Author: "Alice", HeadSHA: "sha1"}

	require.NoError(t, pr.HandlePREventReviewAssignment(context.Background(), ghc, "o", "r", p, reviewAssignmentOpts(true, 2, "round-robin")))
	require.Len(t, ghc.ReviewersRequested, 1)
	got := ghc.ReviewersRequested[0]
	assert.Equal(t, []string{"bob"}, got.Users, "alice excluded as author")
	assert.Equal(t, "Assigned reviewers: @bob", ghc.Comments[0].Body)
}

func TestHandlePREventReviewAssignment_AllApproversAreAuthorNoOp(t *testing.T) {
	quietLogger()
	ghc := github.NewMockClient()
	ghc.PRFiles[1] = []string{"foo.go"}
	seedOwners(ghc, "sha1", []byte("approvers:\n  - alice\n"))
	p := github.PullRequest{Number: 1, Author: "alice", HeadSHA: "sha1"}

	require.NoError(t, pr.HandlePREventReviewAssignment(context.Background(), ghc, "o", "r", p, reviewAssignmentOpts(true, 1, "round-robin")))
	assert.Empty(t, ghc.ReviewersRequested, "expected no reviewer requests when all approvers are author")
	assert.Empty(t, ghc.Comments, "expected no comments when all approvers are author")
}

func TestHandlePREventReviewAssignment_CountLargerThanCandidates(t *testing.T) {
	quietLogger()
	ghc := github.NewMockClient()
	ghc.PRFiles[1] = []string{"foo.go"}
	seedOwners(ghc, "sha1", []byte("approvers:\n  - alice\n  - bob\n"))
	p := github.PullRequest{Number: 1, Author: "carol", HeadSHA: "sha1"}

	// Count=5 but only 2 candidates — should still assign the 2.
	require.NoError(t, pr.HandlePREventReviewAssignment(context.Background(), ghc, "o", "r", p, reviewAssignmentOpts(true, 5, "round-robin")))
	got := ghc.ReviewersRequested[0]
	assert.Equal(t, []string{"alice", "bob"}, got.Users)
}

func TestHandlePREventReviewAssignment_LeastBusyLogsInfoAndFallsBack(t *testing.T) {
	out := captureLogger(func() {
		ghc := github.NewMockClient()
		ghc.PRFiles[1] = []string{"foo.go"}
		seedOwners(ghc, "sha1", []byte("approvers:\n  - charlie\n  - alice\n  - bob\n"))
		p := github.PullRequest{Number: 1, Author: "dave", HeadSHA: "sha1"}

		require.NoError(t, pr.HandlePREventReviewAssignment(context.Background(), ghc, "o", "r", p, reviewAssignmentOpts(true, 2, "least-busy")))
		// Round-robin fallback: still picks the first Count after sort.
		got := ghc.ReviewersRequested[0]
		assert.Equal(t, []string{"alice", "bob"}, got.Users, "sorted, first Count")
	})
	assert.Contains(t, out, "least-busy strategy not yet implemented", "expected INFO log about least-busy fallback")
}