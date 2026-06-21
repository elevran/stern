package config_test

import (
	"strings"
	"testing"

	"github.com/elevran/stern/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidate_Clean(t *testing.T) {
	opts := validOptions()
	assert.Empty(t, opts.Validate(), "expected no errors for valid config")
}

func TestValidate_UnknownPlugin(t *testing.T) {
	opts := validOptions()
	opts.Plugins = []string{"lgmt"} // typo
	errs := opts.Validate()
	require.NotEmpty(t, errs, "expected error for unknown plugin")
	msg := errs[0].Error()
	assert.Contains(t, msg, "ERROR")
	assert.Contains(t, msg, "unknown plugin")
}

func TestValidate_InvalidMergeMethod(t *testing.T) {
	opts := validOptions()
	opts.Merge.Method = "fast-forward"
	errs := opts.Validate()
	hasError := false
	for _, e := range errs {
		if strings.Contains(e.Error(), "merge.method") && strings.Contains(e.Error(), "ERROR") {
			hasError = true
		}
	}
	assert.True(t, hasError, "expected ERROR for invalid merge.method, got: %v", errs)
}

func TestValidate_InvalidMergeStrategy(t *testing.T) {
	opts := validOptions()
	opts.Merge.Strategy = "automatic"
	errs := opts.Validate()
	hasError := false
	for _, e := range errs {
		if strings.Contains(e.Error(), "merge.strategy") && strings.Contains(e.Error(), "ERROR") {
			hasError = true
		}
	}
	assert.True(t, hasError, "expected ERROR for invalid merge.strategy, got: %v", errs)
}

func TestValidate_CherryPickEnabledWithoutPattern(t *testing.T) {
	opts := validOptions()
	opts.Plugins = []string{"cherry-pick"}
	opts.CherryPick.AllowedBranchPattern = ""
	errs := opts.Validate()
	hasError := false
	for _, e := range errs {
		if strings.Contains(e.Error(), "allowed_branch_pattern") && strings.Contains(e.Error(), "ERROR") {
			hasError = true
		}
	}
	assert.True(t, hasError, "expected ERROR for cherry-pick without branch pattern, got: %v", errs)
}

func TestValidate_InvalidBranchPatternRegex(t *testing.T) {
	opts := validOptions()
	opts.CherryPick.AllowedBranchPattern = "[invalid"
	errs := opts.Validate()
	hasError := false
	for _, e := range errs {
		if strings.Contains(e.Error(), "allowed_branch_pattern") && strings.Contains(e.Error(), "ERROR") {
			hasError = true
		}
	}
	assert.True(t, hasError, "expected ERROR for invalid regex, got: %v", errs)
}

func TestValidate_CherryPickCommand(t *testing.T) {
	tests := []struct {
		name      string
		command   string
		wantError bool
	}{
		{"empty (uses default)", "", false},
		{"cherry-pick (hyphenated)", "cherry-pick", false},
		{"cherrypick (no hyphen)", "cherrypick", false},
		{"cp (abbreviation)", "cp", false},
		{"cherry_pick (underscore rejected)", "cherry_pick", true},
		{"arbitrary string", "rebase", true},
		{"uppercase rejected", "CHERRY-PICK", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := validOptions()
			opts.CherryPick.Command = tt.command
			errs := opts.Validate()
			hasErr := false
			for _, e := range errs {
				if e.Level == "ERROR" && strings.Contains(e.Field, "cherry_pick.command") {
					hasErr = true
				}
			}
			if tt.wantError {
				assert.True(t, hasErr, "expected ERROR for command=%q, got: %v", tt.command, errs)
			} else {
				assert.False(t, hasErr, "expected no ERROR for command=%q, got: %v", tt.command, errs)
			}
		})
	}
}

func TestValidate_NativeStrategyNoBlockingLabels(t *testing.T) {
	opts := validOptions()
	opts.Merge.Strategy = "native"
	opts.Merge.BlockingLabels = nil
	errs := opts.Validate()
	hasWarn := false
	for _, e := range errs {
		if strings.Contains(e.Error(), "blocking_labels") && strings.Contains(e.Error(), "WARN") {
			hasWarn = true
		}
	}
	assert.True(t, hasWarn, "expected WARN for native strategy with no blocking labels, got: %v", errs)
}

