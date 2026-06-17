package config

// ReviewAssignmentOptions configures automatic reviewer assignment.
type ReviewAssignmentOptions struct {
	Enabled       bool   `yaml:"enabled"`
	LoadBalancing string `yaml:"load_balancing"`
}
