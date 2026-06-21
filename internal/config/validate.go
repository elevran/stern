package config

import (
	"fmt"
	"regexp"
	"slices"
)

// known plugin names for validation and "did you mean?" suggestions.
var knownPlugins = []string{
	"lgtm", "approve", "hold", "wip", "close", "reopen", "milestone",
	"retest",
	"cherry-pick", "review_assignment", "size", "lifecycle",
	"kind", "area", "priority",
}

// ValidationIssue is a single validation finding.
type ValidationIssue struct {
	Level   string // "ERROR" or "WARN"
	Field   string
	Message string
}

func (v ValidationIssue) Error() string {
	return fmt.Sprintf("%s  %s: %s", v.Level, v.Field, v.Message)
}

// Validate checks all options for errors and warnings.
// Returns all issues found; does not stop at the first error.
func (o *Options) Validate() []ValidationIssue {
	var issues []ValidationIssue

	// Unknown plugin names.
	for i, p := range o.Plugins {
		if !isKnownPlugin(p) {
			issues = append(issues, ValidationIssue{
				Level:   "ERROR",
				Field:   fmt.Sprintf("plugins[%d]", i),
				Message: fmt.Sprintf("unknown plugin %q", p),
			})
		}
	}

	issues = append(issues, o.Merge.validate()...)
	issues = append(issues, o.CherryPick.validate(o.HasPlugin("cherry-pick"))...)
	issues = append(issues, o.Lifecycle.validate()...)
	issues = append(issues, o.ReviewAssignment.validate()...)
	issues = append(issues, o.Kind.validate(o.HasPlugin("kind"))...)
	issues = append(issues, o.Area.validate(o.HasPlugin("area"))...)
	issues = append(issues, o.Priority.validate(o.HasPlugin("priority"))...)
	issues = append(issues, o.Size.validate(o.HasPlugin("size"))...)

	// Per-entry label_definitions validation (#67).
	for i := range o.LabelDefinitions {
		issues = append(issues, o.LabelDefinitions[i].validate(i)...)
	}

	// Cross-reference of label names used by plugins to entries in
	// label_definitions (#47). Only `merge.blocking_labels` exists as a
	// reference field today; `approve.merge_label` and `cherry_pick.label`
	// are not yet implemented in the Options struct (see deviations in PR Q2).
	issues = append(issues, o.validateLabelReferences()...)

	return issues
}

func isKnownPlugin(name string) bool {
	return slices.Contains(knownPlugins, name)
}

// cherry_pick validation lives here as it needs cross-plugin awareness.
func (o *CherryPickOptions) validate(pluginEnabled bool) []ValidationIssue {
	var issues []ValidationIssue
	if pluginEnabled && o.AllowedBranchPattern == "" {
		issues = append(issues, ValidationIssue{
			Level:   "ERROR",
			Field:   "cherry_pick.allowed_branch_pattern",
			Message: "cherry-pick plugin is enabled but allowed_branch_pattern is empty",
		})
	}
	if o.AllowedBranchPattern != "" {
		if _, err := regexp.Compile(o.AllowedBranchPattern); err != nil {
			issues = append(issues, ValidationIssue{
				Level:   "ERROR",
				Field:   "cherry_pick.allowed_branch_pattern",
				Message: fmt.Sprintf("invalid regex: %v", err),
			})
		}
	}
	switch o.Command {
	case "", CherryPickCommandCherryPick, CherryPickCommandCherrypick, CherryPickCommandCP:
		// valid (empty means the default will be applied)
	default:
		issues = append(issues, ValidationIssue{
			Level:   "ERROR",
			Field:   "cherry_pick.command",
			Message: fmt.Sprintf("invalid value %q (must be cherry-pick, cherrypick, or cp)", o.Command),
		})
	}
	return issues
}

// validateLabelReferences checks that any label name referenced by a plugin
// option is defined in label_definitions (#47).
func (o *Options) validateLabelReferences() []ValidationIssue {
	if len(o.LabelDefinitions) == 0 {
		return nil // Nothing to cross-reference against.
	}
	defined := make(map[string]bool, len(o.LabelDefinitions))
	for _, l := range o.LabelDefinitions {
		if l.Name != "" {
			defined[l.Name] = true
		}
	}
	var issues []ValidationIssue
	for _, bl := range o.Merge.BlockingLabels {
		if bl == "" || defined[bl] {
			continue
		}
		issues = append(issues, ValidationIssue{
			Level:   "ERROR",
			Field:   "merge.blocking_labels",
			Message: fmt.Sprintf("label %q not found in label_definitions", bl),
		})
	}
	return issues
}
