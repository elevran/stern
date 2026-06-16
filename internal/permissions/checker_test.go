package permissions_test

import (
	"context"
	"testing"

	gh "github.com/google/go-github/v72/github"

	"github.com/elevran/stern/internal/event"
	ghclient "github.com/elevran/stern/internal/github"
	"github.com/elevran/stern/internal/permissions"
)

func newChecker(ghc *ghclient.MockClient) permissions.Checker {
	sc := &event.Context{
		Org:      "elevran",
		Repo:     "stern",
		BotLogin: "github-actions[bot]",
	}
	return permissions.New(ghc, sc)
}

func TestIsOrgMember(t *testing.T) {
	tests := []struct {
		name    string
		user    string
		members map[string]bool
		want    bool
	}{
		{"member", "alice", map[string]bool{"elevran/alice": true}, true},
		{"non-member", "bob", map[string]bool{"elevran/alice": true}, false},
		{"empty map", "alice", nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ghc := ghclient.NewMockClient()
			ghc.OrgMembers = tt.members
			chk := newChecker(ghc)
			got, err := chk.IsOrgMember(context.Background(), "elevran", tt.user)
			if err != nil {
				t.Fatalf("IsOrgMember() error = %v", err)
			}
			if got != tt.want {
				t.Errorf("IsOrgMember() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestHasWriteAccess(t *testing.T) {
	tests := []struct {
		name        string
		user        string
		writeAccess map[string]bool
		want        bool
	}{
		{"has write", "alice", map[string]bool{"elevran/stern/alice": true}, true},
		{"no write", "bob", map[string]bool{"elevran/stern/alice": true}, false},
		{"empty map", "alice", nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ghc := ghclient.NewMockClient()
			ghc.WriteAccess = tt.writeAccess
			chk := newChecker(ghc)
			got, err := chk.HasWriteAccess(context.Background(), "elevran", "stern", tt.user)
			if err != nil {
				t.Fatalf("HasWriteAccess() error = %v", err)
			}
			if got != tt.want {
				t.Errorf("HasWriteAccess() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsPRAuthor(t *testing.T) {
	pr := &gh.PullRequest{
		User: &gh.User{Login: gh.Ptr("alice")},
	}
	tests := []struct {
		name string
		pr   *gh.PullRequest
		user string
		want bool
	}{
		{"author", pr, "alice", true},
		{"not author", pr, "bob", false},
		{"nil PR", nil, "alice", false},
		{"nil PR user", &gh.PullRequest{}, "alice", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ghc := ghclient.NewMockClient()
			chk := newChecker(ghc)
			got := chk.IsPRAuthor(tt.pr, tt.user)
			if got != tt.want {
				t.Errorf("IsPRAuthor() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsBot(t *testing.T) {
	tests := []struct {
		name  string
		user  string
		want  bool
	}{
		{"is bot", "github-actions[bot]", true},
		{"is not bot", "alice", false},
		{"empty string", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ghc := ghclient.NewMockClient()
			chk := newChecker(ghc)
			got := chk.IsBot(tt.user)
			if got != tt.want {
				t.Errorf("IsBot(%q) = %v, want %v", tt.user, got, tt.want)
			}
		})
	}
}

func TestIsOrgMember_Error(t *testing.T) {
	ghc := ghclient.NewMockClient()
	ghc.Errors["IsOrgMember"] = context.DeadlineExceeded
	chk := newChecker(ghc)
	_, err := chk.IsOrgMember(context.Background(), "elevran", "alice")
	if err == nil {
		t.Fatal("expected error from IsOrgMember")
	}
}

func TestHasWriteAccess_Error(t *testing.T) {
	ghc := ghclient.NewMockClient()
	ghc.Errors["HasWriteAccess"] = context.DeadlineExceeded
	chk := newChecker(ghc)
	_, err := chk.HasWriteAccess(context.Background(), "elevran", "stern", "alice")
	if err == nil {
		t.Fatal("expected error from HasWriteAccess")
	}
}
