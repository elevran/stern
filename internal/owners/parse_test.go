package owners_test

import (
	"context"
	"testing"

	ghclient "github.com/elevran/stern/internal/github"
	"github.com/elevran/stern/internal/owners"
)

func TestLoadForPaths_NoOwners(t *testing.T) {
	ghc := ghclient.NewMockClient() // no files loaded
	result, err := owners.LoadForPaths(context.Background(), ghc, "o", "r", "abc123", []string{"pkg/foo.go"})
	if err != nil {
		t.Fatalf("LoadForPaths() error = %v", err)
	}
	if result.HasOwners() {
		t.Error("expected no owners when no OWNERS files exist")
	}
}

func TestLoadForPaths_RootOwners(t *testing.T) {
	ghc := ghclient.NewMockClient()
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
	ghc := ghclient.NewMockClient()
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
	ghc := ghclient.NewMockClient()
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
	ghc := ghclient.NewMockClient()
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
	ghc := ghclient.NewMockClient()
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
	ghc := ghclient.NewMockClient()
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
	ghc := ghclient.NewMockClient()
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
	ghc := ghclient.NewMockClient()
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
