package ghclient

import (
	"context"
	"fmt"

	gh "github.com/google/go-github/v72/github"
)

// MockClient is an in-process mock for tests. Zero value is usable.
type MockClient struct {
	// Pre-loaded read state.
	RepoLabels   map[string]*gh.Label     // label name -> label
	PullRequests map[int]*gh.PullRequest  // PR number -> PR
	PRFiles      map[int][]*gh.CommitFile // PR number -> files
	FileContent  map[string][]byte        // "path@ref" -> content
	OrgMembers   map[string]bool          // "org/user" -> is member
	WriteAccess  map[string]bool          // "owner/repo/user" -> has write

	// Mutable state modified by calls.
	IssueLabels map[int]map[string]bool // issue number -> set of label names

	// Call records for assertions.
	Reactions         []ReactionRecord
	Comments          []CommentRecord
	AutoMergeEnabled  []string // nodeIDs passed to EnableAutoMerge
	AutoMergeDisabled []string // nodeIDs passed to DisableAutoMerge

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

func NewMockClient() *MockClient {
	return &MockClient{
		RepoLabels:   make(map[string]*gh.Label),
		PullRequests: make(map[int]*gh.PullRequest),
		PRFiles:      make(map[int][]*gh.CommitFile),
		FileContent:  make(map[string][]byte),
		OrgMembers:   make(map[string]bool),
		WriteAccess:  make(map[string]bool),
		IssueLabels:  make(map[int]map[string]bool),
		Errors:       make(map[string]error),
	}
}

func (m *MockClient) err(method string) error {
	return m.Errors[method]
}

func (m *MockClient) ListRepoLabels(_ context.Context, _, _ string) ([]*gh.Label, error) {
	if err := m.err("ListRepoLabels"); err != nil {
		return nil, err
	}
	labels := make([]*gh.Label, 0, len(m.RepoLabels))
	for _, l := range m.RepoLabels {
		labels = append(labels, l)
	}
	return labels, nil
}

func (m *MockClient) CreateLabel(_ context.Context, _, _ string, label *gh.Label) error {
	if err := m.err("CreateLabel"); err != nil {
		return err
	}
	m.RepoLabels[label.GetName()] = label
	return nil
}

func (m *MockClient) UpdateLabel(_ context.Context, _, _, name string, label *gh.Label) error {
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

func (m *MockClient) GetPullRequest(_ context.Context, _, _ string, number int) (*gh.PullRequest, error) {
	if err := m.err("GetPullRequest"); err != nil {
		return nil, err
	}
	pr, ok := m.PullRequests[number]
	if !ok {
		return nil, fmt.Errorf("PR %d not found", number)
	}
	return pr, nil
}

func (m *MockClient) ListPullRequestFiles(_ context.Context, _, _ string, number int) ([]*gh.CommitFile, error) {
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
