package owners_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/elevran/stern/internal/github"
	"github.com/elevran/stern/internal/owners"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// countingClient records how many times GetFileContent is invoked.
type countingClient struct {
	*github.MockClient
	calls int
}

func (c *countingClient) GetFileContent(ctx context.Context, owner, repo, path, ref string) ([]byte, error) {
	c.calls++
	return c.MockClient.GetFileContent(ctx, owner, repo, path, ref)
}

// warmCache populates the cache by running one LoadForPaths call through
// the public API. The cache is then warm for subsequent calls.
func warmCache(t *testing.T, cc *countingClient, cache *owners.FileCache, path string) {
	t.Helper()
	mc := cc.MockClient
	mc.FileContent[path] = []byte("approvers:\n  - alice\n")
	// Also seed an empty OWNERS_ALIASES so the second call's alias lookup
	// hits the cache (loadAliases caches its successful results, but
	// negative lookups are NOT cached and would re-hit the API).
	mc.FileContent["OWNERS_ALIASES@sha"] = []byte("aliases: {}\n")
	_, err := owners.LoadForPaths(context.Background(), cc, cache, "o", "r", "sha", []string{"main.go"})
	require.NoError(t, err)
}

// TestFileCache_HitAvoidsAPICall verifies that a cache hit short-circuits
// the underlying ContentClient.
func TestFileCache_HitAvoidsAPICall(t *testing.T) {
	mc := github.NewMockClient()
	cc := &countingClient{MockClient: mc}

	cache, err := owners.LoadCacheFile("")
	require.NoError(t, err)
	warmCache(t, cc, cache, "OWNERS@sha")
	// warmCache made cold calls for both OWNERS and OWNERS_ALIASES.
	assert.Equal(t, 2, cc.calls)

	// Second call must hit the cache for every file.
	_, err = owners.LoadForPaths(context.Background(), cc, cache, "o", "r", "sha", []string{"main.go"})
	require.NoError(t, err)
	assert.Equal(t, 2, cc.calls, "expected zero additional API calls when cache is warm")
}

// TestFileCache_MissFallsThroughToAPI verifies that a cache miss still calls
// the live API and stores the result for next time.
func TestFileCache_MissFallsThroughToAPI(t *testing.T) {
	mc := github.NewMockClient()
	mc.FileContent["OWNERS@sha"] = []byte("approvers:\n  - alice\n")
	mc.FileContent["OWNERS_ALIASES@sha"] = []byte("aliases: {}\n")

	cc := &countingClient{MockClient: mc}
	cache, err := owners.LoadCacheFile("")
	require.NoError(t, err)

	result, err := owners.LoadForPaths(context.Background(), cc, cache, "o", "r", "sha", []string{"main.go"})
	require.NoError(t, err)
	assert.True(t, result.IsApprover("alice"))
	// Cold cache: 1 OWNERS_ALIASES call + 1 OWNERS call = 2.
	assert.Equal(t, 2, cc.calls, "expected two API calls on cold cache")

	// Second call: both files now cached, zero additional calls.
	_, err = owners.LoadForPaths(context.Background(), cc, cache, "o", "r", "sha", []string{"main.go"})
	require.NoError(t, err)
	assert.Equal(t, 2, cc.calls, "expected zero additional calls when cache is warm")
}

// TestFileCache_NilIsNoop verifies that passing a nil cache leaves the
// existing live-only behaviour unchanged.
func TestFileCache_NilIsNoop(t *testing.T) {
	mc := github.NewMockClient()
	mc.FileContent["OWNERS@sha"] = []byte("approvers:\n  - alice\n")
	mc.FileContent["OWNERS_ALIASES@sha"] = []byte("aliases: {}\n")

	cc := &countingClient{MockClient: mc}
	result, err := owners.LoadForPaths(context.Background(), cc, nil, "o", "r", "sha", []string{"main.go"})
	require.NoError(t, err)
	assert.True(t, result.IsApprover("alice"))
	// Without cache, every call re-fetches both files.
	assert.Equal(t, 2, cc.calls, "expected two API calls with nil cache")

	// A second call also re-fetches both — no caching when cache is nil.
	_, err = owners.LoadForPaths(context.Background(), cc, nil, "o", "r", "sha", []string{"main.go"})
	require.NoError(t, err)
	assert.Equal(t, 4, cc.calls, "expected two more API calls (no caching with nil cache)")
}

// TestFileCache_SaveLoadRoundtrip verifies that Save + LoadCacheFile
// round-trips a populated cache.
func TestFileCache_SaveLoadRoundtrip(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "owners-cache.json")

	// Warm and save.
	{
		mc := github.NewMockClient()
		cc := &countingClient{MockClient: mc}
		cache, err := owners.LoadCacheFile(p)
		require.NoError(t, err)
		warmCache(t, cc, cache, "OWNERS@sha")
		require.NoError(t, cache.Save())
	}

	// File should now exist on disk.
	_, err := os.Stat(p)
	require.NoError(t, err)

	// Fresh load — second run should make zero API calls.
	mc := github.NewMockClient()
	cc := &countingClient{MockClient: mc}
	cache, err := owners.LoadCacheFile(p)
	require.NoError(t, err)
	_, err = owners.LoadForPaths(context.Background(), cc, cache, "o", "r", "sha", []string{"main.go"})
	require.NoError(t, err)
	assert.Equal(t, 0, cc.calls, "expected zero API calls after roundtripped cache")
}

// TestFileCache_LoadMissingFileIsEmpty verifies that LoadCacheFile on a
// non-existent path returns an empty cache (not an error).
func TestFileCache_LoadMissingFileIsEmpty(t *testing.T) {
	c, err := owners.LoadCacheFile(filepath.Join(t.TempDir(), "nonexistent.json"))
	require.NoError(t, err)
	require.NotNil(t, c)
	// Save on a cache with a writable path should create the file.
	require.NoError(t, c.Save())
}
