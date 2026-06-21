package owners

import (
	"encoding/json"
	"fmt"
	"os"
	"path"
	"sync"
)

// FileCache is a path→content read-through cache for OWNERS file content.
// A nil FileCache is a no-op (live API only).
//
// Keys are formatted as "owner/repo/ref/path" — the same tuple that
// identifies a unique GetFileContent call — so different refs (PR head
// SHAs) get their own entries and never collide.
type FileCache struct {
	mu    sync.Mutex
	store map[string][]byte
	path  string // backing file path; "" means in-memory only
}

// LoadCacheFile reads a cache file from disk and returns the populated
// cache. A missing file is not an error — the caller gets an empty
// cache ready for writes.
//
// The path is operator-controlled (it comes from the OWNERS_CACHE_FILE
// env var configured in the workflow file, not from user input).
// gosec G304 is therefore a false positive.
func LoadCacheFile(p string) (*FileCache, error) {
	c := &FileCache{
		store: make(map[string][]byte),
		path:  p,
	}
	if p == "" {
		return c, nil
	}
	// #nosec G304 -- operator-controlled cache path from OWNERS_CACHE_FILE.
	data, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return c, nil
		}
		return nil, fmt.Errorf("reading owners cache %q: %w", p, err)
	}
	if len(data) == 0 {
		return c, nil
	}
	if err := json.Unmarshal(data, &c.store); err != nil {
		return nil, fmt.Errorf("parsing owners cache %q: %w", p, err)
	}
	return c, nil
}

// Save flushes the cache to its backing file. No-op when no path was set
// at construction time.
func (c *FileCache) Save() error {
	if c == nil || c.path == "" {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	data, err := json.Marshal(c.store)
	if err != nil {
		return err
	}
	// 0600 — the cache contains fetched file content from a private repo,
	// readable only by the runner user. gosec G306 prefers 0600 over 0644.
	return os.WriteFile(c.path, data, 0o600)
}

// get returns the cached content for a key. The bool reports whether the
// key was present.
func (c *FileCache) get(key string) ([]byte, bool) {
	if c == nil {
		return nil, false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	data, ok := c.store[key]
	return data, ok
}

// set stores content under a key, overwriting any prior value.
func (c *FileCache) set(key string, data []byte) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.store[key] = data
}

// ambientCache is the read-through cache consulted by LoadForPaths /
// loadAliases when their explicit cache parameter is nil. Production code
// in cmd/stern populates it at startup; tests leave it nil.
var ambientCache *FileCache

// SetAmbientCache installs a process-wide cache for OWNERS file reads.
// Subsequent LoadForPaths calls with a nil cache argument will consult it.
// Mostly useful from cmd/stern where the cache file path comes from an
// environment variable and propagating it through every call site would
// be invasive.
func SetAmbientCache(c *FileCache) { ambientCache = c }

// AmbientCache returns the currently-installed ambient cache, or nil when
// no caller has set one.
func AmbientCache() *FileCache { return ambientCache }

// effectiveCache returns the explicit cache when non-nil, otherwise the
// ambient one. Centralised so the various helpers stay consistent.
func effectiveCache(c *FileCache) *FileCache {
	if c != nil {
		return c
	}
	return ambientCache
}

// cacheKey is the cache lookup key. Centralised so callers and tests
// agree on the format.
func cacheKey(owner, repo, ref, filePath string) string {
	return path.Join(owner, repo, ref, filePath)
}
