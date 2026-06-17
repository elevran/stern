package config

// LabelDefinition describes a single GitHub label managed by stern.
type LabelDefinition struct {
	Name        string `yaml:"name"`
	Color       string `yaml:"color"`
	Description string `yaml:"description"`
}
