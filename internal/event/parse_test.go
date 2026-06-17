package event

import (
	"path/filepath"
	"runtime"
	"testing"
)

func TestParseCommentEvent(t *testing.T) {
	_, file, _, _ := runtime.Caller(0)
	testdata := filepath.Join(filepath.Dir(file), "..", "..", "testdata", "comment.json")
	t.Setenv("GITHUB_EVENT_PATH", testdata)

	evt, err := ParseCommentEvent()
	if err != nil {
		t.Fatalf("ParseCommentEvent() error = %v", err)
	}
	if evt.Comment == nil {
		t.Fatal("expected Comment to be non-nil")
	}
	if got := evt.Comment.GetBody(); got != "/ping\n/hello world" {
		t.Errorf("body = %q, want %q", got, "/ping\n/hello world")
	}
	if got := evt.Comment.GetID(); got != 12345 {
		t.Errorf("comment ID = %d, want 12345", got)
	}
	if evt.Issue == nil {
		t.Fatal("expected Issue to be non-nil")
	}
	if got := evt.Issue.GetNumber(); got != 1 {
		t.Errorf("issue number = %d, want 1", got)
	}
}

func TestContextFromComment(t *testing.T) {
	_, file, _, _ := runtime.Caller(0)
	testdata := filepath.Join(filepath.Dir(file), "..", "..", "testdata", "comment.json")
	t.Setenv("GITHUB_EVENT_PATH", testdata)

	evt, err := ParseCommentEvent()
	if err != nil {
		t.Fatalf("ParseCommentEvent() error = %v", err)
	}

	sc := ContextFromComment(evt, "elevran", "stern", "github-actions[bot]")
	if sc.Org != "elevran" {
		t.Errorf("Org = %q, want %q", sc.Org, "elevran")
	}
	if sc.Repo != "stern" {
		t.Errorf("Repo = %q, want %q", sc.Repo, "stern")
	}
	if sc.CommentID != 12345 {
		t.Errorf("CommentID = %d, want 12345", sc.CommentID)
	}
	if sc.Author != "testuser" {
		t.Errorf("Author = %q, want %q", sc.Author, "testuser")
	}
	if sc.IssueNumber != 1 {
		t.Errorf("IssueNumber = %d, want 1", sc.IssueNumber)
	}
}

func TestOrgRepoFromEnv(t *testing.T) {
	t.Setenv("GITHUB_REPOSITORY", "elevran/stern")
	org, repo, err := OrgRepoFromEnv()
	if err != nil {
		t.Fatalf("OrgRepoFromEnv() error = %v", err)
	}
	if org != "elevran" || repo != "stern" {
		t.Errorf("got %q/%q, want elevran/stern", org, repo)
	}
}

func TestOrgRepoFromEnv_Missing(t *testing.T) {
	t.Setenv("GITHUB_REPOSITORY", "")
	_, _, err := OrgRepoFromEnv()
	if err == nil {
		t.Fatal("expected error when GITHUB_REPOSITORY is empty")
	}
}
