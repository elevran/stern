package owners_test

import (
	"bytes"
	"context"
	"io"
	"slices"
	"strings"
	"testing"

	"github.com/sirupsen/logrus"

	"github.com/elevran/stern/internal/github"
	"github.com/elevran/stern/internal/owners"
)

func TestLoadForPaths_NoOwners(t *testing.T) {
	ghc := github.NewMockClient() // no files loaded
	result, err := owners.LoadForPaths(context.Background(), ghc, "o", "r", "abc123", []string{"pkg/foo.go"})
	if err != nil {
		t.Fatalf("LoadForPaths() error = %v", err)
	}
	if result.HasOwners() {
		t.Error("expected no owners when no OWNERS files exist")
	}
}

func TestLoadForPaths_RootOwners(t *testing.T) {
	ghc := github.NewMockClient()
	ghc.FileContent["OWNERS@abc123"] = []byte(`
approvers:
  - alice
  - bob
reviewers:
  - charlie
`)
	result, err := owners.LoadForPaths(context.Background(), ghc, "o", "r", "abc123", []string{"README.md"})
	if err != nil {
		t.Fatalf("LoadForPaths() error = %v", err)
	}
	if !result.IsApprover("alice") {
		t.Error("expected alice to be an approver")
	}
	if !result.IsApprover("bob") {
		t.Error("expected bob to be an approver")
	}
	if !result.IsReviewer("charlie") {
		t.Error("expected charlie to be a reviewer")
	}
}

func TestLoadForPaths_DirectoryOwners(t *testing.T) {
	ghc := github.NewMockClient()
	ghc.FileContent["pkg/OWNERS@abc123"] = []byte(`
approvers:
  - alice
`)
	result, err := owners.LoadForPaths(context.Background(), ghc, "o", "r", "abc123", []string{"pkg/foo.go"})
	if err != nil {
		t.Fatalf("LoadForPaths() error = %v", err)
	}
	if !result.IsApprover("alice") {
		t.Error("expected alice to be an approver via pkg/OWNERS")
	}
}

func TestLoadForPaths_WalksHierarchy(t *testing.T) {
	ghc := github.NewMockClient()
	// Root OWNERS has alice; pkg/sub/ OWNERS has bob.
	// A file in pkg/sub/ should find both.
	ghc.FileContent["OWNERS@sha"] = []byte("approvers:\n  - alice\n")
	ghc.FileContent["pkg/sub/OWNERS@sha"] = []byte("approvers:\n  - bob\n")

	result, err := owners.LoadForPaths(context.Background(), ghc, "o", "r", "sha", []string{"pkg/sub/bar.go"})
	if err != nil {
		t.Fatalf("LoadForPaths() error = %v", err)
	}
	if !result.IsApprover("alice") {
		t.Error("expected alice from root OWNERS")
	}
	if !result.IsApprover("bob") {
		t.Error("expected bob from pkg/sub/OWNERS")
	}
}

func TestLoadForPaths_AliasExpansion(t *testing.T) {
	ghc := github.NewMockClient()
	ghc.FileContent["OWNERS_ALIASES@sha"] = []byte(`
aliases:
  team-eng:
    - alice
    - bob
`)
	ghc.FileContent["OWNERS@sha"] = []byte("approvers:\n  - team-eng\n")

	result, err := owners.LoadForPaths(context.Background(), ghc, "o", "r", "sha", []string{"main.go"})
	if err != nil {
		t.Fatalf("LoadForPaths() error = %v", err)
	}
	if !result.IsApprover("alice") {
		t.Error("expected alice via alias expansion")
	}
	if !result.IsApprover("bob") {
		t.Error("expected bob via alias expansion")
	}
}

func TestLoadForPaths_CaseInsensitive(t *testing.T) {
	ghc := github.NewMockClient()
	ghc.FileContent["OWNERS@sha"] = []byte("approvers:\n  - Alice\n")

	result, err := owners.LoadForPaths(context.Background(), ghc, "o", "r", "sha", []string{"foo.go"})
	if err != nil {
		t.Fatalf("LoadForPaths() error = %v", err)
	}
	if !result.IsApprover("alice") {
		t.Error("IsApprover should be case-insensitive")
	}
	if !result.IsApprover("ALICE") {
		t.Error("IsApprover should be case-insensitive")
	}
}

func TestLoadForPaths_RejectsDotDotPath(t *testing.T) {
	ghc := github.NewMockClient()
	// Place an OWNERS file where the traversal would land.
	ghc.FileContent["OWNERS@sha"] = []byte("approvers:\n  - attacker\n")
	ghc.FileContent["../../admin/OWNERS@sha"] = []byte("approvers:\n  - attacker\n")

	result, err := owners.LoadForPaths(context.Background(), ghc, "o", "r", "sha",
		[]string{"../../admin/foo.go", "../secret.go"})
	if err != nil {
		t.Fatalf("LoadForPaths() error = %v", err)
	}
	if result.HasOwners() {
		t.Error("expected path traversal paths to be rejected, got owners")
	}
}

