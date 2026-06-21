package owners_test

import (
	"bytes"
	"context"
	"io"
	"testing"

	"github.com/sirupsen/logrus"

	"github.com/elevran/stern/internal/github"
	"github.com/elevran/stern/internal/owners"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadForPaths_NoOwners(t *testing.T) {
	ghc := github.NewMockClient() // no files loaded
	result, err := owners.LoadForPaths(context.Background(), ghc, "o", "r", "abc123", []string{"pkg/foo.go"})
	require.NoError(t, err)
	assert.False(t, result.HasOwners(), "expected no owners when no OWNERS files exist")
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
	require.NoError(t, err)
	assert.True(t, result.IsApprover("alice"), "expected alice to be an approver")
	assert.True(t, result.IsApprover("bob"), "expected bob to be an approver")
	assert.True(t, result.IsReviewer("charlie"), "expected charlie to be a reviewer")
}

func TestLoadForPaths_DirectoryOwners(t *testing.T) {
	ghc := github.NewMockClient()
	ghc.FileContent["pkg/OWNERS@abc123"] = []byte(`
approvers:
  - alice
`)
	result, err := owners.LoadForPaths(context.Background(), ghc, "o", "r", "abc123", []string{"pkg/foo.go"})
	require.NoError(t, err)
	assert.True(t, result.IsApprover("alice"), "expected alice to be an approver via pkg/OWNERS")
}

func TestLoadForPaths_WalksHierarchy(t *testing.T) {
	ghc := github.NewMockClient()
	// Root OWNERS has alice; pkg/sub/ OWNERS has bob.
	// A file in pkg/sub/ should find both.
	ghc.FileContent["OWNERS@sha"] = []byte("approvers:\n  - alice\n")
	ghc.FileContent["pkg/sub/OWNERS@sha"] = []byte("approvers:\n  - bob\n")

	result, err := owners.LoadForPaths(context.Background(), ghc, "o", "r", "sha", []string{"pkg/sub/bar.go"})
	require.NoError(t, err)
	assert.True(t, result.IsApprover("alice"), "expected alice from root OWNERS")
	assert.True(t, result.IsApprover("bob"), "expected bob from pkg/sub/OWNERS")
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
	require.NoError(t, err)
	assert.True(t, result.IsApprover("alice"), "expected alice via alias expansion")
	assert.True(t, result.IsApprover("bob"), "expected bob via alias expansion")
}

func TestLoadForPaths_CaseInsensitive(t *testing.T) {
	ghc := github.NewMockClient()
	ghc.FileContent["OWNERS@sha"] = []byte("approvers:\n  - Alice\n")

	result, err := owners.LoadForPaths(context.Background(), ghc, "o", "r", "sha", []string{"foo.go"})
	require.NoError(t, err)
	assert.True(t, result.IsApprover("alice"), "IsApprover should be case-insensitive")
	assert.True(t, result.IsApprover("ALICE"), "IsApprover should be case-insensitive")
}

func TestLoadForPaths_RejectsDotDotPath(t *testing.T) {
	ghc := github.NewMockClient()
	// Place an OWNERS file where the traversal would land.
	ghc.FileContent["OWNERS@sha"] = []byte("approvers:\n  - attacker\n")
	ghc.FileContent["../../admin/OWNERS@sha"] = []byte("approvers:\n  - attacker\n")

	result, err := owners.LoadForPaths(context.Background(), ghc, "o", "r", "sha",
		[]string{"../../admin/foo.go", "../secret.go"})
	require.NoError(t, err)
	assert.False(t, result.HasOwners(), "expected path traversal paths to be rejected, got owners")
}

func TestLoadForPaths_RejectsAbsolutePath(t *testing.T) {
	ghc := github.NewMockClient()
	ghc.FileContent["OWNERS@sha"] = []byte("approvers:\n  - attacker\n")

	result, err := owners.LoadForPaths(context.Background(), ghc, "o", "r", "sha",
		[]string{"/etc/passwd"})
	require.NoError(t, err)
	assert.False(t, result.HasOwners(), "expected absolute path to be rejected")
}

func TestLoadForPaths_NormalPathStillWorks(t *testing.T) {
	ghc := github.NewMockClient()
	ghc.FileContent["OWNERS@sha"] = []byte("approvers:\n  - alice\n")

	result, err := owners.LoadForPaths(context.Background(), ghc, "o", "r", "sha",
		[]string{"pkg/foo.go", "internal/bar/baz.go"})
	require.NoError(t, err)
	assert.True(t, result.IsApprover("alice"), "expected alice from root OWNERS for normal paths")
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
	require.NoError(t, err)
	assert.Equal(t, []string{"alice", "mike", "zoe"}, result.Approvers, "Approvers should be sorted ascending")
	assert.Equal(t, []string{"bob", "yara"}, result.Reviewers, "Reviewers should be sorted ascending")
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
	require.NoError(t, err, "LoadForPaths() should not return an error for malformed YAML (warning, not error)")
	assert.Contains(t, out, "level=warning", "expected warning level in log output")
	assert.Contains(t, out, "OWNERS file exists but could not be parsed", "expected parse warning message in log output")
	assert.Contains(t, out, "path=pkg/OWNERS", "expected path=pkg/OWNERS in log output")
	assert.True(t, result.IsApprover("alice"), "expected alice from the valid root OWNERS file to still be present")
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
	require.NoError(t, err, "LoadForPaths() should not return an error for malformed YAML")
	assert.True(t, result.IsApprover("bob"), "expected bob from pkg/OWNERS to still resolve despite root OWNERS being malformed")
}

