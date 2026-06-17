package config

// LGTMOptions configures the /lgtm slash command handler.
type LGTMOptions struct {
	AllowSelfLGTM    bool `yaml:"allow_self_lgtm"`
	InvalidateOnPush bool `yaml:"invalidate_on_push"`
}
