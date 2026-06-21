package config

// LifecycleItemOptions holds timing and comment overrides for one item type
// (issues or pull_requests). Zero values mean "inherit from the global defaults".
type LifecycleItemOptions struct {
	StaleDays     int    `yaml:"stale_days"`
	RottenDays    int    `yaml:"rotten_days"`
	CloseAfter    int    `yaml:"close_after"`
	StaleComment  string `yaml:"stale_comment"`
	RottenComment string `yaml:"rotten_comment"`
	CloseComment  string `yaml:"close_comment"`
}

// LifecycleOptions configures stale/rotten lifecycle management.
type LifecycleOptions struct {
	Enabled bool `yaml:"enabled"`

	// Global defaults — used when Issues/PullRequests fields are zero.
	StaleDays     int    `yaml:"stale_days"`
	RottenDays    int    `yaml:"rotten_days"`
	CloseAfter    int    `yaml:"close_after"`
	CloseStale    bool   `yaml:"close_stale"` // close directly from stale, skip rotten
	StaleComment  string `yaml:"stale_comment"`
	RottenComment string `yaml:"rotten_comment"`
	CloseComment  string `yaml:"close_comment"`

	// Per-type overrides. Non-zero fields take precedence over global defaults.
	Issues       LifecycleItemOptions `yaml:"issues"`
	PullRequests LifecycleItemOptions `yaml:"pull_requests"`
}

// ForIssues returns the resolved LifecycleItemOptions for issues,
// falling back to global defaults for any zero fields.
func (o *LifecycleOptions) ForIssues() LifecycleItemOptions {
	return o.resolve(o.Issues)
}

// ForPRs returns the resolved LifecycleItemOptions for pull requests,
// falling back to global defaults for any zero fields.
func (o *LifecycleOptions) ForPRs() LifecycleItemOptions {
	return o.resolve(o.PullRequests)
}

func (o *LifecycleOptions) resolve(override LifecycleItemOptions) LifecycleItemOptions {
	r := override
	if r.StaleDays == 0 {
		r.StaleDays = o.StaleDays
	}
	if r.RottenDays == 0 {
		r.RottenDays = o.RottenDays
	}
	if r.CloseAfter == 0 {
		r.CloseAfter = o.CloseAfter
	}
	if r.StaleComment == "" {
		r.StaleComment = o.StaleComment
	}
	if r.RottenComment == "" {
		r.RottenComment = o.RottenComment
	}
	if r.CloseComment == "" {
		r.CloseComment = o.CloseComment
	}
	return r
}

func (o *LifecycleOptions) applyDefaults() {
	if o.StaleDays == 0 {
		o.StaleDays = 90
	}
	if o.RottenDays == 0 {
		o.RottenDays = 30
	}
	if o.CloseAfter == 0 {
		o.CloseAfter = 30
	}
}

func (o *LifecycleOptions) validate() []ValidationIssue {
	var issues []ValidationIssue
	for _, check := range []struct {
		name string
		val  int
	}{
		{"lifecycle.stale_days", o.StaleDays},
		{"lifecycle.rotten_days", o.RottenDays},
		{"lifecycle.close_after", o.CloseAfter},
	} {
		if check.val <= 0 {
			issues = append(issues, ValidationIssue{
				Level:   "ERROR",
				Field:   check.name,
				Message: "must be a positive integer",
			})
		}
	}
	// Per-type overrides: zero means "inherit", so only explicitly-negative
	// values are rejected. Positive values are valid overrides.
	issues = append(issues, validateItemOverride("lifecycle.issues", o.Issues)...)
	issues = append(issues, validateItemOverride("lifecycle.pull_requests", o.PullRequests)...)
	return issues
}

// validateItemOverride checks that any explicitly-set per-type field is
// non-negative. Zero is permitted because it signals "inherit the global
// default" — see resolve() for the fallback logic.
func validateItemOverride(prefix string, o LifecycleItemOptions) []ValidationIssue {
	var issues []ValidationIssue
	for _, check := range []struct {
		name string
		val  int
	}{
		{prefix + ".stale_days", o.StaleDays},
		{prefix + ".rotten_days", o.RottenDays},
		{prefix + ".close_after", o.CloseAfter},
	} {
		if check.val < 0 {
			issues = append(issues, ValidationIssue{
				Level:   "ERROR",
				Field:   check.name,
				Message: "must be 0 or a positive integer (0 means inherit the global default)",
			})
		}
	}
	return issues
}
