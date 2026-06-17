package config

// CherryPickOptions configures the /cherry-pick slash command handler.
type CherryPickOptions struct {
	AllowedBranchPattern string `yaml:"allowed_branch_pattern"`
	Command              string `yaml:"command"` // cherry-pick | cherrypick | cp
}

func (o *CherryPickOptions) applyDefaults() {
	if o.Command == "" {
		o.Command = "cherry-pick"
	}
}
