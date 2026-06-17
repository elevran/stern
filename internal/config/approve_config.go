package config

// ApproveOptions configures the /approve slash command handler.
type ApproveOptions struct {
	AllowSelfApproval bool `yaml:"allow_self_approval"`
	InvalidateOnPush  bool `yaml:"invalidate_on_push"`
	RequireOwner      bool `yaml:"require_owner"`
}
