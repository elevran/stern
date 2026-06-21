package labels

// Implemented label plugins.
// LGTM and Approved are required for auto-merge; Hold and WIP block it;
// NeedsRebase is a blocking label applied by the rebase plugin.
const (
	LGTM     = "lgtm"
	Approved = "approved"

	Hold = "do-not-merge/hold"
	WIP  = "do-not-merge/wip"

	NeedsRebase = "needs-rebase"
)

// Lifecycle plugin — constants declared for forward compatibility, but the
// lifecycle plugin that would manage these labels is not yet implemented.
// Issue #76 tracks either removal or wiring up the plugin.
const (
	NeedsTriage = "needs-triage"

	LifecycleStale  = "lifecycle/stale"
	LifecycleRotten = "lifecycle/rotten"
	LifecycleFrozen = "lifecycle/frozen"
)

// Size plugin constants. SizePrefix is used by the implemented size plugin
// (internal/pr/size.go), which constructs the bucket label as
// SizePrefix + bucket. The named bucket constants below are not referenced
// anywhere in the codebase and are retained only as documentation of the
// canonical bucket names. The size plugin itself dynamically selects the
// bucket from configured SizeBuckets, so these constants do not need to be
// referenced for the plugin to function.
const (
	SizePrefix = "size/"
	SizeXS     = SizePrefix + "XS"
	SizeS      = SizePrefix + "S"
	SizeM      = SizePrefix + "M"
	SizeL      = SizePrefix + "L"
	SizeXL     = SizePrefix + "XL"
	SizeXXL    = SizePrefix + "XXL"
)
