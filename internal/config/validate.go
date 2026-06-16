package config

import (
	"fmt"
	"regexp"
	"strings"
)

// known plugin names for validation and "did you mean?" suggestions.
var knownPlugins = []string{
	"lgtm", "approve", "hold", "wip",
	"cherry-pick", "review_assignment", "size", "lifecycle",
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

// Validate checks the options for errors and warnings. Returns all issues;
// does not stop at the first error.
func (o *Options) Validate() []error {
	var issues []ValidationIssue

	// Unknown plugin names.
	for i, p := range o.Plugins {
		if !isKnownPlugin(p) {
			msg := fmt.Sprintf("unknown plugin %q", p)
			if suggestion := closestPlugin(p); suggestion != "" {
				msg += fmt.Sprintf(" (did you mean %q?)", suggestion)
			}
			issues = append(issues, ValidationIssue{
				Level:   "ERROR",
				Field:   fmt.Sprintf("plugins[%d]", i),
				Message: msg,
			})
		}
	}

	// merge.method
	switch o.Merge.Method {
	case "squash", "merge", "rebase", "":
	default:
		issues = append(issues, ValidationIssue{
			Level:   "ERROR",
			Field:   "merge.method",
			Message: fmt.Sprintf("invalid value %q (must be squash, merge, or rebase)", o.Merge.Method),
		})
	}

	// merge.strategy
	switch o.Merge.Strategy {
	case "native", "bot", "":
	default:
		issues = append(issues, ValidationIssue{
			Level:   "ERROR",
			Field:   "merge.strategy",
			Message: fmt.Sprintf("invalid value %q (must be native or bot)", o.Merge.Strategy),
		})
	}

	// cherry_pick.allowed_branch_pattern must compile if cherry-pick is enabled.
	if o.HasPlugin("cherry-pick") && o.CherryPick.AllowedBranchPattern == "" {
		issues = append(issues, ValidationIssue{
			Level:   "ERROR",
			Field:   "cherry_pick.allowed_branch_pattern",
			Message: "cherry-pick plugin is enabled but allowed_branch_pattern is empty",
		})
	}
	if o.CherryPick.AllowedBranchPattern != "" {
		if _, err := regexp.Compile(o.CherryPick.AllowedBranchPattern); err != nil {
			issues = append(issues, ValidationIssue{
				Level:   "ERROR",
				Field:   "cherry_pick.allowed_branch_pattern",
				Message: fmt.Sprintf("invalid regex: %v", err),
			})
		}
	}

	// lifecycle thresholds must be positive.
	if o.Lifecycle.StaleDays < 0 {
		issues = append(issues, ValidationIssue{
			Level:   "ERROR",
			Field:   "lifecycle.stale_days",
			Message: "must be a positive integer",
		})
	}
	if o.Lifecycle.RottenDays < 0 {
		issues = append(issues, ValidationIssue{
			Level:   "ERROR",
			Field:   "lifecycle.rotten_days",
			Message: "must be a positive integer",
		})
	}

	// Warn when native strategy has no blocking labels.
	if o.Merge.Strategy == "native" && len(o.Merge.BlockingLabels) == 0 {
		issues = append(issues, ValidationIssue{
			Level:   "WARN",
			Field:   "merge.blocking_labels",
			Message: "empty — hold labels will not block auto-merge",
		})
	}

	errs := make([]error, len(issues))
	for i, issue := range issues {
		errs[i] = issue
	}
	return errs
}

func isKnownPlugin(name string) bool {
	for _, k := range knownPlugins {
		if k == name {
			return true
		}
	}
	return false
}

// closestPlugin returns the known plugin name within edit distance 2 of name,
// or "" if none is close enough.
func closestPlugin(name string) string {
	best := ""
	bestDist := 3 // threshold
	for _, k := range knownPlugins {
		if d := editDistance(name, k); d < bestDist {
			bestDist = d
			best = k
		}
	}
	return best
}

// editDistance computes the Levenshtein distance between a and b.
func editDistance(a, b string) int {
	a = strings.ToLower(a)
	b = strings.ToLower(b)
	m, n := len(a), len(b)
	dp := make([][]int, m+1)
	for i := range dp {
		dp[i] = make([]int, n+1)
		dp[i][0] = i
	}
	for j := 0; j <= n; j++ {
		dp[0][j] = j
	}
	for i := 1; i <= m; i++ {
		for j := 1; j <= n; j++ {
			if a[i-1] == b[j-1] {
				dp[i][j] = dp[i-1][j-1]
			} else {
				dp[i][j] = 1 + min3(dp[i-1][j], dp[i][j-1], dp[i-1][j-1])
			}
		}
	}
	return dp[m][n]
}

func min3(a, b, c int) int {
	if a < b {
		if a < c {
			return a
		}
		return c
	}
	if b < c {
		return b
	}
	return c
}
