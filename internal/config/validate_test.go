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
	if !strings.Contains(msg, "did you mean") {
		t.Errorf("expected 'did you mean' suggestion, got: %s", msg)
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
			Strategy:      "native",
			Method:        "squash",
			BlockingLabels: []string{"do-not-merge/hold"},
		},
	}
}