func TestValidate_AllIssuesCollected(t *testing.T) {
	opts := validOptions()
	opts.Plugins = []string{"lgmt", "approv"} // two typos
	opts.Merge.Method = "fast-forward"
	errs := opts.Validate()
	// Should have: 2 unknown plugin errors + 1 merge.method error = 3+ errors
	assert.GreaterOrEqual(t, len(errs), 3, "expected at least 3 errors (all collected), got %d: %v", len(errs), errs)
}

func TestValidate_LoadBalancing(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{"empty (unset)", "", false},
		{"round-robin", "round-robin", false},
		{"least-busy", "least-busy", false},
		{"typo with underscore", "round_robin", true},
		{"unknown value", "random", true},
		{"uppercase", "Round-Robin", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := validOptions()
			opts.ReviewAssignment.LoadBalancing = tt.value
			errs := opts.Validate()
			hasErr := false
			for _, e := range errs {
				if strings.Contains(e.Error(), "review_assignment.load_balancing") {
					hasErr = true
				}
			}
			if tt.wantErr {
				assert.True(t, hasErr, "expected ERROR for load_balancing=%q, got: %v", tt.value, errs)
			} else {
				assert.False(t, hasErr, "expected no ERROR for load_balancing=%q, got: %v", tt.value, errs)
			}
		})
	}
}

func TestValidate_WarnOnlyExits0(t *testing.T) {
	opts := validOptions()
	opts.Merge.Strategy = "native"
	opts.Merge.BlockingLabels = nil
	errs := opts.Validate()

	hasError := false
	hasWarn := false
	for _, e := range errs {
		if strings.Contains(e.Error(), "ERROR") {
			hasError = true
		}
		if strings.Contains(e.Error(), "WARN") {
			hasWarn = true
		}
	}
	assert.False(t, hasError, "expected no ERROR for warn-only config")
	assert.True(t, hasWarn, "expected at least one WARN")
}

// validOptions returns an Options with all required fields set to valid values.
func validOptions() *config.Options {
	return &config.Options{
		Org:  "elevran",
		Repo: "stern",
		Merge: config.MergeOptions{
			Strategy:       "native",
			Method:         "squash",
			BlockingLabels: []string{"do-not-merge/hold"},
		},
		Lifecycle: config.LifecycleOptions{
			StaleDays:  90,
			RottenDays: 30,
			CloseAfter: 30,
		},
	}
}

func TestValidate_LifecycleDays(t *testing.T) {
	tests := []struct {
		name       string
		staleDays  int
		rottenDays int
		closeAfter int
		wantErr    bool
	}{
		{"defaults applied (all positive)", 90, 30, 30, false},
		{"explicit positive", 60, 14, 7, false},
		{"stale_days zero (rejected per #17/#74)", 0, 30, 30, true},
		{"rotten_days zero (rejected per #17/#74)", 90, 0, 30, true},
		{"close_after zero (rejected per #17/#74)", 90, 30, 0, true},
		{"all zero", 0, 0, 0, true},
		{"stale_days negative", -1, 30, 30, true},
		{"rotten_days negative", 90, -1, 30, true},
		{"close_after negative", 90, 30, -1, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := validOptions()
			opts.Lifecycle = config.LifecycleOptions{
				StaleDays:  tt.staleDays,
				RottenDays: tt.rottenDays,
				CloseAfter: tt.closeAfter,
			}
			issues := opts.Validate()
			hasErr := false
			for _, e := range issues {
				if strings.Contains(e.Error(), "lifecycle.") {
					hasErr = true
				}
			}
			if tt.wantErr {
				assert.True(t, hasErr, "expected ERROR for stale=%d rotten=%d close_after=%d",
					tt.staleDays, tt.rottenDays, tt.closeAfter)
			} else {
				assert.False(t, hasErr, "expected no ERROR for stale=%d rotten=%d close_after=%d",
					tt.staleDays, tt.rottenDays, tt.closeAfter)
			}
		})
	}
}

func TestValidate_LifecycleEnabled(t *testing.T) {
	opts := validOptions()
	opts.Lifecycle.Enabled = true
	issues := opts.Validate()
	for _, e := range issues {
		if strings.Contains(e.Error(), "lifecycle.") {
			t.Errorf("expected no lifecycle errors with Enabled=true and valid timing, got: %v", issues)
		}
	}
}

