package config_test

import (
	"os"
	"strings"
	"testing"

	"github.com/elevran/stern/internal/config"
)

func writeFile(path string, data []byte) error {
	return os.WriteFile(path, data, 0o644) // #nosec G306 -- test scratch file
}

func TestGenerate_ParsesBack(t *testing.T) {
	data, err := config.Generate("testorg", "testrepo")
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if len(data) == 0 {
		t.Fatal("Generate() returned empty output")
	}

	// Parse via LoadFromFile to exercise the full path including applyDefaults.
	tmpFile := t.TempDir() + "/stern.yaml"
	if err := writeFile(tmpFile, data); err != nil {
		t.Fatalf("writing temp file: %v", err)
	}
	opts, err := config.LoadFromFile(tmpFile)
	if err != nil {
		t.Fatalf("LoadFromFile() error = %v", err)
	}

	if errs := opts.Validate(); len(errs) != 0 {
		t.Errorf("Validate() returned errors for generated config: %v", errs)
	}
	if opts.Org != "testorg" {
		t.Errorf("Org = %q, want testorg", opts.Org)
	}
	if opts.Repo != "testrepo" {
		t.Errorf("Repo = %q, want testrepo", opts.Repo)
	}
}

func TestGenerate_ContainsAllPlugins(t *testing.T) {
	data, err := config.Generate("org", "repo")
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	s := string(data)
	plugins := []string{"lgtm", "approve", "hold", "wip", "cherry-pick", "review_assignment", "size", "lifecycle"}
	for _, p := range plugins {
		if !strings.Contains(s, p) {
			t.Errorf("generated config does not mention plugin %q", p)
		}
	}
}

func TestOrgRepoFromGitHubRepository(t *testing.T) {
	t.Run("flags take precedence", func(t *testing.T) {
		t.Setenv("GITHUB_REPOSITORY", "envorg/envrepo")
		org, repo := config.OrgRepoFromGitHubRepository("flagorg", "flagrepo")
		if org != "flagorg" || repo != "flagrepo" {
			t.Errorf("got %s/%s, want flagorg/flagrepo", org, repo)
		}
	})
	t.Run("env fallback", func(t *testing.T) {
		t.Setenv("GITHUB_REPOSITORY", "envorg/envrepo")
		org, repo := config.OrgRepoFromGitHubRepository("", "")
		if org != "envorg" || repo != "envrepo" {
			t.Errorf("got %s/%s, want envorg/envrepo", org, repo)
		}
	})
	t.Run("empty env", func(t *testing.T) {
		t.Setenv("GITHUB_REPOSITORY", "")
		org, repo := config.OrgRepoFromGitHubRepository("", "")
		if org != "" || repo != "" {
			t.Errorf("got %s/%s, want empty", org, repo)
		}
	})
}
