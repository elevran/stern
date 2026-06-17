package config

import "fmt"

// MergeOptions configures merge eligibility and auto-merge behavior.
type MergeOptions struct {
	Strategy       string   `yaml:"strategy"`       // native | bot
	Method         string   `yaml:"method"`         // squash | merge | rebase
	BlockingLabels []string `yaml:"blocking_labels"`
}

func (o *MergeOptions) applyDefaults() {
	if o.Strategy == "" {
		o.Strategy = "native"
	}
	if o.Method == "" {
		o.Method = "squash"
	}
}

func (o *MergeOptions) validate() []ValidationIssue {
	var issues []ValidationIssue
	switch o.Method {
	case "squash", "merge", "rebase", "":
	default:
		issues = append(issues, ValidationIssue{
			Level:   "ERROR",
			Field:   "merge.method",
			Message: fmt.Sprintf("invalid value %q (must be squash, merge, or rebase)", o.Method),
		})
	}
	switch o.Strategy {
	case "native", "bot", "":
	default:
		issues = append(issues, ValidationIssue{
			Level:   "ERROR",
			Field:   "merge.strategy",
			Message: fmt.Sprintf("invalid value %q (must be native or bot)", o.Strategy),
		})
	}
	if o.Strategy == "native" && len(o.BlockingLabels) == 0 {
		issues = append(issues, ValidationIssue{
			Level:   "WARN",
			Field:   "merge.blocking_labels",
			Message: "empty — hold labels will not block auto-merge",
		})
	}
	return issues
}
