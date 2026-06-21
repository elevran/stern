package event

import (
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseCommentEvent(t *testing.T) {
	_, file, _, _ := runtime.Caller(0)
	testdata := filepath.Join(filepath.Dir(file), "..", "..", "testdata", "comment.json")
	t.Setenv("GITHUB_EVENT_PATH", testdata)

	evt, err := ParseCommentEvent()
	require.NoError(t, err)
	require.NotNil(t, evt.Comment)
	assert.Equal(t, "/ping\n/hello world", evt.Comment.GetBody())
	assert.Equal(t, int64(12345), evt.Comment.GetID())
	require.NotNil(t, evt.Issue)
	assert.Equal(t, 1, evt.Issue.GetNumber())
}

func TestContextFromComment(t *testing.T) {
	_, file, _, _ := runtime.Caller(0)
	testdata := filepath.Join(filepath.Dir(file), "..", "..", "testdata", "comment.json")
	t.Setenv("GITHUB_EVENT_PATH", testdata)

	evt, err := ParseCommentEvent()
	require.NoError(t, err)

	sc := ContextFromComment(evt, "elevran", "stern", "github-actions[bot]")
	assert.Equal(t, "elevran", sc.Org)
	assert.Equal(t, "stern", sc.Repo)
	assert.Equal(t, int64(12345), sc.CommentID)
	assert.Equal(t, "testuser", sc.Author)
	assert.Equal(t, 1, sc.IssueNumber)
}

func TestOrgRepoFromEnv(t *testing.T) {
	t.Setenv("GITHUB_REPOSITORY", "elevran/stern")
	org, repo, err := OrgRepoFromEnv()
	require.NoError(t, err)
	assert.Equal(t, "elevran", org)
	assert.Equal(t, "stern", repo)
}

func TestOrgRepoFromEnv_Missing(t *testing.T) {
	t.Setenv("GITHUB_REPOSITORY", "")
	_, _, err := OrgRepoFromEnv()
	assert.Error(t, err)
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
			assert.Equal(t, c.want, IsBot(c.sender, c.botLogin))
		})
	}
}

func TestParsePREvent(t *testing.T) {
	_, file, _, _ := runtime.Caller(0)
	testdata := filepath.Join(filepath.Dir(file), "testdata", "pull_request_opened.json")
	t.Setenv("GITHUB_EVENT_PATH", testdata)

	evt, err := ParsePREvent()
	require.NoError(t, err)
	assert.Equal(t, "opened", evt.GetAction())
	assert.Equal(t, 42, evt.GetNumber())
	assert.Equal(t, "alice", evt.GetSender().GetLogin())
	pr := evt.GetPullRequest()
	require.NotNil(t, pr)
	assert.Equal(t, "Add IsBot helper", pr.GetTitle())
}

func TestParseIssueEvent(t *testing.T) {
	_, file, _, _ := runtime.Caller(0)
	testdata := filepath.Join(filepath.Dir(file), "testdata", "issues_opened.json")
	t.Setenv("GITHUB_EVENT_PATH", testdata)

	evt, err := ParseIssueEvent()
	require.NoError(t, err)
	assert.Equal(t, "opened", evt.GetAction())
	issue := evt.GetIssue()
	require.NotNil(t, issue)
	assert.Equal(t, 7, issue.GetNumber())
	assert.Equal(t, "Test issue", issue.GetTitle())
	assert.Equal(t, "bob", evt.GetSender().GetLogin())
}

func TestActionsRunURL(t *testing.T) {
	t.Run("both set on github.com", func(t *testing.T) {
		t.Setenv("GITHUB_SERVER_URL", "https://github.com")
		t.Setenv("GITHUB_REPOSITORY", "elevran/stern")
		t.Setenv("GITHUB_RUN_ID", "12345")

		assert.Equal(t, "https://github.com/elevran/stern/actions/runs/12345", ActionsRunURL())
	})

	t.Run("custom server URL", func(t *testing.T) {
		t.Setenv("GITHUB_SERVER_URL", "https://github.example.com")
		t.Setenv("GITHUB_REPOSITORY", "elevran/stern")
		t.Setenv("GITHUB_RUN_ID", "99")

		assert.Equal(t, "https://github.example.com/elevran/stern/actions/runs/99", ActionsRunURL())
	})

	t.Run("server URL defaults when unset", func(t *testing.T) {
		t.Setenv("GITHUB_SERVER_URL", "")
		t.Setenv("GITHUB_REPOSITORY", "elevran/stern")
		t.Setenv("GITHUB_RUN_ID", "1")

		assert.Equal(t, "https://github.com/elevran/stern/actions/runs/1", ActionsRunURL())
	})

	t.Run("repo missing returns empty", func(t *testing.T) {
		t.Setenv("GITHUB_SERVER_URL", "https://github.com")
		t.Setenv("GITHUB_REPOSITORY", "")
		t.Setenv("GITHUB_RUN_ID", "1")

		assert.Empty(t, ActionsRunURL())
	})

	t.Run("run ID missing returns empty", func(t *testing.T) {
		t.Setenv("GITHUB_SERVER_URL", "https://github.com")
		t.Setenv("GITHUB_REPOSITORY", "elevran/stern")
		t.Setenv("GITHUB_RUN_ID", "")

		assert.Empty(t, ActionsRunURL())
	})
}

