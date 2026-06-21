package config

import "fmt"

// ReviewAssignmentOptions configures automatic reviewer assignment.
type ReviewAssignmentOptions struct {
	Enabled       bool   `yaml:"enabled"`
	LoadBalancing string `yaml:"load_balancing"`
	Count         int    `yaml:"count"`
}

// applyDefaults sets Count to 1 when unset.
func (o *ReviewAssignmentOptions) applyDefaults() {
	if o.Count == 0 {
		o.Count = 1
	}
}

func (o *ReviewAssignmentOptions) validate() []ValidationIssue {
	switch o.LoadBalancing {
	case "", "round-robin", "least-busy":
		return nil
	default:
		return []ValidationIssue{{
			Level:   "ERROR",
			Field:   "review_assignment.load_balancing",
			Message: fmt.Sprintf("invalid value %q (must be round-robin or least-busy)", o.LoadBalancing),
		}}
	}
}
