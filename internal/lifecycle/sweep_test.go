package lifecycle_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/elevran/stern/internal/config"
	"github.com/elevran/stern/internal/github"
	"github.com/elevran/stern/internal/labels"
	"github.com/elevran/stern/internal/lifecycle"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fixedNow is the reference "now" used by all tests; UpdatedAt is computed
// relative to it via daysBefore.
var fixedNow = time.Date(2026, 6, 21, 12, 0, 0, 0, time.UTC)

func daysBefore(n int) time.Time {
	return fixedNow.Add(-time.Duration(n) * 24 * time.Hour)
}

func baseOpts() *config.Options {
	return &config.Options{
		Lifecycle: config.LifecycleOptions{
			Enabled:       true,
			StaleDays:     90,
			RottenDays:    30,
			CloseAfter:    30,
			StaleComment:  "stale msg",
			RottenComment: "rotten msg",
			CloseComment:  "close msg",
		},
	}
}

func TestSweep_FreshItem_NoAction(t *testing.T) {
	ghc := github.NewMockClient()
	ghc.Items = []github.Item{
		{Number: 1, UpdatedAt: daysBefore(1)},
	}
	require.NoError(t, lifecycle.Sweep(context.Background(), ghc, "o", "r", baseOpts(), fixedNow))

	assert.Empty(t, ghc.IssueLabels[1], "fresh item should not get any labels")
	assert.Empty(t, ghc.Comments)
	assert.Empty(t, ghc.IssueClosed)
}

func TestSweep_StaleThreshold_AddsStale(t *testing.T) {
	ghc := github.NewMockClient()
	ghc.Items = []github.Item{
		{Number: 1, UpdatedAt: daysBefore(90)},
	}
	require.NoError(t, lifecycle.Sweep(context.Background(), ghc, "o", "r", baseOpts(), fixedNow))

	assert.True(t, ghc.IssueLabels[1][labels.LifecycleStale])
	require.Len(t, ghc.Comments, 1)
	assert.Equal(t, "stale msg", ghc.Comments[0].Body)
}

func TestSweep_StaleItem_AtRottenThreshold_AddsRotten(t *testing.T) {
	ghc := github.NewMockClient()
	ghc.Items = []github.Item{
		{Number: 1, UpdatedAt: daysBefore(30), Labels: []string{labels.LifecycleStale}},
	}
	// Seed the existing stale label so RemoveLabel can clear it.
	ghc.IssueLabels[1] = map[string]bool{labels.LifecycleStale: true}

	require.NoError(t, lifecycle.Sweep(context.Background(), ghc, "o", "r", baseOpts(), fixedNow))

	assert.False(t, ghc.IssueLabels[1][labels.LifecycleStale], "stale should be removed")
	assert.True(t, ghc.IssueLabels[1][labels.LifecycleRotten])
	require.Len(t, ghc.Comments, 1)
	assert.Equal(t, "rotten msg", ghc.Comments[0].Body)
}

func TestSweep_RottenItem_AtCloseThreshold_Closes(t *testing.T) {
	ghc := github.NewMockClient()
	ghc.Items = []github.Item{
		{Number: 1, UpdatedAt: daysBefore(30), Labels: []string{labels.LifecycleRotten}},
	}
	require.NoError(t, lifecycle.Sweep(context.Background(), ghc, "o", "r", baseOpts(), fixedNow))

	require.Len(t, ghc.Comments, 1)
	assert.Equal(t, "close msg", ghc.Comments[0].Body)
	require.Equal(t, []int{1}, ghc.IssueClosed)
}

func TestSweep_Frozen_SkipsEntirely(t *testing.T) {
	ghc := github.NewMockClient()
	ghc.Items = []github.Item{
		{Number: 1, UpdatedAt: daysBefore(365), Labels: []string{labels.LifecycleFrozen}},
		{Number: 2, UpdatedAt: daysBefore(365), Labels: []string{labels.LifecycleFrozen, labels.LifecycleStale}},
	}
	require.NoError(t, lifecycle.Sweep(context.Background(), ghc, "o", "r", baseOpts(), fixedNow))

	assert.Empty(t, ghc.IssueLabels[1])
	assert.Empty(t, ghc.IssueLabels[2], "frozen takes precedence even with stale also present")
	assert.Empty(t, ghc.Comments)
	assert.Empty(t, ghc.IssueClosed)
}

func TestSweep_PRWithMilestone_Skipped(t *testing.T) {
	ghc := github.NewMockClient()
	ghc.Items = []github.Item{
		{Number: 1, UpdatedAt: daysBefore(365), IsPR: true, HasMilestone: true},
	}
	require.NoError(t, lifecycle.Sweep(context.Background(), ghc, "o", "r", baseOpts(), fixedNow))

	assert.Empty(t, ghc.IssueLabels[1])
	assert.Empty(t, ghc.IssueClosed)
}

