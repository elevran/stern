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

func (o *KindOptions) validate(pluginEnabled bool) []ValidationIssue {
	if pluginEnabled && len(o.Values) == 0 {
		return []ValidationIssue{{
			Level:   "WARN",
			Field:   "kind.values",
			Message: "kind plugin is enabled but values list is empty",
		}}
	}
	return nil
}

func (o *AreaOptions) validate(pluginEnabled bool) []ValidationIssue {
	if pluginEnabled && len(o.Values) == 0 {
		return []ValidationIssue{{
			Level:   "WARN",
			Field:   "area.values",
			Message: "area plugin is enabled but values list is empty",
		}}
	}
	return nil
}

func (o *PriorityOptions) validate(pluginEnabled bool) []ValidationIssue {
	if pluginEnabled && len(o.Values) == 0 {
		return []ValidationIssue{{
			Level:   "WARN",
			Field:   "priority.values",
			Message: "priority plugin is enabled but values list is empty",
		}}
	}
	return nil
}
