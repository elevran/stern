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
	return issues
}
