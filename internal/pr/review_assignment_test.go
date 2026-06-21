package pr_test

import (
	"bytes"
	"context"
	"io"
	"slices"
	"strings"
	"testing"

	"github.com/sirupsen/logrus"

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

	if err := pr.HandlePREventReviewAssignment(context.Background(), ghc, "o", "r", p, reviewAssignmentOpts(false, 1, "round-robin")); err != nil {
		t.Fatalf("HandlePREventReviewAssignment() error = %v", err)
	}
	if len(ghc.ReviewersRequested) != 0 {
		t.Errorf("expected no reviewer requests when disabled, got %v", ghc.ReviewersRequested)
	}
	if len(ghc.Comments) != 0 {
		t.Errorf("expected no comments when disabled, got %v", ghc.Comments)
	}
}

func TestHandlePREventReviewAssignment_NoOwnersNoOp(t *testing.T) {
	quietLogger()
	ghc := github.NewMockClient()
	ghc.PRFiles[1] = []string{"foo.go"}
	// No FileContent seeded: every OWNERS lookup will fail.
	p := github.PullRequest{Number: 1, Author: "bob", HeadSHA: "sha1"}

	if err := pr.HandlePREventReviewAssignment(context.Background(), ghc, "o", "r", p, reviewAssignmentOpts(true, 1, "round-robin")); err != nil {
		t.Fatalf("HandlePREventReviewAssignment() error = %v", err)
	}
	if len(ghc.ReviewersRequested) != 0 {
		t.Errorf("expected no reviewer requests when no OWNERS found, got %v", ghc.ReviewersRequested)
	}
	if len(ghc.Comments) != 0 {
		t.Errorf("expected no comments when no OWNERS found, got %v", ghc.Comments)
	}
}

func TestHandlePREventReviewAssignment_NoFilesNoOp(t *testing.T) {
	quietLogger()
	ghc := github.NewMockClient()
	// No PRFiles seeded.
	seedOwners(ghc, "sha1", []byte("approvers:\n  - alice\n"))
	p := github.PullRequest{Number: 1, Author: "bob", HeadSHA: "sha1"}

	if err := pr.HandlePREventReviewAssignment(context.Background(), ghc, "o", "r", p, reviewAssignmentOpts(true, 1, "round-robin")); err != nil {
		t.Fatalf("HandlePREventReviewAssignment() error = %v", err)
	}
	if len(ghc.ReviewersRequested) != 0 {
		t.Errorf("expected no reviewer requests when PR has no files, got %v", ghc.ReviewersRequested)
	}
}

func TestHandlePREventReviewAssignment_SingleApproverAssigned(t *testing.T) {
	quietLogger()
	ghc := github.NewMockClient()
	ghc.PRFiles[1] = []string{"foo.go"}
	seedOwners(ghc, "sha1", []byte("approvers:\n  - alice\n"))
	p := github.PullRequest{Number: 1, Author: "bob", HeadSHA: "sha1"}

	if err := pr.HandlePREventReviewAssignment(context.Background(), ghc, "o", "r", p, reviewAssignmentOpts(true, 1, "round-robin")); err != nil {
		t.Fatalf("HandlePREventReviewAssignment() error = %v", err)
	}
	if len(ghc.ReviewersRequested) != 1 {
		t.Fatalf("expected 1 reviewer request, got %d: %v", len(ghc.ReviewersRequested), ghc.ReviewersRequested)
	}
	got := ghc.ReviewersRequested[0]
	if got.Number != 1 {
		t.Errorf("RequestReviewers number = %d, want 1", got.Number)
	}
	if !slices.Equal(got.Users, []string{"alice"}) {
		t.Errorf("RequestReviewers users = %v, want [alice]", got.Users)
	}
	if len(ghc.Comments) != 1 {
		t.Fatalf("expected 1 comment, got %d", len(ghc.Comments))
	}
	want := "Assigned reviewers: @alice"
	if ghc.Comments[0].Body != want {
		t.Errorf("comment body = %q, want %q", ghc.Comments[0].Body, want)
	}
	if ghc.Comments[0].Number != 1 {
		t.Errorf("comment number = %d, want 1", ghc.Comments[0].Number)
	}
}

func TestHandlePREventReviewAssignment_MultipleApproversPicksFirstCountSorted(t *testing.T) {
	quietLogger()
	ghc := github.NewMockClient()
	ghc.PRFiles[1] = []string{"foo.go"}
	seedOwners(ghc, "sha1", []byte("approvers:\n  - charlie\n  - alice\n  - bob\n"))
	p := github.PullRequest{Number: 1, Author: "dave", HeadSHA: "sha1"}

	if err := pr.HandlePREventReviewAssignment(context.Background(), ghc, "o", "r", p, reviewAssignmentOpts(true, 2, "round-robin")); err != nil {
		t.Fatalf("HandlePREventReviewAssignment() error = %v", err)
	}
	if len(ghc.ReviewersRequested) != 1 {
		t.Fatalf("expected 1 reviewer request call, got %d", len(ghc.ReviewersRequested))
	}
	got := ghc.ReviewersRequested[0]
	if !slices.Equal(got.Users, []string{"alice", "bob"}) {
		t.Errorf("RequestReviewers users = %v, want [alice bob] (sorted, first Count)", got.Users)
	}
	want := "Assigned reviewers: @alice, @bob"
	if ghc.Comments[0].Body != want {
		t.Errorf("comment body = %q, want %q", ghc.Comments[0].Body, want)
	}
}

