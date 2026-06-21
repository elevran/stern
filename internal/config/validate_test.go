package config_test

import (
	"strings"
	"testing"

	"github.com/elevran/stern/internal/config"
)

func TestValidate_Clean(t *testing.T) {
	opts := validOptions()
	if errs := opts.Validate(); len(errs) != 0 {
		t.Errorf("expected no errors for valid config, got: %v", errs)
	}
}

func TestValidate_UnknownPlugin(t *testing.T) {
	opts := validOptions()
	opts.Plugins = []string{"lgmt"} // typo
	errs := opts.Validate()
	if len(errs) == 0 {
		t.Fatal("expected error for unknown plugin")
	}
	msg := errs[0].Error()
	if !strings.Contains(msg, "ERROR") {
		t.Errorf("expected ERROR level, got: %s", msg)
	}
	if !strings.Contains(msg, "unknown plugin") {
		t.Errorf("expected 'unknown plugin' in message, got: %s", msg)
	}
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
	if !hasError {
		t.Errorf("expected ERROR for invalid merge.method, got: %v", errs)
	}
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
	if !hasError {
		t.Errorf("expected ERROR for invalid merge.strategy, got: %v", errs)
	}
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
	if !hasError {
		t.Errorf("expected ERROR for cherry-pick without branch pattern, got: %v", errs)
	}
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
	if !hasError {
		t.Errorf("expected ERROR for invalid regex, got: %v", errs)
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
	if !hasWarn {
		t.Errorf("expected WARN for native strategy with no blocking labels, got: %v", errs)
	}
}

func TestValidate_AllIssuesCollected(t *testing.T) {
	opts := validOptions()
	opts.Plugins = []string{"lgmt", "approv"} // two typos
	opts.Merge.Method = "fast-forward"
	errs := opts.Validate()
	// Should have: 2 unknown plugin errors + 1 merge.method error = 3+ errors
	if len(errs) < 3 {
		t.Errorf("expected at least 3 errors (all collected), got %d: %v", len(errs), errs)
	}
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
			if tt.wantErr && !hasErr {
				t.Errorf("expected ERROR for load_balancing=%q, got: %v", tt.value, errs)
			}
			if !tt.wantErr && hasErr {
				t.Errorf("expected no ERROR for load_balancing=%q, got: %v", tt.value, errs)
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
	if hasError {
		t.Error("expected no ERROR for warn-only config")
	}
	if !hasWarn {
		t.Error("expected at least one WARN")
	}
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
		},
	}
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
				if !found {
					t.Errorf("expected ERROR matching %q, got issues: %v", want, issues)
				}
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
		if !found {
			t.Errorf("expected blocking_labels cross-ref ERROR, got: %v", issues)
		}
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
