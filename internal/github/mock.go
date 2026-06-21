package github

import (
	"context"
	"fmt"
)

// MockClient is an in-process mock for tests. Zero value is usable.
type MockClient struct {
	// Pre-loaded read state.
	RepoLabels      map[string]Label      // label name -> label
	PullRequests    map[int]*PullRequest  // PR number -> PR
	PRFiles         map[int][]string      // PR number -> filenames
	FileContent     map[string][]byte     // "path@ref" -> content
	OrgMembers      map[string]bool       // "org/user" -> is member
	WriteAccess     map[string]bool       // "owner/repo/user" -> has write
	Milestones      map[int]Milestone     // milestone number -> Milestone
	FailedCheckRuns map[string][]CheckRun // "owner/repo/sha" -> failed check runs
	Reviews         map[int][]Review      // PR number -> review records

	// Mutable state modified by calls.
	IssueLabels    map[int]map[string]bool // issue number -> set of label names
	IssueMilestone map[int]int             // issue number -> milestone number (0 = none)
	Assignees      map[int][]string        // issue number -> assigned users
	ReviewRequests map[int][]string        // PR number -> requested reviewers

	// Call records for assertions.
	Reactions          []ReactionRecord
	Comments           []CommentRecord
	AutoMergeEnabled   []string // nodeIDs passed to EnableAutoMerge
	AutoMergeDisabled  []string // nodeIDs passed to DisableAutoMerge
	IssueClosed        []int    // issue numbers passed to CloseIssue
	IssueReopened      []int    // issue numbers passed to ReopenIssue
	MilestoneSet       []MilestoneSetRecord
	MilestoneCleared   []int   // issue numbers passed to ClearMilestone
	AssigneesAdded     []UsersRecord
	AssigneesRemoved   []UsersRecord
	ReviewersRequested []UsersRecord
	ReviewersRemoved   []UsersRecord
	RerunCheckRuns   []int64        // check run IDs passed to RerunCheckRun
	ReviewsPosted    []ReviewRecord
	ReviewsDismissed []int64 // reviewIDs passed to DismissPullRequestReview

	// Return errors for specific method names.
	Errors map[string]error
}

type ReactionRecord struct {
	CommentID int64
	Content   string
}

type CommentRecord struct {
	Number int
	Body   string
}

type MilestoneSetRecord struct {
	Number      int
	MilestoneID int
}

type UsersRecord struct {
	Number int
	Users  []string
}

type ReviewRecord struct {
	Number int
	Event  string
	Body   string
	ID     int64
}

func NewMockClient() *MockClient {
	return &MockClient{
		RepoLabels:      make(map[string]Label),
		PullRequests:    make(map[int]*PullRequest),
		PRFiles:         make(map[int][]string),
		FileContent:     make(map[string][]byte),
		OrgMembers:      make(map[string]bool),
		WriteAccess:     make(map[string]bool),
		Milestones:      make(map[int]Milestone),
		FailedCheckRuns: make(map[string][]CheckRun),
		Reviews:         make(map[int][]Review),
		IssueLabels:     make(map[int]map[string]bool),
		IssueMilestone:  make(map[int]int),
		Assignees:       make(map[int][]string),
		ReviewRequests:  make(map[int][]string),
		Errors:          make(map[string]error),
	}
}

func (m *MockClient) err(method string) error {
	return m.Errors[method]
}

func (m *MockClient) ListRepoLabels(_ context.Context, _, _ string) ([]Label, error) {
	if err := m.err("ListRepoLabels"); err != nil {
		return nil, err
	}
	labels := make([]Label, 0, len(m.RepoLabels))
	for _, l := range m.RepoLabels {
		labels = append(labels, l)
	}
	return labels, nil
}

func (m *MockClient) CreateLabel(_ context.Context, _, _ string, label Label) error {
	if err := m.err("CreateLabel"); err != nil {
		return err
	}
	m.RepoLabels[label.Name] = label
	return nil
}

func (m *MockClient) UpdateLabel(_ context.Context, _, _, name string, label Label) error {
	if err := m.err("UpdateLabel"); err != nil {
		return err
	}
	m.RepoLabels[name] = label
	return nil
}

func (m *MockClient) DeleteLabel(_ context.Context, _, _, name string) error {
	if err := m.err("DeleteLabel"); err != nil {
		return err
	}
	delete(m.RepoLabels, name)
	return nil
}

