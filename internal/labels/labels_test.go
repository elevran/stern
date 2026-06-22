package labels

import "testing"

// TestConstants pins the string values of the label constants. If any of
// these change, slash commands, OWNERS auto-merge, and the lifecycle sweep
// would break in subtle ways — a constant here catches accidental edits.
func TestConstants(t *testing.T) {
	tests := []struct {
		name string
		got  string
		want string
	}{
		{"LGTM", LGTM, "lgtm"},
		{"Approved", Approved, "approved"},
		{"Hold", Hold, "do-not-merge/hold"},
		{"WIP", WIP, "do-not-merge/wip"},
		{"NeedsRebase", NeedsRebase, "needs-rebase"},
		{"NeedsTriage", NeedsTriage, "needs-triage"},
		{"LifecycleStale", LifecycleStale, "lifecycle/stale"},
		{"LifecycleRotten", LifecycleRotten, "lifecycle/rotten"},
		{"LifecycleFrozen", LifecycleFrozen, "lifecycle/frozen"},
		{"SizePrefix", SizePrefix, "size/"},
		{"SizeXS", SizeXS, "size/XS"},
		{"SizeS", SizeS, "size/S"},
		{"SizeM", SizeM, "size/M"},
		{"SizeL", SizeL, "size/L"},
		{"SizeXL", SizeXL, "size/XL"},
		{"SizeXXL", SizeXXL, "size/XXL"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Errorf("%s = %q, want %q", tt.name, tt.got, tt.want)
			}
		})
	}
}

// TestSizeBucketNames verifies each named bucket matches the documented
// prefix + canonical casing.
func TestSizeBucketNames(t *testing.T) {
	for _, b := range []struct {
		name string
		val  string
	}{
		{"SizeXS", SizeXS},
		{"SizeS", SizeS},
		{"SizeM", SizeM},
		{"SizeL", SizeL},
		{"SizeXL", SizeXL},
		{"SizeXXL", SizeXXL},
	} {
		if got, want := b.val, SizePrefix+b.name[4:]; got != want {
			t.Errorf("%s = %q, want %q (SizePrefix+%q)", b.name, got, want, b.name[4:])
		}
	}
}
