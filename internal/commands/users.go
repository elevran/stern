package commands

import (
	"fmt"
	"strings"
)

// parseUsers strips "@" prefixes from each arg, lowercases, and deduplicates
// (preserving first-occurrence order). If args is empty and selfFallback is
// non-empty, returns []string{selfFallback}. Returns an error if the result
// would be empty.
func parseUsers(args []string, selfFallback string) ([]string, error) {
	seen := make(map[string]bool)
	var out []string
	for _, a := range args {
		u := strings.TrimPrefix(a, "@")
		u = strings.ToLower(u)
		if u == "" || seen[u] {
			continue
		}
		seen[u] = true
		out = append(out, u)
	}
	if len(out) == 0 {
		if selfFallback != "" {
			return []string{strings.ToLower(selfFallback)}, nil
		}
		return nil, fmt.Errorf("at least one user is required")
	}
	return out, nil
}