func TestSweep_CloseStale_StaleClosesAtRottenThreshold(t *testing.T) {
	ghc := github.NewMockClient()
	ghc.Items = []github.Item{
		{Number: 1, UpdatedAt: daysBefore(30), Labels: []string{labels.LifecycleStale}},
	}
	opts := baseOpts()
	opts.Lifecycle.CloseStale = true

	require.NoError(t, lifecycle.Sweep(context.Background(), ghc, "o", "r", opts, fixedNow))

	require.Equal(t, []int{1}, ghc.IssueClosed)
	assert.False(t, ghc.IssueLabels[1][labels.LifecycleRotten], "rotten label should never be applied in close_stale mode")
	require.Len(t, ghc.Comments, 1)
	assert.Equal(t, "close msg", ghc.Comments[0].Body)
}

func TestSweep_PRSpecificOverride(t *testing.T) {
	ghc := github.NewMockClient()
	ghc.Items = []github.Item{
		{Number: 1, UpdatedAt: daysBefore(30), IsPR: true}, // hits PR override (30)
		{Number: 2, UpdatedAt: daysBefore(30)},             // issue: 30 < global 90, no action
	}
	opts := baseOpts()
	opts.Lifecycle.PullRequests = config.LifecycleItemOptions{StaleDays: 30}

	require.NoError(t, lifecycle.Sweep(context.Background(), ghc, "o", "r", opts, fixedNow))

	assert.True(t, ghc.IssueLabels[1][labels.LifecycleStale], "PR should hit the 30-day override")
	assert.Empty(t, ghc.IssueLabels[2], "issue still under 90-day global default")
}

func TestSweep_IssueSpecificOverride(t *testing.T) {
	ghc := github.NewMockClient()
	ghc.Items = []github.Item{
		{Number: 1, UpdatedAt: daysBefore(45)},
	}
	opts := baseOpts()
	opts.Lifecycle.Issues = config.LifecycleItemOptions{StaleDays: 45}

	require.NoError(t, lifecycle.Sweep(context.Background(), ghc, "o", "r", opts, fixedNow))

	assert.True(t, ghc.IssueLabels[1][labels.LifecycleStale], "issue should hit the 45-day override")
}

func TestSweep_NoCommentTemplate_NoCommentPosted(t *testing.T) {
	ghc := github.NewMockClient()
	ghc.Items = []github.Item{
		{Number: 1, UpdatedAt: daysBefore(90)},
	}
	opts := baseOpts()
	opts.Lifecycle.StaleComment = "" // suppress

	require.NoError(t, lifecycle.Sweep(context.Background(), ghc, "o", "r", opts, fixedNow))

	assert.True(t, ghc.IssueLabels[1][labels.LifecycleStale], "label should still be applied")
	assert.Empty(t, ghc.Comments, "empty template should suppress the comment")
}

func TestSweep_ListOpenItemsError_Fatal(t *testing.T) {
	ghc := github.NewMockClient()
	ghc.Errors["ListOpenItems"] = errors.New("boom")

	err := lifecycle.Sweep(context.Background(), ghc, "o", "r", baseOpts(), fixedNow)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "boom")
}

func TestSweep_PerItemError_LoggedAndOthersProcessed(t *testing.T) {
	ghc := github.NewMockClient()
	ghc.Items = []github.Item{
		{Number: 1, UpdatedAt: daysBefore(90)}, // would transition to stale
		{Number: 2, UpdatedAt: daysBefore(90)}, // would transition to stale
	}
	// Force AddLabels to fail for every item — but Sweep must return nil
	// and proceed past the failure so the second item is still attempted.
	ghc.Errors["AddLabels"] = errors.New("transient")

	err := lifecycle.Sweep(context.Background(), ghc, "o", "r", baseOpts(), fixedNow)
	require.NoError(t, err, "Sweep should not abort on per-item failures")
}

func TestSweep_BoundaryBelowThreshold_NoAction(t *testing.T) {
	// 89 days < stale_days=90 → no transition
	ghc := github.NewMockClient()
	ghc.Items = []github.Item{
		{Number: 1, UpdatedAt: daysBefore(89)},
	}
	require.NoError(t, lifecycle.Sweep(context.Background(), ghc, "o", "r", baseOpts(), fixedNow))
	assert.Empty(t, ghc.IssueLabels[1], "89 days is below the 90-day threshold")
}

func TestSweep_StaleItemBelowRotten_NoAction(t *testing.T) {
	ghc := github.NewMockClient()
	ghc.Items = []github.Item{
		{Number: 1, UpdatedAt: daysBefore(15), Labels: []string{labels.LifecycleStale}},
	}
	ghc.IssueLabels[1] = map[string]bool{labels.LifecycleStale: true}

	require.NoError(t, lifecycle.Sweep(context.Background(), ghc, "o", "r", baseOpts(), fixedNow))
	assert.True(t, ghc.IssueLabels[1][labels.LifecycleStale], "stale should remain")
	assert.False(t, ghc.IssueLabels[1][labels.LifecycleRotten])
	assert.Empty(t, ghc.IssueClosed)
}
