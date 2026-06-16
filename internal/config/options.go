package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Options holds all stern configuration loaded from stern.yaml.
type Options struct {
	Org      string `yaml:"org"`
	Repo     string `yaml:"repo"`
	BotLogin string `yaml:"bot_login"`

	Plugins []string `yaml:"plugins"`

	Merge           MergeOptions           `yaml:"merge"`
	LGTM            LGTMOptions            `yaml:"lgtm"`
	Approve         ApproveOptions         `yaml:"approve"`
	CherryPick      CherryPickOptions      `yaml:"cherry_pick"`
	ReviewAssignment ReviewAssignmentOptions `yaml:"review_assignment"`
	Lifecycle       LifecycleOptions       `yaml:"lifecycle"`

	LabelDefinitions []LabelDefinition `yaml:"label_definitions"`
}

type MergeOptions struct {
	Strategy      string   `yaml:"strategy"`       // native | bot
	Method        string   `yaml:"method"`         // squash | merge | rebase
	BlockingLabels []string `yaml:"blocking_labels"`
}

type LGTMOptions struct {
	AllowSelfLGTM    bool `yaml:"allow_self_lgtm"`
	InvalidateOnPush bool `yaml:"invalidate_on_push"`
}

type ApproveOptions struct {
	AllowSelfApproval bool `yaml:"allow_self_approval"`
	InvalidateOnPush  bool `yaml:"invalidate_on_push"`
	RequireOwner      bool `yaml:"require_owner"`
}

type CherryPickOptions struct {
	AllowedBranchPattern string `yaml:"allowed_branch_pattern"`
	Command              string `yaml:"command"` // cherry-pick | cherrypick | cp
}

type ReviewAssignmentOptions struct {
	Enabled      bool   `yaml:"enabled"`
	LoadBalancing string `yaml:"load_balancing"`
}

type LifecycleOptions struct {
	StaleDays   int  `yaml:"stale_days"`
	RottenDays  int  `yaml:"rotten_days"`
	CloseRotten bool `yaml:"close_rotten"`
}

type LabelDefinition struct {
	Name        string `yaml:"name"`
	Color       string `yaml:"color"`
	Description string `yaml:"description"`
}

// LoadFromFile reads and parses a stern YAML config file.
func LoadFromFile(path string) (*Options, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config %s: %w", path, err)
	}
	var opts Options
	if err := yaml.Unmarshal(data, &opts); err != nil {
		return nil, fmt.Errorf("parsing config %s: %w", path, err)
	}
	opts.applyDefaults()
	return &opts, nil
}

func (o *Options) applyDefaults() {
	if o.BotLogin == "" {
		o.BotLogin = "github-actions[bot]"
	}
	if o.Merge.Strategy == "" {
		o.Merge.Strategy = "native"
	}
	if o.Merge.Method == "" {
		o.Merge.Method = "squash"
	}
	if o.CherryPick.Command == "" {
		o.CherryPick.Command = "cherry-pick"
	}
	if o.Lifecycle.StaleDays == 0 {
		o.Lifecycle.StaleDays = 90
	}
	if o.Lifecycle.RottenDays == 0 {
		o.Lifecycle.RottenDays = 30
	}
}

// Validate checks the options for errors. Returns nil if valid.
// Full validation is implemented in Task 0.5.
func (o *Options) Validate() []error {
	return nil
}

// HasPlugin reports whether the named plugin is enabled.
func (o *Options) HasPlugin(name string) bool {
	for _, p := range o.Plugins {
		if p == name {
			return true
		}
	}
	return false
}
