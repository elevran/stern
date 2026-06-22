package github

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMockClient_EnableDisableAutoMerge_RecordsNodeIDs(t *testing.T) {
	m := NewMockClient()
	ctx := context.Background()

	require.NoError(t, m.EnableAutoMerge(ctx, "PR_node_1", "squash"))
	require.NoError(t, m.EnableAutoMerge(ctx, "PR_node_2", "squash"))
	assert.Equal(t, []string{"PR_node_1", "PR_node_2"}, m.AutoMergeEnabled)
	assert.Equal(t, 2, m.EnableAutoMergeCallCount)

	require.NoError(t, m.DisableAutoMerge(ctx, "PR_node_1"))
	assert.Equal(t, []string{"PR_node_1"}, m.AutoMergeDisabled)
	assert.Equal(t, 1, m.DisableAutoMergeCallCount)
}

func TestMockClient_EnableAutoMerge_ErrorInjection(t *testing.T) {
	want := errors.New("upstream down")
	m := NewMockClient()
	m.Errors = map[string]error{"EnableAutoMerge": want}

	err := m.EnableAutoMerge(context.Background(), "PR_node_1", "squash")
	require.ErrorIs(t, err, want)
	// Call count still increments before the error check, matching real-client semantics.
	assert.Equal(t, 1, m.EnableAutoMergeCallCount)
}

func TestMockClient_GetFileContent(t *testing.T) {
	m := NewMockClient()
	m.FileContent = map[string][]byte{
		"OWNERS@main": []byte("approvers:\n  - alice\n"),
	}
	ctx := context.Background()

	got, err := m.GetFileContent(ctx, "elevran", "stern", "OWNERS", "main")
	require.NoError(t, err)
	assert.Equal(t, []byte("approvers:\n  - alice\n"), got)

	// Missing file must surface as a 404 IsNotFoundError can recognise.
	_, err = m.GetFileContent(ctx, "elevran", "stern", "MISSING", "main")
	require.Error(t, err)
	assert.True(t, IsNotFoundError(err), "missing file should be IsNotFoundError, got %v", err)
}

func TestMockClient_CloseReopenIssue_RecordsNumbers(t *testing.T) {
	m := NewMockClient()
	ctx := context.Background()

	require.NoError(t, m.CloseIssue(ctx, "elevran", "stern", 7))
	require.NoError(t, m.ReopenIssue(ctx, "elevran", "stern", 7))
	require.NoError(t, m.CloseIssue(ctx, "elevran", "stern", 8))

	assert.Equal(t, []int{7, 8}, m.IssueClosed)
	assert.Equal(t, []int{7}, m.IssueReopened)
}

func TestMockClient_Milestones(t *testing.T) {
	m := NewMockClient()
	m.Milestones = map[int]Milestone{
		1: {Number: 1, Title: "v1.0"},
		2: {Number: 2, Title: "v2.0"},
	}
	ctx := context.Background()

	got, err := m.ListMilestones(ctx, "elevran", "stern")
	require.NoError(t, err)
	assert.Len(t, got, 2)
	assert.Contains(t, got, Milestone{Number: 1, Title: "v1.0"})

	require.NoError(t, m.SetMilestone(ctx, "elevran", "stern", 42, 1))
	assert.Equal(t, 1, m.IssueMilestone[42])
	assert.Equal(t, []MilestoneSetRecord{{Number: 42, MilestoneID: 1}}, m.MilestoneSet)

	require.NoError(t, m.ClearMilestone(ctx, "elevran", "stern", 42))
	_, exists := m.IssueMilestone[42]
	assert.False(t, exists, "ClearMilestone should remove the entry")
	assert.Equal(t, []int{42}, m.MilestoneCleared)
}

func TestMockClient_Assignees(t *testing.T) {
	m := NewMockClient()
	ctx := context.Background()

	require.NoError(t, m.AddAssignees(ctx, "elevran", "stern", 1, []string{"alice", "bob"}))
	assert.ElementsMatch(t, []string{"alice", "bob"}, m.Assignees[1])
	assert.Equal(t, []UsersRecord{{Number: 1, Users: []string{"alice", "bob"}}}, m.AssigneesAdded)

	// Re-adding the same user is idempotent — recordUsers dedupes.
	require.NoError(t, m.AddAssignees(ctx, "elevran", "stern", 1, []string{"alice"}))
	assert.ElementsMatch(t, []string{"alice", "bob"}, m.Assignees[1])

	require.NoError(t, m.RemoveAssignees(ctx, "elevran", "stern", 1, []string{"alice"}))
	assert.Equal(t, []string{"bob"}, m.Assignees[1])
	assert.Equal(t, []UsersRecord{{Number: 1, Users: []string{"alice"}}}, m.AssigneesRemoved)
}

