package github

import (
	"context"
	"fmt"
	"net/http"

	gh "github.com/google/go-github/v72/github"
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

// CreatedPRMeta captures the arguments of a CreatePullRequest call so tests
// can assert on the title/head/base/body independently of the returned PR.
type CreatedPRMeta struct {
	Number int
	Title  string
	Head   string
	Base   string
	Body   string
}

// ReactionRecord captures a single reaction added by CreateCommentReaction.
type ReactionRecord struct {
	CommentID int64
	Content   string
}

// CommentRecord captures a single issue/PR comment posted via CreateIssueComment.
type CommentRecord struct {
	Number int
	Body   string
}

// MilestoneSetRecord captures a single SetMilestone invocation.
type MilestoneSetRecord struct {
	Number      int
	MilestoneID int
}

// UsersRecord captures the (number, users) pair for assignees and reviewer
// add/remove operations.
type UsersRecord struct {
	Number int
	Users  []string
}

// NewMockClient returns a MockClient with all internal maps pre-initialised,
// so tests can assign into them without nil-map checks.
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

// ListRepoLabels returns the snapshot of repo labels pre-loaded into m.RepoLabels.
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

// CreateLabel stores label under m.RepoLabels[label.Name].
func (m *MockClient) CreateLabel(_ context.Context, _, _ string, label Label) error {
	if err := m.err("CreateLabel"); err != nil {
		return err
	}
	m.RepoLabels[label.Name] = label
	return nil
}

// UpdateLabel replaces the entry under m.RepoLabels[name].
func (m *MockClient) UpdateLabel(_ context.Context, _, _, name string, label Label) error {
	if err := m.err("UpdateLabel"); err != nil {
		return err
	}
	m.RepoLabels[name] = label
	return nil
}

// DeleteLabel removes m.RepoLabels[name].
func (m *MockClient) DeleteLabel(_ context.Context, _, _, name string) error {
	if err := m.err("DeleteLabel"); err != nil {
		return err
	}
	delete(m.RepoLabels, name)
	return nil
}

// AddLabels unions labels into the set tracked at m.IssueLabels[number].
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

// RemoveLabel drops label from the set tracked at m.IssueLabels[number].
func (m *MockClient) RemoveLabel(_ context.Context, _, _ string, number int, label string) error {
	if err := m.err("RemoveLabel"); err != nil {
		return err
	}
	if m.IssueLabels[number] != nil {
		delete(m.IssueLabels[number], label)
	}
	return nil
}

// GetPullRequest returns the pre-loaded PR or a "not found" error if absent.
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

// ListPullRequestFiles returns the pre-loaded file list for number (may be nil).
func (m *MockClient) ListPullRequestFiles(_ context.Context, _, _ string, number int) ([]string, error) {
	if err := m.err("ListPullRequestFiles"); err != nil {
		return nil, err
	}
	return m.PRFiles[number], nil
}

// CreateCommentReaction appends to m.Reactions.
func (m *MockClient) CreateCommentReaction(_ context.Context, _, _ string, commentID int64, content string) error {
	if err := m.err("CreateCommentReaction"); err != nil {
		return err
	}
	m.Reactions = append(m.Reactions, ReactionRecord{CommentID: commentID, Content: content})
	return nil
}

// CreateIssueComment appends to m.Comments.
func (m *MockClient) CreateIssueComment(_ context.Context, _, _ string, number int, body string) error {
	if err := m.err("CreateIssueComment"); err != nil {
		return err
	}
	m.Comments = append(m.Comments, CommentRecord{Number: number, Body: body})
	return nil
}

// IsOrgMember returns m.OrgMembers[org+"/"+user].
func (m *MockClient) IsOrgMember(_ context.Context, org, user string) (bool, error) {
	if err := m.err("IsOrgMember"); err != nil {
		return false, err
	}
	return m.OrgMembers[org+"/"+user], nil
}

// HasWriteAccess returns m.WriteAccess[owner+"/"+repo+"/"+user].
func (m *MockClient) HasWriteAccess(_ context.Context, owner, repo, user string) (bool, error) {
	if err := m.err("HasWriteAccess"); err != nil {
		return false, err
	}
	return m.WriteAccess[owner+"/"+repo+"/"+user], nil
}

// EnableAutoMerge records nodeID in m.AutoMergeEnabled and increments the call count.
func (m *MockClient) EnableAutoMerge(_ context.Context, nodeID string, _ string) error {
	m.EnableAutoMergeCallCount++
	if err := m.err("EnableAutoMerge"); err != nil {
		return err
	}
	m.AutoMergeEnabled = append(m.AutoMergeEnabled, nodeID)
	return nil
}

// DisableAutoMerge records nodeID in m.AutoMergeDisabled and increments the call count.
func (m *MockClient) DisableAutoMerge(_ context.Context, nodeID string) error {
	m.DisableAutoMergeCallCount++
	if err := m.err("DisableAutoMerge"); err != nil {
		return err
	}
	m.AutoMergeDisabled = append(m.AutoMergeDisabled, nodeID)
	return nil
}

// GetFileContent returns m.FileContent[path+"@"+ref] or a 404 ErrorResponse
// when the key is absent, matching the real client's behaviour.
func (m *MockClient) GetFileContent(_ context.Context, _, _, path, ref string) ([]byte, error) {
	if err := m.err("GetFileContent"); err != nil {
		return nil, err
	}
	key := path + "@" + ref
	content, ok := m.FileContent[key]
	if !ok {
		// Match the real client's behaviour: missing files surface as a
		// 404, which IsNotFoundError recognises. Tests that exercise
		// production fail-closed semantics (e.g. owners.LoadForPaths)
		// rely on this distinction.
		return nil, &gh.ErrorResponse{
			Response: &http.Response{StatusCode: http.StatusNotFound},
			Message:  fmt.Sprintf("file %q not found at ref %q", path, ref),
		}
	}
	return content, nil
}

// CloseIssue appends number to m.IssueClosed.
func (m *MockClient) CloseIssue(_ context.Context, _, _ string, number int) error {
	if err := m.err("CloseIssue"); err != nil {
		return err
	}
	m.IssueClosed = append(m.IssueClosed, number)
	return nil
}

// ReopenIssue appends number to m.IssueReopened.
func (m *MockClient) ReopenIssue(_ context.Context, _, _ string, number int) error {
	if err := m.err("ReopenIssue"); err != nil {
		return err
	}
	m.IssueReopened = append(m.IssueReopened, number)
	return nil
}

// ListMilestones returns a snapshot of m.Milestones.
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

// SetMilestone records the assignment in m.IssueMilestone and appends to m.MilestoneSet.
func (m *MockClient) SetMilestone(_ context.Context, _, _ string, number int, milestoneID int) error {
	if err := m.err("SetMilestone"); err != nil {
		return err
	}
	m.IssueMilestone[number] = milestoneID
	m.MilestoneSet = append(m.MilestoneSet, MilestoneSetRecord{Number: number, MilestoneID: milestoneID})
	return nil
}

// ClearMilestone deletes m.IssueMilestone[number] and appends to m.MilestoneCleared.
func (m *MockClient) ClearMilestone(_ context.Context, _, _ string, number int) error {
	if err := m.err("ClearMilestone"); err != nil {
		return err
	}
	delete(m.IssueMilestone, number)
	m.MilestoneCleared = append(m.MilestoneCleared, number)
	return nil
}
// AddAssignees merges users into m.Assignees[number] and appends to m.AssigneesAdded.
func (m *MockClient) AddAssignees(_ context.Context, _, _ string, number int, users []string) error {
	if err := m.err("AddAssignees"); err != nil {
		return err
	}
	recordUsers(m.Assignees, number, users)
	m.AssigneesAdded = append(m.AssigneesAdded, UsersRecord{Number: number, Users: users})
	return nil
}

// RemoveAssignees removes users from m.Assignees[number] and appends to m.AssigneesRemoved.
func (m *MockClient) RemoveAssignees(_ context.Context, _, _ string, number int, users []string) error {
	if err := m.err("RemoveAssignees"); err != nil {
		return err
	}
	unrecordUsers(m.Assignees, number, users)
	m.AssigneesRemoved = append(m.AssigneesRemoved, UsersRecord{Number: number, Users: users})
	return nil
}

// RequestReviewers merges users into m.ReviewRequests[number] and appends to m.ReviewersRequested.
func (m *MockClient) RequestReviewers(_ context.Context, _, _ string, number int, users []string) error {
	if err := m.err("RequestReviewers"); err != nil {
		return err
	}
	recordUsers(m.ReviewRequests, number, users)
	m.ReviewersRequested = append(m.ReviewersRequested, UsersRecord{Number: number, Users: users})
	return nil
}

// RemoveReviewers removes users from m.ReviewRequests[number] and appends to m.ReviewersRemoved.
func (m *MockClient) RemoveReviewers(_ context.Context, _, _ string, number int, users []string) error {
	if err := m.err("RemoveReviewers"); err != nil {
		return err
	}
	unrecordUsers(m.ReviewRequests, number, users)
	m.ReviewersRemoved = append(m.ReviewersRemoved, UsersRecord{Number: number, Users: users})
	return nil
}

// ListCheckRuns returns m.CheckRuns[owner+"/"+repo+"/"+sha] (may be nil).
func (m *MockClient) ListCheckRuns(_ context.Context, owner, repo, sha string) ([]CheckRun, error) {
	if err := m.err("ListCheckRuns"); err != nil {
		return nil, err
	}
	return m.CheckRuns[owner+"/"+repo+"/"+sha], nil
}

// RerunCheckRun appends id to m.RerunCheckRuns.
func (m *MockClient) RerunCheckRun(_ context.Context, _, _ string, id int64) error {
	if err := m.err("RerunCheckRun"); err != nil {
		return err
	}
	m.RerunCheckRuns = append(m.RerunCheckRuns, id)
	return nil
}

// ListOpenItems returns m.Items.
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
		Number:  next,
		Title:   title,
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
