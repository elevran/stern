package github

import (
	"context"
	"fmt"
)

// MockClient is an in-process mock for tests. Zero value is usable.
type MockClient struct {
	// Pre-loaded read state.
	RepoLabels   map[string]Label      // label name -> label
	PullRequests map[int]*PullRequest  // PR number -> PR
	PRFiles      map[int][]string      // PR number -> filenames
	FileContent  map[string][]byte     // "path@ref" -> content
	OrgMembers   map[string]bool       // "org/user" -> is member
	WriteAccess  map[string]bool       // "owner/repo/user" -> has write
	Milestones   map[int]Milestone     // milestone number -> Milestone
	CheckRuns    map[string][]CheckRun // "owner/repo/sha" -> all check runs (caller filters)
	Items        []Item                // open issues + pull requests returned by ListOpenItems

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
	MilestoneCleared   []int // issue numbers passed to ClearMilestone
	AssigneesAdded     []UsersRecord
	AssigneesRemoved   []UsersRecord
	ReviewersRequested []UsersRecord
	ReviewersRemoved   []UsersRecord
	RerunCheckRuns     []int64 // check run IDs passed to RerunCheckRun

	// EnableAutoMergeCallCount is the number of EnableAutoMerge invocations.
	EnableAutoMergeCallCount int
	// DisableAutoMergeCallCount is the number of DisableAutoMerge invocations.
	DisableAutoMergeCallCount int

	// CreatedPRs records PRs returned by CreatePullRequest, keyed by the
	// auto-generated PR number.
	CreatedPRs map[int]PullRequest
	// CreatedPRMeta records the title/head/base/body arguments of every
	// CreatePullRequest call so tests can assert on the PR metadata.
	CreatedPRMeta []CreatedPRMeta

	// Return errors for specific method names.
	Errors map[string]error
}

type CreatedPRMeta struct {
	Number int
	Title  string
	Head   string
	Base   string
	Body   string
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

func NewMockClient() *MockClient {
	return &MockClient{
		RepoLabels:     make(map[string]Label),
		PullRequests:   make(map[int]*PullRequest),
		PRFiles:        make(map[int][]string),
		FileContent:    make(map[string][]byte),
		OrgMembers:     make(map[string]bool),
		WriteAccess:    make(map[string]bool),
		Milestones:     make(map[int]Milestone),
		CheckRuns:      make(map[string][]CheckRun),
		IssueLabels:    make(map[int]map[string]bool),
		IssueMilestone: make(map[int]int),
		Assignees:      make(map[int][]string),
		ReviewRequests: make(map[int][]string),
		CreatedPRs:     make(map[int]PullRequest),
		Errors:         make(map[string]error),
	}
}

func (m *MockClient) err(method string) error {
	return m.Errors[method]
}

// recordUsers adds users to the per-number slice in m, deduplicating
// against the existing entries. It is shared by AddAssignees and
// RequestReviewers to avoid duplicating the merge-set logic.
func recordUsers(m map[int][]string, number int, users []string) {
	existing := make(map[string]bool, len(m[number])+len(users))
	for _, u := range m[number] {
		existing[u] = true
	}
	for _, u := range users {
		existing[u] = true
	}
	merged := make([]string, 0, len(existing))
	for u := range existing {
		merged = append(merged, u)
	}
	m[number] = merged
}

// unrecordUsers removes users from the per-number slice in m, preserving
// order. It is shared by RemoveAssignees and RemoveReviewers.
func unrecordUsers(m map[int][]string, number int, users []string) {
	toRemove := make(map[string]bool, len(users))
	for _, u := range users {
		toRemove[u] = true
	}
	remaining := make([]string, 0, len(m[number]))
	for _, u := range m[number] {
		if !toRemove[u] {
			remaining = append(remaining, u)
		}
	}
	m[number] = remaining
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
	m.EnableAutoMergeCallCount++
	if err := m.err("EnableAutoMerge"); err != nil {
		return err
	}
	m.AutoMergeEnabled = append(m.AutoMergeEnabled, nodeID)
	return nil
}

func (m *MockClient) DisableAutoMerge(_ context.Context, nodeID string) error {
	m.DisableAutoMergeCallCount++
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
	recordUsers(m.Assignees, number, users)
	m.AssigneesAdded = append(m.AssigneesAdded, UsersRecord{Number: number, Users: users})
	return nil
}

func (m *MockClient) RemoveAssignees(_ context.Context, _, _ string, number int, users []string) error {
	if err := m.err("RemoveAssignees"); err != nil {
		return err
	}
	unrecordUsers(m.Assignees, number, users)
	m.AssigneesRemoved = append(m.AssigneesRemoved, UsersRecord{Number: number, Users: users})
	return nil
}

func (m *MockClient) RequestReviewers(_ context.Context, _, _ string, number int, users []string) error {
	if err := m.err("RequestReviewers"); err != nil {
		return err
	}
	recordUsers(m.ReviewRequests, number, users)
	m.ReviewersRequested = append(m.ReviewersRequested, UsersRecord{Number: number, Users: users})
	return nil
}

func (m *MockClient) RemoveReviewers(_ context.Context, _, _ string, number int, users []string) error {
	if err := m.err("RemoveReviewers"); err != nil {
		return err
	}
	unrecordUsers(m.ReviewRequests, number, users)
	m.ReviewersRemoved = append(m.ReviewersRemoved, UsersRecord{Number: number, Users: users})
	return nil
}

func (m *MockClient) ListCheckRuns(_ context.Context, owner, repo, sha string) ([]CheckRun, error) {
	if err := m.err("ListCheckRuns"); err != nil {
		return nil, err
	}
	return m.CheckRuns[owner+"/"+repo+"/"+sha], nil
}

func (m *MockClient) RerunCheckRun(_ context.Context, _, _ string, id int64) error {
	if err := m.err("RerunCheckRun"); err != nil {
		return err
	}
	m.RerunCheckRuns = append(m.RerunCheckRuns, id)
	return nil
}

func (m *MockClient) ListOpenItems(_ context.Context, _, _ string) ([]Item, error) {
	if err := m.err("ListOpenItems"); err != nil {
		return nil, err
	}
	return m.Items, nil
}

// CreatePullRequest records a fake PR creation. The returned PR number is
// one greater than the highest existing PR number in m.PullRequests (or
// the next free number in m.CreatedPRs) so tests can assert on it.
func (m *MockClient) CreatePullRequest(_ context.Context, _, _, title, head, base, body string) (int, error) {
	if err := m.err("CreatePullRequest"); err != nil {
		return 0, err
	}
	next := len(m.CreatedPRs) + 100 // 100+ to avoid collision with hand-set PR numbers like 1
	m.CreatedPRs[next] = PullRequest{
		Number: next,
		Title:  title,
		HeadSHA: "fake-head-sha",
		Labels:  []string{},
	}
	m.CreatedPRMeta = append(m.CreatedPRMeta, CreatedPRMeta{
		Number: next,
		Title:  title,
		Head:   head,
		Base:   base,
		Body:   body,
	})
	return next, nil
}