func TestHandlePREventReviewAssignment_AuthorExcluded(t *testing.T) {
	quietLogger()
	ghc := github.NewMockClient()
	ghc.PRFiles[1] = []string{"foo.go"}
	// alice and bob are approvers, but alice is the PR author.
	seedOwners(ghc, "sha1", []byte("approvers:\n  - alice\n  - bob\n"))
	p := github.PullRequest{Number: 1, Author: "Alice", HeadSHA: "sha1"}

	if err := pr.HandlePREventReviewAssignment(context.Background(), ghc, "o", "r", p, reviewAssignmentOpts(true, 2, "round-robin")); err != nil {
		t.Fatalf("HandlePREventReviewAssignment() error = %v", err)
	}
	if len(ghc.ReviewersRequested) != 1 {
		t.Fatalf("expected 1 reviewer request, got %d", len(ghc.ReviewersRequested))
	}
	got := ghc.ReviewersRequested[0]
	if !slices.Equal(got.Users, []string{"bob"}) {
		t.Errorf("RequestReviewers users = %v, want [bob] (alice excluded as author)", got.Users)
	}
	if ghc.Comments[0].Body != "Assigned reviewers: @bob" {
		t.Errorf("comment body = %q, want %q", ghc.Comments[0].Body, "Assigned reviewers: @bob")
	}
}

func TestHandlePREventReviewAssignment_AllApproversAreAuthorNoOp(t *testing.T) {
	quietLogger()
	ghc := github.NewMockClient()
	ghc.PRFiles[1] = []string{"foo.go"}
	seedOwners(ghc, "sha1", []byte("approvers:\n  - alice\n"))
	p := github.PullRequest{Number: 1, Author: "alice", HeadSHA: "sha1"}

	if err := pr.HandlePREventReviewAssignment(context.Background(), ghc, "o", "r", p, reviewAssignmentOpts(true, 1, "round-robin")); err != nil {
		t.Fatalf("HandlePREventReviewAssignment() error = %v", err)
	}
	if len(ghc.ReviewersRequested) != 0 {
		t.Errorf("expected no reviewer requests when all approvers are author, got %v", ghc.ReviewersRequested)
	}
	if len(ghc.Comments) != 0 {
		t.Errorf("expected no comments when all approvers are author, got %v", ghc.Comments)
	}
}

func TestHandlePREventReviewAssignment_CountLargerThanCandidates(t *testing.T) {
	quietLogger()
	ghc := github.NewMockClient()
	ghc.PRFiles[1] = []string{"foo.go"}
	seedOwners(ghc, "sha1", []byte("approvers:\n  - alice\n  - bob\n"))
	p := github.PullRequest{Number: 1, Author: "carol", HeadSHA: "sha1"}

	// Count=5 but only 2 candidates — should still assign the 2.
	if err := pr.HandlePREventReviewAssignment(context.Background(), ghc, "o", "r", p, reviewAssignmentOpts(true, 5, "round-robin")); err != nil {
		t.Fatalf("HandlePREventReviewAssignment() error = %v", err)
	}
	got := ghc.ReviewersRequested[0]
	if !slices.Equal(got.Users, []string{"alice", "bob"}) {
		t.Errorf("RequestReviewers users = %v, want [alice bob]", got.Users)
	}
}

func TestHandlePREventReviewAssignment_LeastBusyLogsInfoAndFallsBack(t *testing.T) {
	out := captureLogger(func() {
		ghc := github.NewMockClient()
		ghc.PRFiles[1] = []string{"foo.go"}
		seedOwners(ghc, "sha1", []byte("approvers:\n  - charlie\n  - alice\n  - bob\n"))
		p := github.PullRequest{Number: 1, Author: "dave", HeadSHA: "sha1"}

		if err := pr.HandlePREventReviewAssignment(context.Background(), ghc, "o", "r", p, reviewAssignmentOpts(true, 2, "least-busy")); err != nil {
			t.Fatalf("HandlePREventReviewAssignment() error = %v", err)
		}
		// Round-robin fallback: still picks the first Count after sort.
		got := ghc.ReviewersRequested[0]
		if !slices.Equal(got.Users, []string{"alice", "bob"}) {
			t.Errorf("RequestReviewers users = %v, want [alice bob] (sorted, first Count)", got.Users)
		}
	})
	if !strings.Contains(out, "least-busy strategy not yet implemented") {
		t.Errorf("expected INFO log about least-busy fallback, got: %q", out)
	}
}
