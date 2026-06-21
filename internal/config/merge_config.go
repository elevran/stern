package config

import "fmt"

// Merge method constants.
const (
	MergeMethodSquash = "squash"
	MergeMethodMerge  = "merge"
	MergeMethodRebase = "rebase"
)

// Merge strategy constants.
const (
	MergeStrategyNative = "native"
	MergeStrategyBot    = "bot"
)

// MergeOptions configures merge eligibility and auto-merge behavior.
type MergeOptions struct {
	Strategy       string   `yaml:"strategy"` // native | bot
	Method         string   `yaml:"method"`   // squash | merge | rebase
	BlockingLabels []string `yaml:"blocking_labels"`
}

func (o *MergeOptions) applyDefaults() {
	if o.Strategy == "" {
		o.Strategy = MergeStrategyNative
	}
	if o.Method == "" {
		o.Method = MergeMethodSquash
	}
}

func (o *MergeOptions) validate() []ValidationIssue {
	var issues []ValidationIssue
	switch o.Method {
	case MergeMethodSquash, MergeMethodMerge, MergeMethodRebase, "":
	default:
		issues = append(issues, ValidationIssue{
			Level:   "ERROR",
			Field:   "merge.method",
			Message: fmt.Sprintf("invalid value %q (must be squash, merge, or rebase)", o.Method),
		})
	}
	switch o.Strategy {
	case MergeStrategyNative, MergeStrategyBot, "":
	default:
		issues = append(issues, ValidationIssue{
			Level:   "ERROR",
			Field:   "merge.strategy",
			Message: fmt.Sprintf("invalid value %q (must be native or bot)", o.Strategy),
		})
	}
	if o.Strategy == MergeStrategyNative && len(o.BlockingLabels) == 0 {
		issues = append(issues, ValidationIssue{
			Level:   "WARN",
			Field:   "merge.blocking_labels",
			Message: "empty — hold labels will not block auto-merge",
		})
	}
	return issues
}
