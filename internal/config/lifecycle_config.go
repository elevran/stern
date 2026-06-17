package config

// LifecycleOptions configures stale/rotten lifecycle management.
type LifecycleOptions struct {
	StaleDays   int  `yaml:"stale_days"`
	RottenDays  int  `yaml:"rotten_days"`
	CloseRotten bool `yaml:"close_rotten"`
}

func (o *LifecycleOptions) applyDefaults() {
	if o.StaleDays == 0 {
		o.StaleDays = 90
	}
	if o.RottenDays == 0 {
		o.RottenDays = 30
	}
}

func (o *LifecycleOptions) validate() []ValidationIssue {
	var issues []ValidationIssue
	if o.StaleDays < 0 {
		issues = append(issues, ValidationIssue{
			Level:   "ERROR",
			Field:   "lifecycle.stale_days",
			Message: "must be a positive integer",
		})
	}
	if o.RottenDays < 0 {
		issues = append(issues, ValidationIssue{
			Level:   "ERROR",
			Field:   "lifecycle.rotten_days",
			Message: "must be a positive integer",
		})
	}
	return issues
}