func TestMockClient_Reviewers(t *testing.T) {
	m := NewMockClient()
	ctx := context.Background()

	require.NoError(t, m.RequestReviewers(ctx, "elevran", "stern", 1, []string{"carol"}))
	assert.Equal(t, []string{"carol"}, m.ReviewRequests[1])
	assert.Equal(t, []UsersRecord{{Number: 1, Users: []string{"carol"}}}, m.ReviewersRequested)

	require.NoError(t, m.RemoveReviewers(ctx, "elevran", "stern", 1, []string{"carol"}))
	assert.Empty(t, m.ReviewRequests[1])
	assert.Equal(t, []UsersRecord{{Number: 1, Users: []string{"carol"}}}, m.ReviewersRemoved)
}

func TestMockClient_CheckRuns(t *testing.T) {
	m := NewMockClient()
	m.CheckRuns = map[string][]CheckRun{
		"elevran/stern/deadbeef": {
			{ID: 100, Name: "build", Conclusion: "failure"},
			{ID: 101, Name: "test", Conclusion: "success"},
		},
	}
	ctx := context.Background()

	got, err := m.ListCheckRuns(ctx, "elevran", "stern", "deadbeef")
	require.NoError(t, err)
	assert.Len(t, got, 2)

	// Unknown ref returns nil slice — matches zero-value map lookup behaviour.
	got, err = m.ListCheckRuns(ctx, "elevran", "stern", "missing-sha")
	require.NoError(t, err)
	assert.Empty(t, got)

	require.NoError(t, m.RerunCheckRun(ctx, "elevran", "stern", 100))
	assert.Equal(t, []int64{100}, m.RerunCheckRuns)
}

func TestMockClient_ListOpenItems(t *testing.T) {
	m := NewMockClient()
	m.Items = []Item{
		{Number: 1, IsPR: false, HasMilestone: false},
		{Number: 2, IsPR: true, HasMilestone: true},
	}
	got, err := m.ListOpenItems(context.Background(), "elevran", "stern")
	require.NoError(t, err)
	assert.Len(t, got, 2)
}

func TestMockClient_CreatePullRequest(t *testing.T) {
	m := NewMockClient()
	ctx := context.Background()

	n, err := m.CreatePullRequest(ctx, "elevran", "stern", "cherry-pick: foo", "feature", "release-1.2", "body text")
	require.NoError(t, err)
	assert.Positive(t, n, "CreatePullRequest should return a positive PR number")
	assert.Contains(t, m.CreatedPRs, n)
	assert.Equal(t, "cherry-pick: foo", m.CreatedPRs[n].Title)
	require.Len(t, m.CreatedPRMeta, 1)
	assert.Equal(t, "feature", m.CreatedPRMeta[0].Head)
	assert.Equal(t, "release-1.2", m.CreatedPRMeta[0].Base)
	assert.Equal(t, "body text", m.CreatedPRMeta[0].Body)

	// Second call gets a different (higher) number.
	n2, err := m.CreatePullRequest(ctx, "elevran", "stern", "second", "h", "b", "x")
	require.NoError(t, err)
	assert.NotEqual(t, n, n2)
}

func TestMockClient_OrgMemberAndWriteAccess(t *testing.T) {
	m := NewMockClient()
	m.OrgMembers = map[string]bool{"elevran/alice": true}
	m.WriteAccess = map[string]bool{"elevran/stern/bob": true}
	ctx := context.Background()

	isMember, err := m.IsOrgMember(ctx, "elevran", "alice")
	require.NoError(t, err)
	assert.True(t, isMember)

	isMember, err = m.IsOrgMember(ctx, "elevran", "nobody")
	require.NoError(t, err)
	assert.False(t, isMember)

	hasWrite, err := m.HasWriteAccess(ctx, "elevran", "stern", "bob")
	require.NoError(t, err)
	assert.True(t, hasWrite)

	hasWrite, err = m.HasWriteAccess(ctx, "elevran", "stern", "alice")
	require.NoError(t, err)
	assert.False(t, hasWrite)
}
