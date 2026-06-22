package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	gh "github.com/google/go-github/v72/github"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elevran/stern/internal/config"
	"github.com/elevran/stern/internal/github"
)

// withGlobals saves and restores the package-level mutable state touched
// by the RunE functions (globalOpts, dryRun). Tests use this to avoid
// leaking state between cases.
func withGlobals(t *testing.T) {
	t.Helper()
	origOpts := globalOpts
	origDryRun := dryRun
	globalOpts = &config.Options{}
	dryRun = false
	t.Cleanup(func() {
		globalOpts = origOpts
		dryRun = origDryRun
	})
}

// writeEventJSON marshals payload to a temp file and sets GITHUB_EVENT_PATH
// so the parse* functions pick it up. Used for pull_request_target and issues
// event payloads.
func writeEventJSON(t *testing.T, payload any) {
	t.Helper()
	data, err := json.Marshal(payload)
	require.NoError(t, err, "marshal event")
	dir := t.TempDir()
	path := filepath.Join(dir, "event.json")
	require.NoError(t, os.WriteFile(path, data, 0o600), "write event")
	t.Setenv("GITHUB_EVENT_PATH", path)
}

// prOpenedEvent returns a minimal pull_request_target "opened" event payload
// for runPREvent tests. PR has a clean state so handleWIP is a no-op and
// the dispatch wiring is what the test exercises.
func prOpenedEvent() *gh.PullRequestEvent {
	return &gh.PullRequestEvent{
		Action: gh.Ptr("opened"),
		Sender: &gh.User{Login: gh.Ptr("alice")},
		PullRequest: &gh.PullRequest{
			Number:    gh.Ptr(1),
			Title:     gh.Ptr("fix bug"),
			Draft:     gh.Ptr(false),
			Additions: gh.Ptr(5),
			Deletions: gh.Ptr(2),
			Head:      &gh.PullRequestBranch{SHA: gh.Ptr("sha1")},
			Labels:    []*gh.Label{},
		},
	}
}

func TestRunPREvent_HappyPath(t *testing.T) {
	withGlobals(t)
	t.Setenv("GITHUB_REPOSITORY", "elevran/stern")
	writeEventJSON(t, prOpenedEvent())

	ghc := github.NewMockClient()
	require.NoError(t, runPREvent(ghc))
}

func TestRunPREvent_BotSkipped(t *testing.T) {
	withGlobals(t)
	t.Setenv("GITHUB_REPOSITORY", "elevran/stern")
	globalOpts.BotLogin = "github-actions[bot]"
	writeEventJSON(t, prOpenedEvent())

	ghc := github.NewMockClient()
	require.NoError(t, runPREvent(ghc))
	assert.Empty(t, ghc.IssueLabels, "expected no label mutations when sender is a bot")
}

func TestRunPREvent_NoOwnersCacheFile(t *testing.T) {
	withGlobals(t)
	t.Setenv("GITHUB_REPOSITORY", "elevran/stern")
	// No OWNERS_CACHE_FILE — should be a no-op, not an error.
	writeEventJSON(t, prOpenedEvent())

	ghc := github.NewMockClient()
	require.NoError(t, runPREvent(ghc))
}

func TestRunPREvent_BadEventPath(t *testing.T) {
	withGlobals(t)
	// GITHUB_EVENT_PATH unset → ParsePREvent returns error → runPREvent returns error.
	require.Error(t, runPREvent(github.NewMockClient()))
}

func TestRunLifecycleCmd_DisabledNoOp(t *testing.T) {
	withGlobals(t)
	globalOpts.Lifecycle.Enabled = false

	require.NoError(t, runLifecycleCmd(nil, nil))
}

func TestRunLifecycleCmd_MissingOrgRepo(t *testing.T) {
	withGlobals(t)
	globalOpts.Lifecycle.Enabled = true
	// globalOpts.Org / Repo both empty → lifecycleOrgRepo() returns error.
	// Set a token so buildClient succeeds and we get past that step.
	t.Setenv("GITHUB_TOKEN", "fake")

	err := runLifecycleCmd(nil, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "org/repo not set")
}

func TestRunLifecycle_HappyPath(t *testing.T) {
	withGlobals(t)
	globalOpts.Org = "elevran"
	globalOpts.Repo = "stern"
	globalOpts.Lifecycle.Enabled = true
	globalOpts.Lifecycle.StaleDays = 1

	ghc := github.NewMockClient()
	// An item whose UpdatedAt is older than the stale threshold. now is
	// fixed so the test is deterministic.
	ghc.Items = []github.Item{{
		Number:    7,
		Labels:    []string{},
		UpdatedAt: time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC),
	}}

	require.NoError(t, runLifecycle(ghc, time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)))

	assert.True(t, ghc.IssueLabels[7]["lifecycle/stale"], "expected stale label applied to a sufficiently old item")
}

func TestRunIssueEvent_Parses(t *testing.T) {
	withGlobals(t)
	// Minimal issues-event payload. runIssueEvent only parses and logs.
	writeEventJSON(t, &gh.IssuesEvent{
		Action: gh.Ptr("opened"),
		Sender: &gh.User{Login: gh.Ptr("alice")},
		Issue: &gh.Issue{
			Number: gh.Ptr(1),
			Title:  gh.Ptr("Test issue"),
		},
	})

	require.NoError(t, runIssueEvent(nil, nil))
}

func TestRunIssueEvent_BadEventPath(t *testing.T) {
	withGlobals(t)
	// GITHUB_EVENT_PATH unset → ParseIssueEvent returns error.
	require.Error(t, runIssueEvent(nil, nil))
}

func TestRunMergeCheck_StubNoOp(t *testing.T) {
	require.NoError(t, runMergeCheck(nil, nil))
}

func TestBuildClient_NoToken(t *testing.T) {
	// t.Setenv to empty string matches NewFromEnv's "" check.
	t.Setenv("GITHUB_TOKEN", "")

	_, err := buildClient()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "GITHUB_TOKEN")
}

func TestBuildClient_WithToken(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "fake")
	origDryRun := dryRun
	dryRun = false
	t.Cleanup(func() { dryRun = origDryRun })

	ghc, err := buildClient()
	require.NoError(t, err)
	assert.NotNil(t, ghc)
}