func TestValidate_LifecyclePerTypeOverrides(t *testing.T) {
	tests := []struct {
		name     string
		override config.LifecycleItemOptions
		field    string // substring expected in the failing issue.Field; empty means no error
	}{
		{"zero override (inherit)", config.LifecycleItemOptions{}, ""},
		{"positive override", config.LifecycleItemOptions{StaleDays: 30, RottenDays: 14, CloseAfter: 7}, ""},
		{"negative stale_days", config.LifecycleItemOptions{StaleDays: -1}, "lifecycle.issues.stale_days"},
		{"negative rotten_days", config.LifecycleItemOptions{RottenDays: -1}, "lifecycle.issues.rotten_days"},
		{"negative close_after", config.LifecycleItemOptions{CloseAfter: -1}, "lifecycle.issues.close_after"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := validOptions()
			opts.Lifecycle.Issues = tt.override
			issues := opts.Validate()
			if tt.field == "" {
				for _, e := range issues {
					if strings.HasPrefix(e.Field, "lifecycle.issues") {
						t.Errorf("expected no lifecycle.issues error, got: %v", issues)
					}
				}
				return
			}
			found := false
			for _, e := range issues {
				if e.Level == "ERROR" && strings.Contains(e.Field, tt.field) {
					found = true
				}
			}
			assert.True(t, found, "expected ERROR for %s, got: %v", tt.field, issues)
		})
	}

	t.Run("pull_requests negative is reported under pull_requests prefix", func(t *testing.T) {
		opts := validOptions()
		opts.Lifecycle.PullRequests = config.LifecycleItemOptions{StaleDays: -1}
		issues := opts.Validate()
		found := false
		for _, e := range issues {
			if e.Level == "ERROR" && strings.Contains(e.Field, "lifecycle.pull_requests.stale_days") {
				found = true
			}
		}
		assert.True(t, found, "expected ERROR for lifecycle.pull_requests.stale_days, got: %v", issues)
	})
}

func TestLifecycle_ForIssuesForPRs(t *testing.T) {
	t.Run("zero overrides inherit global defaults", func(t *testing.T) {
		o := config.LifecycleOptions{
			StaleDays:     90,
			RottenDays:    30,
			CloseAfter:    14,
			StaleComment:  "g-stale",
			RottenComment: "g-rotten",
			CloseComment:  "g-close",
		}
		got := o.ForIssues()
		assert.Equal(t, 90, got.StaleDays)
		assert.Equal(t, 30, got.RottenDays)
		assert.Equal(t, 14, got.CloseAfter)
		assert.Equal(t, "g-stale", got.StaleComment)
		assert.Equal(t, "g-rotten", got.RottenComment)
		assert.Equal(t, "g-close", got.CloseComment)
	})

	t.Run("non-zero overrides win", func(t *testing.T) {
		o := config.LifecycleOptions{
			StaleDays:     90,
			RottenDays:    30,
			CloseAfter:    14,
			StaleComment:  "g-stale",
			RottenComment: "g-rotten",
			CloseComment:  "g-close",
			Issues: config.LifecycleItemOptions{
				StaleDays:    60,
				StaleComment: "i-stale",
			},
		}
		got := o.ForIssues()
		// Override fields take precedence.
		assert.Equal(t, 60, got.StaleDays)
		assert.Equal(t, "i-stale", got.StaleComment)
		// Unset override fields fall back to global defaults.
		assert.Equal(t, 30, got.RottenDays)
		assert.Equal(t, 14, got.CloseAfter)
		assert.Equal(t, "g-rotten", got.RottenComment)
		assert.Equal(t, "g-close", got.CloseComment)
	})

	t.Run("ForPRs uses pull_requests overrides", func(t *testing.T) {
		o := config.LifecycleOptions{
			StaleDays:  90,
			RottenDays: 30,
			CloseAfter: 14,
			PullRequests: config.LifecycleItemOptions{
				StaleDays:  30,
				RottenDays: 14,
			},
		}
		gotPR := o.ForPRs()
		assert.Equal(t, 30, gotPR.StaleDays)
		assert.Equal(t, 14, gotPR.RottenDays)
		assert.Equal(t, 14, gotPR.CloseAfter, "CloseAfter should inherit global default")

		// ForIssues unaffected by PR overrides.
		gotIssue := o.ForIssues()
		assert.Equal(t, 90, gotIssue.StaleDays)
		assert.Equal(t, 30, gotIssue.RottenDays)
	})
}

