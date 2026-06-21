package config

// KindOptions configures the /kind slash command handler.
type KindOptions struct {
	Values []string `yaml:"values"` // e.g. [bug, feature, docs]
}

// AreaOptions configures the /area slash command handler.
type AreaOptions struct {
	Values []string `yaml:"values"`
}

// PriorityOptions configures the /priority slash command handler.
type PriorityOptions struct {
	Values []string `yaml:"values"` // e.g. [P0, P1, P2]
}

func validateLabelValues(field string, values []string, enabled bool) []ValidationIssue {
	if enabled && len(values) == 0 {
		return []ValidationIssue{{
			Level:   "WARN",
			Field:   field,
			Message: field + " plugin is enabled but values list is empty",
		}}
	}
	return nil
}

func (o *KindOptions) validate(pluginEnabled bool) []ValidationIssue {
	return validateLabelValues("kind", o.Values, pluginEnabled)
}

func (o *AreaOptions) validate(pluginEnabled bool) []ValidationIssue {
	return validateLabelValues("area", o.Values, pluginEnabled)
}

func (o *PriorityOptions) validate(pluginEnabled bool) []ValidationIssue {
	return validateLabelValues("priority", o.Values, pluginEnabled)
}
