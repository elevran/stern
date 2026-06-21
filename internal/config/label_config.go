package config

import (
	"fmt"
	"regexp"
)

// LabelDefinition describes a single GitHub label managed by stern.
type LabelDefinition struct {
	Name        string `yaml:"name"`
	Color       string `yaml:"color"`
	Description string `yaml:"description"`
}

// labelColorRegex matches the 6-digit hex color strings that GitHub accepts
// for label colors (without a leading "#").
var labelColorRegex = regexp.MustCompile(`^[0-9a-fA-F]{6}$`)

// validate checks a single LabelDefinition for errors. Issues are returned
// with field paths like "label_definitions[3].name".
func (l LabelDefinition) validate(index int) []ValidationIssue {
	var issues []ValidationIssue
	prefix := fmt.Sprintf("label_definitions[%d]", index)
	if l.Name == "" {
		issues = append(issues, ValidationIssue{
			Level:   "ERROR",
			Field:   prefix + ".name",
			Message: "must not be empty",
		})
	}
	if !labelColorRegex.MatchString(l.Color) {
		issues = append(issues, ValidationIssue{
			Level:   "ERROR",
			Field:   prefix + ".color",
			Message: fmt.Sprintf("invalid hex color %q (must be 6 hex digits, no #)", l.Color),
		})
	}
	if len(l.Description) > 100 {
		issues = append(issues, ValidationIssue{
			Level:   "ERROR",
			Field:   prefix + ".description",
			Message: "exceeds GitHub's 100-character limit",
		})
	}
	return issues
}