func TestValidate_LabelDefinitions(t *testing.T) {
	tests := []struct {
		name      string
		labels    []config.LabelDefinition
		wantFails []string // substrings expected in issue.Field
	}{
		{
			name: "valid entry",
			labels: []config.LabelDefinition{
				{Name: "lgtm", Color: "0e8a16", Description: "Looks good to me"},
			},
			wantFails: nil,
		},
		{
			name: "empty name",
			labels: []config.LabelDefinition{
				{Name: "", Color: "0e8a16"},
			},
			wantFails: []string{"label_definitions[0].name"},
		},
		{
			name: "color with hash",
			labels: []config.LabelDefinition{
				{Name: "lgtm", Color: "#0e8a16"},
			},
			wantFails: []string{"label_definitions[0].color"},
		},
		{
			name: "color too short",
			labels: []config.LabelDefinition{
				{Name: "lgtm", Color: "fff"},
			},
			wantFails: []string{"label_definitions[0].color"},
		},
		{
			name: "color non-hex",
			labels: []config.LabelDefinition{
				{Name: "lgtm", Color: "zzzzzz"},
			},
			wantFails: []string{"label_definitions[0].color"},
		},
		{
			name: "description too long",
			labels: []config.LabelDefinition{
				{Name: "lgtm", Color: "0e8a16", Description: strings.Repeat("a", 101)},
			},
			wantFails: []string{"label_definitions[0].description"},
		},
		{
			name: "uppercase hex color accepted",
			labels: []config.LabelDefinition{
				{Name: "lgtm", Color: "0E8A16"},
			},
			wantFails: nil,
		},
		{
			name: "second entry wrong index in path",
			labels: []config.LabelDefinition{
				{Name: "lgtm", Color: "0e8a16"},
				{Name: "approved", Color: "0e8a16"},
				{Name: "", Color: "0e8a16"},
			},
			wantFails: []string{"label_definitions[2].name"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := validOptions()
			opts.LabelDefinitions = tt.labels
			issues := opts.Validate()
			for _, want := range tt.wantFails {
				found := false
				for _, e := range issues {
					if e.Level == "ERROR" && strings.Contains(e.Field, want) {
						found = true
						break
					}
				}
				assert.True(t, found, "expected ERROR matching %q, got issues: %v", want, issues)
			}
			if tt.wantFails == nil {
				for _, e := range issues {
					if e.Level == "ERROR" && strings.HasPrefix(e.Field, "label_definitions") {
						t.Errorf("expected no label_definitions errors, got: %v", issues)
					}
				}
			}
		})
	}
}

func TestValidate_LabelCrossReferences(t *testing.T) {
	t.Run("blocking label defined", func(t *testing.T) {
		opts := validOptions()
		opts.LabelDefinitions = []config.LabelDefinition{
			{Name: "do-not-merge/hold", Color: "b60205"},
		}
		opts.Merge.BlockingLabels = []string{"do-not-merge/hold"}
		issues := opts.Validate()
		for _, e := range issues {
			if strings.Contains(e.Field, "merge.blocking_labels") {
				t.Errorf("expected no blocking_labels cross-ref error, got: %v", issues)
			}
		}
	})
	t.Run("blocking label undefined", func(t *testing.T) {
		opts := validOptions()
		opts.LabelDefinitions = []config.LabelDefinition{
			{Name: "lgtm", Color: "0e8a16"},
		}
		opts.Merge.BlockingLabels = []string{"do-not-merge/hold"}
		issues := opts.Validate()
		found := false
		for _, e := range issues {
			if e.Level == "ERROR" && strings.Contains(e.Field, "merge.blocking_labels") &&
				strings.Contains(e.Message, "do-not-merge/hold") {
				found = true
			}
		}
		assert.True(t, found, "expected blocking_labels cross-ref ERROR, got: %v", issues)
	})
	t.Run("empty label_definitions skips check", func(t *testing.T) {
		opts := validOptions()
		// no label_definitions, blocking_labels set to an unknown value
		issues := opts.Validate()
		for _, e := range issues {
			if strings.Contains(e.Field, "merge.blocking_labels") &&
				strings.Contains(e.Message, "not found in label_definitions") {
				t.Errorf("expected no cross-ref error when no label_definitions, got: %v", issues)
			}
		}
	})
}
