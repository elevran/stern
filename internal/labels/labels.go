package labels

const (
	LGTM     = "lgtm"
	Approved = "approved"

	Hold = "do-not-merge/hold"
	WIP  = "do-not-merge/wip"

	NeedsRebase = "needs-rebase"
	NeedsTriage = "needs-triage"

	LifecycleStale  = "lifecycle/stale"
	LifecycleRotten = "lifecycle/rotten"
	LifecycleFrozen = "lifecycle/frozen"

	SizePrefix = "size/"
	SizeXS     = SizePrefix + "XS"
	SizeS      = SizePrefix + "S"
	SizeM      = SizePrefix + "M"
	SizeL      = SizePrefix + "L"
	SizeXL     = SizePrefix + "XL"
	SizeXXL    = SizePrefix + "XXL"
)
