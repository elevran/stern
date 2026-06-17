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

	Merge            MergeOptions            `yaml:"merge"`
	LGTM             LGTMOptions             `yaml:"lgtm"`
	Approve          ApproveOptions          `yaml:"approve"`
	CherryPick       CherryPickOptions       `yaml:"cherry_pick"`
	ReviewAssignment ReviewAssignmentOptions `yaml:"review_assignment"`
	Lifecycle        LifecycleOptions        `yaml:"lifecycle"`

	LabelDefinitions []LabelDefinition `yaml:"label_definitions"`
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
	o.Merge.applyDefaults()
	o.CherryPick.applyDefaults()
	o.Lifecycle.applyDefaults()
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