func TestLoadForPaths_RejectsAbsolutePath(t *testing.T) {
	ghc := github.NewMockClient()
	ghc.FileContent["OWNERS@sha"] = []byte("approvers:\n  - attacker\n")

	result, err := owners.LoadForPaths(context.Background(), ghc, "o", "r", "sha",
		[]string{"/etc/passwd"})
	if err != nil {
		t.Fatalf("LoadForPaths() error = %v", err)
	}
	if result.HasOwners() {
		t.Error("expected absolute path to be rejected")
	}
}

func TestLoadForPaths_NormalPathStillWorks(t *testing.T) {
	ghc := github.NewMockClient()
	ghc.FileContent["OWNERS@sha"] = []byte("approvers:\n  - alice\n")

	result, err := owners.LoadForPaths(context.Background(), ghc, "o", "r", "sha",
		[]string{"pkg/foo.go", "internal/bar/baz.go"})
	if err != nil {
		t.Fatalf("LoadForPaths() error = %v", err)
	}
	if !result.IsApprover("alice") {
		t.Error("expected alice from root OWNERS for normal paths")
	}
}

// TestLoadForPaths_SortedApproversAndReviewers verifies that the Approvers and
// Reviewers slices are returned in ascending sorted order regardless of the
// order in which the OWNERS file was read or the order in which approvers
// were added to the set. The behavior was changed in #78 to use
// slices.Sorted(maps.Keys(...)) for deterministic ordering.
func TestLoadForPaths_SortedApproversAndReviewers(t *testing.T) {
	ghc := github.NewMockClient()
	// Intentionally list approvers/reviewers in non-alphabetical order so
	// unsorted output would not match the expected result.
	ghc.FileContent["OWNERS@sha"] = []byte(`approvers:
  - zoe
  - alice
  - mike
reviewers:
  - yara
  - bob
`)

	result, err := owners.LoadForPaths(context.Background(), ghc, "o", "r", "sha", []string{"main.go"})
	if err != nil {
		t.Fatalf("LoadForPaths() error = %v", err)
	}
	wantApprovers := []string{"alice", "mike", "zoe"}
	if got := result.Approvers; !slices.Equal(got, wantApprovers) {
		t.Errorf("Approvers = %v, want %v (sorted ascending)", got, wantApprovers)
	}
	wantReviewers := []string{"bob", "yara"}
	if got := result.Reviewers; !slices.Equal(got, wantReviewers) {
		t.Errorf("Reviewers = %v, want %v (sorted ascending)", got, wantReviewers)
	}
}

// captureLogger swaps logrus.StandardLogger().Out with a buffer for the
// duration of fn, restoring the original on return. Mirrors the helper in
// internal/pr/review_assignment_test.go.
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

// TestLoadForPaths_MalformedYAMLLogsWarning verifies that when an OWNERS file
// exists but cannot be parsed as YAML, LoadForPaths logs a warning and skips
// the file (does not return an error). This is the behavior change in #68.
func TestLoadForPaths_MalformedYAMLLogsWarning(t *testing.T) {
	ghc := github.NewMockClient()
	// Mixed valid and invalid YAML in the hierarchy: the bad file should
	// produce a warning and be skipped, while the good file's approver
	// should still appear in the result.
	ghc.FileContent["OWNERS@sha"] = []byte("approvers:\n  - alice\n")
	ghc.FileContent["pkg/OWNERS@sha"] = []byte("approvers:\n  - alice\n  : this is not valid yaml: : :\n")

	var out string
	var result *owners.ResolvedOwners
	var err error
	out = captureLogger(func() {
		result, err = owners.LoadForPaths(context.Background(), ghc, "o", "r", "sha", []string{"pkg/foo.go"})
	})
	if err != nil {
		t.Fatalf("LoadForPaths() error = %v, want nil (warning, not error)", err)
	}
	if !strings.Contains(out, "level=warning") {
		t.Errorf("expected warning level in log output, got: %q", out)
	}
	if !strings.Contains(out, "OWNERS file exists but could not be parsed") {
		t.Errorf("expected parse warning message in log output, got: %q", out)
	}
	if !strings.Contains(out, "path=pkg/OWNERS") {
		t.Errorf("expected path=pkg/OWNERS in log output, got: %q", out)
	}
	if !result.IsApprover("alice") {
		t.Error("expected alice from the valid root OWNERS file to still be present")
	}
}

// TestLoadForPaths_MalformedYAMLDoesNotBlockOtherFiles ensures the continue
// path is preserved: a bad OWNERS in one directory must not prevent the
// caller from getting owners from other directories in the hierarchy.
func TestLoadForPaths_MalformedYAMLDoesNotBlockOtherFiles(t *testing.T) {
	quietLogger()
	ghc := github.NewMockClient()
	// Bad OWNERS at the root, good OWNERS deeper in the tree.
	ghc.FileContent["OWNERS@sha"] = []byte(": : : not yaml : :\n")
	ghc.FileContent["pkg/OWNERS@sha"] = []byte("approvers:\n  - bob\n")

	result, err := owners.LoadForPaths(context.Background(), ghc, "o", "r", "sha", []string{"pkg/foo.go"})
	if err != nil {
		t.Fatalf("LoadForPaths() error = %v, want nil", err)
	}
	if !result.IsApprover("bob") {
		t.Error("expected bob from pkg/OWNERS to still resolve despite root OWNERS being malformed")
	}
}
