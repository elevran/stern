package config_test

import (
	"os"
	"strings"
	"testing"

	"github.com/elevran/stern/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeFile(path string, data []byte) error {
	return os.WriteFile(path, data, 0o644) // #nosec G306 -- test scratch file
}

func TestGenerate_ParsesBack(t *testing.T) {
	data, err := config.Generate("testorg", "testrepo")
	require.NoError(t, err)
	require.NotEmpty(t, data, "Generate() returned empty output")

	// Parse via LoadFromFile to exercise the full path including applyDefaults.
	tmpFile := t.TempDir() + "/stern.yaml"
	require.NoError(t, writeFile(tmpFile, data))
	opts, err := config.LoadFromFile(tmpFile)
	require.NoError(t, err)

	assert.Empty(t, opts.Validate(), "Validate() returned errors for generated config")
	assert.Equal(t, "testorg", opts.Org)
	assert.Equal(t, "testrepo", opts.Repo)
}

func TestGenerate_ContainsAllPlugins(t *testing.T) {
	data, err := config.Generate("org", "repo")
	require.NoError(t, err)
	s := string(data)
	plugins := []string{"lgtm", "approve", "hold", "wip", "cherry-pick", "review_assignment", "size", "lifecycle"}
	for _, p := range plugins {
		assert.True(t, strings.Contains(s, p), "generated config does not mention plugin %q", p)
	}
}

func TestOrgRepoFromGitHubRepository(t *testing.T) {
	t.Run("flags take precedence", func(t *testing.T) {
		t.Setenv("GITHUB_REPOSITORY", "envorg/envrepo")
		org, repo := config.OrgRepoFromGitHubRepository("flagorg", "flagrepo")
		assert.Equal(t, "flagorg", org)
		assert.Equal(t, "flagrepo", repo)
	})
	t.Run("env fallback", func(t *testing.T) {
		t.Setenv("GITHUB_REPOSITORY", "envorg/envrepo")
		org, repo := config.OrgRepoFromGitHubRepository("", "")
		assert.Equal(t, "envorg", org)
		assert.Equal(t, "envrepo", repo)
	})
	t.Run("empty env", func(t *testing.T) {
		t.Setenv("GITHUB_REPOSITORY", "")
		org, repo := config.OrgRepoFromGitHubRepository("", "")
		assert.Equal(t, "", org)
		assert.Equal(t, "", repo)
	})
}