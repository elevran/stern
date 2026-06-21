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

func TestIsBot(t *testing.T) {
	cases := []struct {
		name     string
		sender   string
		botLogin string
		want     bool
	}{
		{"github-actions app suffix", "github-actions[bot]", "stern-bot", true},
		{"exact bot login", "stern-bot", "stern-bot", true},
		{"non-bot user", "alice", "stern-bot", false},
		{"user named 'alicebot' is not a bot", "alicebot", "stern-bot", false},
		{"empty sender", "", "stern-bot", false},
		{"empty bot login with bot suffix", "github-actions[bot]", "", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := IsBot(c.sender, c.botLogin); got != c.want {
				t.Errorf("IsBot(%q, %q) = %v, want %v", c.sender, c.botLogin, got, c.want)
			}
		})
	}
}

func TestParsePREvent(t *testing.T) {
	_, file, _, _ := runtime.Caller(0)
	testdata := filepath.Join(filepath.Dir(file), "testdata", "pull_request_opened.json")
	t.Setenv("GITHUB_EVENT_PATH", testdata)

	evt, err := ParsePREvent()
	if err != nil {
		t.Fatalf("ParsePREvent() error = %v", err)
	}
	if got := evt.GetAction(); got != "opened" {
		t.Errorf("action = %q, want %q", got, "opened")
	}
	if got := evt.GetNumber(); got != 42 {
		t.Errorf("number = %d, want 42", got)
	}
	if got := evt.GetSender().GetLogin(); got != "alice" {
		t.Errorf("sender login = %q, want alice", got)
	}
	pr := evt.GetPullRequest()
	if pr == nil {
		t.Fatal("expected PullRequest to be non-nil")
	}
	if got := pr.GetTitle(); got != "Add IsBot helper" {
		t.Errorf("PR title = %q, want %q", got, "Add IsBot helper")
	}
}

func TestParseIssueEvent(t *testing.T) {
	_, file, _, _ := runtime.Caller(0)
	testdata := filepath.Join(filepath.Dir(file), "testdata", "issues_opened.json")
	t.Setenv("GITHUB_EVENT_PATH", testdata)

	evt, err := ParseIssueEvent()
	if err != nil {
		t.Fatalf("ParseIssueEvent() error = %v", err)
	}
	if got := evt.GetAction(); got != "opened" {
		t.Errorf("action = %q, want %q", got, "opened")
	}
	issue := evt.GetIssue()
	if issue == nil {
		t.Fatal("expected Issue to be non-nil")
	}
	if got := issue.GetNumber(); got != 7 {
		t.Errorf("issue number = %d, want 7", got)
	}
	if got := issue.GetTitle(); got != "Test issue" {
		t.Errorf("issue title = %q, want %q", got, "Test issue")
	}
	if got := evt.GetSender().GetLogin(); got != "bob" {
		t.Errorf("sender login = %q, want bob", got)
	}
}

func TestActionsRunURL(t *testing.T) {
	t.Run("both set on github.com", func(t *testing.T) {
		t.Setenv("GITHUB_SERVER_URL", "https://github.com")
		t.Setenv("GITHUB_REPOSITORY", "elevran/stern")
		t.Setenv("GITHUB_RUN_ID", "12345")

		if got, want := ActionsRunURL(), "https://github.com/elevran/stern/actions/runs/12345"; got != want {
			t.Errorf("ActionsRunURL() = %q, want %q", got, want)
		}
	})

	t.Run("custom server URL", func(t *testing.T) {
		t.Setenv("GITHUB_SERVER_URL", "https://github.example.com")
		t.Setenv("GITHUB_REPOSITORY", "elevran/stern")
		t.Setenv("GITHUB_RUN_ID", "99")

		if got, want := ActionsRunURL(), "https://github.example.com/elevran/stern/actions/runs/99"; got != want {
			t.Errorf("ActionsRunURL() = %q, want %q", got, want)
		}
	})

	t.Run("server URL defaults when unset", func(t *testing.T) {
		t.Setenv("GITHUB_SERVER_URL", "")
		t.Setenv("GITHUB_REPOSITORY", "elevran/stern")
		t.Setenv("GITHUB_RUN_ID", "1")

		if got, want := ActionsRunURL(), "https://github.com/elevran/stern/actions/runs/1"; got != want {
			t.Errorf("ActionsRunURL() = %q, want %q", got, want)
		}
	})

	t.Run("repo missing returns empty", func(t *testing.T) {
		t.Setenv("GITHUB_SERVER_URL", "https://github.com")
		t.Setenv("GITHUB_REPOSITORY", "")
		t.Setenv("GITHUB_RUN_ID", "1")

		if got := ActionsRunURL(); got != "" {
			t.Errorf("ActionsRunURL() = %q, want empty", got)
		}
	})

	t.Run("run ID missing returns empty", func(t *testing.T) {
		t.Setenv("GITHUB_SERVER_URL", "https://github.com")
		t.Setenv("GITHUB_REPOSITORY", "elevran/stern")
		t.Setenv("GITHUB_RUN_ID", "")

		if got := ActionsRunURL(); got != "" {
			t.Errorf("ActionsRunURL() = %q, want empty", got)
		}
	})
}