func (m *MockClient) AddLabels(_ context.Context, _, _ string, number int, labels []string) error {
	if err := m.err("AddLabels"); err != nil {
		return err
	}
	if m.IssueLabels[number] == nil {
		m.IssueLabels[number] = make(map[string]bool)
	}
	for _, l := range labels {
		m.IssueLabels[number][l] = true
	}
	return nil
}

func (m *MockClient) RemoveLabel(_ context.Context, _, _ string, number int, label string) error {
	if err := m.err("RemoveLabel"); err != nil {
		return err
	}
	if m.IssueLabels[number] != nil {
		delete(m.IssueLabels[number], label)
	}
	return nil
}

func (m *MockClient) GetPullRequest(_ context.Context, _, _ string, number int) (PullRequest, error) {
	if err := m.err("GetPullRequest"); err != nil {
		return PullRequest{}, err
	}
	pr, ok := m.PullRequests[number]
	if !ok {
		return PullRequest{}, fmt.Errorf("PR %d not found", number)
	}
	return *pr, nil
}

func (m *MockClient) ListPullRequestFiles(_ context.Context, _, _ string, number int) ([]string, error) {
	if err := m.err("ListPullRequestFiles"); err != nil {
		return nil, err
	}
	return m.PRFiles[number], nil
}

func (m *MockClient) CreateCommentReaction(_ context.Context, _, _ string, commentID int64, content string) error {
	if err := m.err("CreateCommentReaction"); err != nil {
		return err
	}
	m.Reactions = append(m.Reactions, ReactionRecord{CommentID: commentID, Content: content})
	return nil
}

func (m *MockClient) CreateIssueComment(_ context.Context, _, _ string, number int, body string) error {
	if err := m.err("CreateIssueComment"); err != nil {
		return err
	}
	m.Comments = append(m.Comments, CommentRecord{Number: number, Body: body})
	return nil
}

func (m *MockClient) IsOrgMember(_ context.Context, org, user string) (bool, error) {
	if err := m.err("IsOrgMember"); err != nil {
		return false, err
	}
	return m.OrgMembers[org+"/"+user], nil
}

func (m *MockClient) HasWriteAccess(_ context.Context, owner, repo, user string) (bool, error) {
	if err := m.err("HasWriteAccess"); err != nil {
		return false, err
	}
	return m.WriteAccess[owner+"/"+repo+"/"+user], nil
}

func (m *MockClient) EnableAutoMerge(_ context.Context, nodeID string, _ string) error {
	if err := m.err("EnableAutoMerge"); err != nil {
		return err
	}
	m.AutoMergeEnabled = append(m.AutoMergeEnabled, nodeID)
	return nil
}

func (m *MockClient) DisableAutoMerge(_ context.Context, nodeID string) error {
	if err := m.err("DisableAutoMerge"); err != nil {
		return err
	}
	m.AutoMergeDisabled = append(m.AutoMergeDisabled, nodeID)
	return nil
}

func (m *MockClient) GetFileContent(_ context.Context, _, _, path, ref string) ([]byte, error) {
	if err := m.err("GetFileContent"); err != nil {
		return nil, err
	}
	key := path + "@" + ref
	content, ok := m.FileContent[key]
	if !ok {
		return nil, fmt.Errorf("file %q not found at ref %q", path, ref)
	}
	return content, nil
}

func (m *MockClient) CloseIssue(_ context.Context, _, _ string, number int) error {
	if err := m.err("CloseIssue"); err != nil {
		return err
	}
	m.IssueClosed = append(m.IssueClosed, number)
	return nil
}

func (m *MockClient) ReopenIssue(_ context.Context, _, _ string, number int) error {
	if err := m.err("ReopenIssue"); err != nil {
		return err
	}
	m.IssueReopened = append(m.IssueReopened, number)
	return nil
}

func (m *MockClient) ListMilestones(_ context.Context, _, _ string) ([]Milestone, error) {
	if err := m.err("ListMilestones"); err != nil {
		return nil, err
	}
	out := make([]Milestone, 0, len(m.Milestones))
	for _, ms := range m.Milestones {
		out = append(out, ms)
	}
	return out, nil
}

func (m *MockClient) SetMilestone(_ context.Context, _, _ string, number int, milestoneID int) error {
	if err := m.err("SetMilestone"); err != nil {
		return err
	}
	m.IssueMilestone[number] = milestoneID
	m.MilestoneSet = append(m.MilestoneSet, MilestoneSetRecord{Number: number, MilestoneID: milestoneID})
	return nil
}

