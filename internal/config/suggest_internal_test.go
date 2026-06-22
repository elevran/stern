package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSuggestPlugin(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		// Exact matches return themselves — they shouldn't reach suggestPlugin
		// in practice (Validate checks isKnownPlugin first), but the function
		// still returns the closest match, which is the exact match.
		{"exact", "lgtm", "lgtm"},
		// Distance 1: classic typos.
		{"single transposition", "lgmt", "lgtm"},
		{"single substitution", "lgt", "lgtm"},
		{"missing char", "aprove", "approve"},
		// Distance 2.
		{"two substitutions", "approv", "approve"},
		{"transposed cherry-pick", "cherrypick", "cherry-pick"},
		// Distance > 2: no suggestion.
		{"totally different", "xyzzy", ""},
		{"empty", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, suggestPlugin(tt.in))
		})
	}
}

func TestEditDistance(t *testing.T) {
	tests := []struct {
		a, b string
		want int
	}{
		{"", "", 0},
		{"abc", "abc", 0},
		{"", "abc", 3},
		{"abc", "", 3},
		{"kitten", "sitting", 3},
		{"flaw", "lawn", 2},
		{"a", "b", 1},
		// Transposition costs 2 in Levenshtein (one delete + one insert).
		{"lgmt", "lgtm", 2},
		{"cherrypick", "cherry-pick", 1},
	}
	for _, tt := range tests {
		t.Run(tt.a+"_"+tt.b, func(t *testing.T) {
			assert.Equal(t, tt.want, editDistance(tt.a, tt.b))
		})
	}
}