func (m *MockClient) ClearMilestone(_ context.Context, _, _ string, number int) error {
	if err := m.err("ClearMilestone"); err != nil {
		return err
	}
	delete(m.IssueMilestone, number)
	m.MilestoneCleared = append(m.MilestoneCleared, number)
	return nil
}
func (m *MockClient) AddAssignees(_ context.Context, _, _ string, number int, users []string) error {
	if err := m.err("AddAssignees"); err != nil {
		return err
	}
	existing := make(map[string]bool)
	for _, u := range m.Assignees[number] {
		existing[u] = true
	}
	for _, u := range users {
		existing[u] = true
	}
	merged := make([]string, 0, len(existing))
	for u := range existing {
		merged = append(merged, u)
	}
	m.Assignees[number] = merged
	m.AssigneesAdded = append(m.AssigneesAdded, UsersRecord{Number: number, Users: users})
	return nil
}

func (m *MockClient) RemoveAssignees(_ context.Context, _, _ string, number int, users []string) error {
	if err := m.err("RemoveAssignees"); err != nil {
		return err
	}
	toRemove := make(map[string]bool, len(users))
	for _, u := range users {
		toRemove[u] = true
	}
	remaining := make([]string, 0, len(m.Assignees[number]))
	for _, u := range m.Assignees[number] {
		if !toRemove[u] {
			remaining = append(remaining, u)
		}
	}
	m.Assignees[number] = remaining
	m.AssigneesRemoved = append(m.AssigneesRemoved, UsersRecord{Number: number, Users: users})
	return nil
}

func (m *MockClient) RequestReviewers(_ context.Context, _, _ string, number int, users []string) error {
	if err := m.err("RequestReviewers"); err != nil {
		return err
	}
	existing := make(map[string]bool)
	for _, u := range m.ReviewRequests[number] {
		existing[u] = true
	}
	for _, u := range users {
		existing[u] = true
	}
	merged := make([]string, 0, len(existing))
	for u := range existing {
		merged = append(merged, u)
	}
	m.ReviewRequests[number] = merged
	m.ReviewersRequested = append(m.ReviewersRequested, UsersRecord{Number: number, Users: users})
	return nil
}

func (m *MockClient) RemoveReviewers(_ context.Context, _, _ string, number int, users []string) error {
	if err := m.err("RemoveReviewers"); err != nil {
		return err
	}
	toRemove := make(map[string]bool, len(users))
	for _, u := range users {
		toRemove[u] = true
	}
	remaining := make([]string, 0, len(m.ReviewRequests[number]))
	for _, u := range m.ReviewRequests[number] {
		if !toRemove[u] {
			remaining = append(remaining, u)
		}
	}
	m.ReviewRequests[number] = remaining
	m.ReviewersRemoved = append(m.ReviewersRemoved, UsersRecord{Number: number, Users: users})
	return nil
}

func (m *MockClient) ListFailedCheckRuns(_ context.Context, owner, repo, sha string) ([]CheckRun, error) {
	if err := m.err("ListFailedCheckRuns"); err != nil {
		return nil, err
	}
	return m.FailedCheckRuns[owner+"/"+repo+"/"+sha], nil
}

func (m *MockClient) RerunCheckRun(_ context.Context, _, _ string, id int64) error {
	if err := m.err("RerunCheckRun"); err != nil {
		return err
	}
	m.RerunCheckRuns = append(m.RerunCheckRuns, id)
	return nil
}

func (m *MockClient) ListPullRequestReviews(_ context.Context, _, _ string, number int) ([]Review, error) {
	if err := m.err("ListPullRequestReviews"); err != nil {
		return nil, err
	}
	return m.Reviews[number], nil
}

func (m *MockClient) CreatePullRequestReview(_ context.Context, _, _ string, number int, event, body string) error {
	if err := m.err("CreatePullRequestReview"); err != nil {
		return err
	}
	id := int64(len(m.Reviews[number]) + 1)
	review := Review{ID: id, State: event, Login: "bot"}
	m.Reviews[number] = append(m.Reviews[number], review)
	m.ReviewsPosted = append(m.ReviewsPosted, ReviewRecord{Number: number, Event: event, Body: body, ID: id})
	return nil
}

func (m *MockClient) DismissPullRequestReview(_ context.Context, _, _ string, _ int, reviewID int64, _ string) error {
	if err := m.err("DismissPullRequestReview"); err != nil {
		return err
	}
	for prNum := range m.Reviews {
		for i := range m.Reviews[prNum] {
			if m.Reviews[prNum][i].ID == reviewID {
				m.Reviews[prNum][i].State = "DISMISSED"
				break
			}
		}
	}
	m.ReviewsDismissed = append(m.ReviewsDismissed, reviewID)
	return nil
}
