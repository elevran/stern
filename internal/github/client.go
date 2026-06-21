package github

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	gh "github.com/google/go-github/v72/github"
	"golang.org/x/oauth2"
)

// LabelsClient covers label CRUD and label assignment operations.
type LabelsClient interface {
	ListRepoLabels(ctx context.Context, owner, repo string) ([]Label, error)
	CreateLabel(ctx context.Context, owner, repo string, label Label) error
	UpdateLabel(ctx context.Context, owner, repo, name string, label Label) error
	DeleteLabel(ctx context.Context, owner, repo, name string) error
	AddLabels(ctx context.Context, owner, repo string, number int, labels []string) error
	RemoveLabel(ctx context.Context, owner, repo string, number int, label string) error
}

// PullRequestsClient covers pull request reads and auto-merge mutations.
type PullRequestsClient interface {
	GetPullRequest(ctx context.Context, owner, repo string, number int) (PullRequest, error)
	ListPullRequestFiles(ctx context.Context, owner, repo string, number int) ([]string, error)
	EnableAutoMerge(ctx context.Context, nodeID string, method string) error
	DisableAutoMerge(ctx context.Context, nodeID string) error
}

// CommentsClient covers comment and reaction creation.
type CommentsClient interface {
	CreateCommentReaction(ctx context.Context, owner, repo string, commentID int64, content string) error
	CreateIssueComment(ctx context.Context, owner, repo string, number int, body string) error
}

// PermissionsClient covers membership and access level checks.
type PermissionsClient interface {
	IsOrgMember(ctx context.Context, org, user string) (bool, error)
	HasWriteAccess(ctx context.Context, owner, repo, user string) (bool, error)
}

// ContentClient covers reading file contents from a repository.
type ContentClient interface {
	GetFileContent(ctx context.Context, owner, repo, path, ref string) ([]byte, error)
}

// IssueStateClient covers opening and closing issues and pull requests.
type IssueStateClient interface {
	CloseIssue(ctx context.Context, owner, repo string, number int) error
	ReopenIssue(ctx context.Context, owner, repo string, number int) error
}

// MilestoneClient covers milestone lookup and assignment on issues/PRs.
type MilestoneClient interface {
	ListMilestones(ctx context.Context, owner, repo string) ([]Milestone, error)
	SetMilestone(ctx context.Context, owner, repo string, number int, milestoneID int) error
	ClearMilestone(ctx context.Context, owner, repo string, number int) error
}

// UsersClient covers reviewer and assignee management.
type UsersClient interface {
	AddAssignees(ctx context.Context, owner, repo string, number int, users []string) error
	RemoveAssignees(ctx context.Context, owner, repo string, number int, users []string) error
	RequestReviewers(ctx context.Context, owner, repo string, number int, users []string) error
	RemoveReviewers(ctx context.Context, owner, repo string, number int, users []string) error
}

// ChecksClient covers check run listing and re-run operations.
type ChecksClient interface {
	ListCheckRuns(ctx context.Context, owner, repo, sha string) ([]CheckRun, error)
	RerunCheckRun(ctx context.Context, owner, repo string, id int64) error
}

// LifecycleClient covers operations needed by the scheduled lifecycle sweep.
// Mutations on items (label add/remove, comment, close) are covered by the
// existing LabelsClient / CommentsClient / IssueStateClient interfaces.
type LifecycleClient interface {
	// ListOpenItems returns all open issues and pull requests. Both kinds are
	// returned via the Issues API; Item.IsPR is set when the underlying
	// response contains a pull_request key.
	ListOpenItems(ctx context.Context, owner, repo string) ([]Item, error)
}

// Client is the full composed interface used by production code.
type Client interface {
	LabelsClient
	PullRequestsClient
	CommentsClient
	PermissionsClient
	ContentClient
	IssueStateClient
	MilestoneClient
	UsersClient
	ChecksClient
	LifecycleClient
}

type realClient struct {
	ghc *gh.Client
}

// NewFromEnv creates a Client authenticated via GITHUB_TOKEN.
func NewFromEnv() (Client, error) {
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		return nil, fmt.Errorf("GITHUB_TOKEN is not set")
	}
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
	tc := oauth2.NewClient(context.Background(), ts)
	return &realClient{ghc: gh.NewClient(tc)}, nil
}

func (c *realClient) ListRepoLabels(ctx context.Context, owner, repo string) ([]Label, error) {
	var all []Label
	opts := &gh.ListOptions{PerPage: 100}
	for {
		labels, resp, err := c.ghc.Issues.ListLabels(ctx, owner, repo, opts)
		if err != nil {
			return nil, err
		}
		for _, l := range labels {
			all = append(all, labelFromGH(l))
		}
		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}
	return all, nil
}

func (c *realClient) CreateLabel(ctx context.Context, owner, repo string, label Label) error {
	_, _, err := c.ghc.Issues.CreateLabel(ctx, owner, repo, labelToGH(label))
	return err
}

func (c *realClient) UpdateLabel(ctx context.Context, owner, repo, name string, label Label) error {
	_, _, err := c.ghc.Issues.EditLabel(ctx, owner, repo, name, labelToGH(label))
	return err
}

func (c *realClient) DeleteLabel(ctx context.Context, owner, repo, name string) error {
	_, err := c.ghc.Issues.DeleteLabel(ctx, owner, repo, name)
	return err
}

func (c *realClient) AddLabels(ctx context.Context, owner, repo string, number int, labels []string) error {
	_, _, err := c.ghc.Issues.AddLabelsToIssue(ctx, owner, repo, number, labels)
	return err
}

func (c *realClient) RemoveLabel(ctx context.Context, owner, repo string, number int, label string) error {
	_, err := c.ghc.Issues.RemoveLabelForIssue(ctx, owner, repo, number, label)
	return err
}

func (c *realClient) AddAssignees(ctx context.Context, owner, repo string, number int, users []string) error {
	_, _, err := c.ghc.Issues.AddAssignees(ctx, owner, repo, number, users)
	return err
}

func (c *realClient) RemoveAssignees(ctx context.Context, owner, repo string, number int, users []string) error {
	_, _, err := c.ghc.Issues.RemoveAssignees(ctx, owner, repo, number, users)
	return err
}

func (c *realClient) RequestReviewers(ctx context.Context, owner, repo string, number int, users []string) error {
	_, _, err := c.ghc.PullRequests.RequestReviewers(ctx, owner, repo, number, gh.ReviewersRequest{Reviewers: users})
	return err
}

func (c *realClient) RemoveReviewers(ctx context.Context, owner, repo string, number int, users []string) error {
	_, err := c.ghc.PullRequests.RemoveReviewers(ctx, owner, repo, number, gh.ReviewersRequest{Reviewers: users})
	return err
}

func (c *realClient) GetPullRequest(ctx context.Context, owner, repo string, number int) (PullRequest, error) {
	pr, _, err := c.ghc.PullRequests.Get(ctx, owner, repo, number)
	if err != nil {
		return PullRequest{}, err
	}
	return PullRequestFromGH(pr), nil
}

func (c *realClient) ListPullRequestFiles(ctx context.Context, owner, repo string, number int) ([]string, error) {
	var all []string
	opts := &gh.ListOptions{PerPage: 100}
	for {
		files, resp, err := c.ghc.PullRequests.ListFiles(ctx, owner, repo, number, opts)
		if err != nil {
			return nil, err
		}
		for _, f := range files {
			all = append(all, f.GetFilename())
		}
		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}
	return all, nil
}

func (c *realClient) CreateCommentReaction(ctx context.Context, owner, repo string, commentID int64, content string) error {
	_, _, err := c.ghc.Reactions.CreateIssueCommentReaction(ctx, owner, repo, commentID, content)
	return err
}

func (c *realClient) CreateIssueComment(ctx context.Context, owner, repo string, number int, body string) error {
	comment := &gh.IssueComment{Body: gh.Ptr(body)}
	_, _, err := c.ghc.Issues.CreateComment(ctx, owner, repo, number, comment)
	return err
}

func (c *realClient) IsOrgMember(ctx context.Context, org, user string) (bool, error) {
	isMember, _, err := c.ghc.Organizations.IsMember(ctx, org, user)
	return isMember, err
}

func (c *realClient) HasWriteAccess(ctx context.Context, owner, repo, user string) (bool, error) {
	perm, _, err := c.ghc.Repositories.GetPermissionLevel(ctx, owner, repo, user)
	if err != nil {
		return false, err
	}
	switch perm.GetPermission() {
	case "write", "maintain", "admin":
		return true, nil
	default:
		return false, nil
	}
}

// graphqlRequest is the JSON body for a GraphQL POST.
type graphqlRequest struct {
	Query     string         `json:"query"`
	Variables map[string]any `json:"variables,omitempty"`
}

// graphqlError mirrors a single entry in the top-level GraphQL "errors" array.
type graphqlError struct {
	Message string `json:"message"`
	Type    string `json:"type"` // e.g. "FORBIDDEN", "NOT_FOUND"
}

// graphqlResponse holds top-level GraphQL errors (mutations return null data on failure).
type graphqlResponse struct {
	Data   json.RawMessage `json:"data"`
	Errors []graphqlError  `json:"errors"`
}

// GraphQLError is the typed error returned from graphql() when the GraphQL
// response contains an error entry. Type is the GitHub error type
// (e.g. "FORBIDDEN", "NOT_FOUND"); callers can match on it with errors.As.
type GraphQLError struct {
	Message string
	Type    string
}

// Error implements the error interface. Message is intentionally used so the
// formatted string matches the previous "%s" behavior.
func (e *GraphQLError) Error() string {
	return e.Message
}

func (c *realClient) graphql(ctx context.Context, query string, vars map[string]any) error {
	req, err := c.ghc.NewRequest("POST", "graphql", graphqlRequest{Query: query, Variables: vars})
	if err != nil {
		return err
	}
	var resp graphqlResponse
	if _, err = c.ghc.Do(ctx, req, &resp); err != nil {
		return err
	}
	if len(resp.Errors) > 0 {
		first := resp.Errors[0]
		return &GraphQLError{Message: first.Message, Type: first.Type}
	}
	return nil
}

// EnableAutoMerge enables GitHub auto-merge via GraphQL. The mutation is
// idempotent — calling it when auto-merge is already enabled succeeds.
func (c *realClient) EnableAutoMerge(ctx context.Context, nodeID string, method string) error {
	if method == "" {
		method = "SQUASH"
	}
	return c.graphql(ctx,
		`mutation($id:ID!,$method:PullRequestMergeMethod!){
			enablePullRequestAutoMerge(input:{pullRequestId:$id,mergeMethod:$method}){
				pullRequest{id}
			}
		}`,
		map[string]any{"id": nodeID, "method": strings.ToUpper(method)},
	)
}

// DisableAutoMerge disables GitHub auto-merge via GraphQL. The mutation is
// idempotent — calling it when auto-merge is not enabled succeeds.
func (c *realClient) DisableAutoMerge(ctx context.Context, nodeID string) error {
	return c.graphql(ctx,
		`mutation($id:ID!){
			disablePullRequestAutoMerge(input:{pullRequestId:$id}){
				pullRequest{id}
			}
		}`,
		map[string]any{"id": nodeID},
	)
}

func (c *realClient) GetFileContent(ctx context.Context, owner, repo, path, ref string) ([]byte, error) {
	opts := &gh.RepositoryContentGetOptions{Ref: ref}
	fc, _, _, err := c.ghc.Repositories.GetContents(ctx, owner, repo, path, opts)
	if err != nil {
		return nil, err
	}
	if fc == nil {
		return nil, fmt.Errorf("path %q not found", path)
	}
	content, err := fc.GetContent()
	if err != nil {
		return nil, fmt.Errorf("decoding %s: %w", path, err)
	}
	return []byte(content), nil
}

func (c *realClient) CloseIssue(ctx context.Context, owner, repo string, number int) error {
	req := &gh.IssueRequest{State: gh.Ptr("closed")}
	_, _, err := c.ghc.Issues.Edit(ctx, owner, repo, number, req)
	return err
}

func (c *realClient) ReopenIssue(ctx context.Context, owner, repo string, number int) error {
	req := &gh.IssueRequest{State: gh.Ptr("open")}
	_, _, err := c.ghc.Issues.Edit(ctx, owner, repo, number, req)
	return err
}

func (c *realClient) ListMilestones(ctx context.Context, owner, repo string) ([]Milestone, error) {
	var all []Milestone
	opts := &gh.MilestoneListOptions{
		State:       "all",
		ListOptions: gh.ListOptions{PerPage: 100},
	}
	for {
		milestones, resp, err := c.ghc.Issues.ListMilestones(ctx, owner, repo, opts)
		if err != nil {
			return nil, err
		}
		for _, m := range milestones {
			all = append(all, Milestone{Number: m.GetNumber(), Title: m.GetTitle()})
		}
		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}
	return all, nil
}

func (c *realClient) SetMilestone(ctx context.Context, owner, repo string, number int, milestoneID int) error {
	_, _, err := c.ghc.Issues.Edit(ctx, owner, repo, number, &gh.IssueRequest{
		Milestone: &milestoneID,
	})
	return err
}

func (c *realClient) ClearMilestone(ctx context.Context, owner, repo string, number int) error {
	_, _, err := c.ghc.Issues.RemoveMilestone(ctx, owner, repo, number)
	return err
}

// ListCheckRuns lists all check runs on the given ref, paginating through
// every page. Callers apply their own conclusion filter (e.g. only failed).
func (c *realClient) ListCheckRuns(ctx context.Context, owner, repo, sha string) ([]CheckRun, error) {
	var runs []CheckRun
	opts := &gh.ListCheckRunsOptions{
		ListOptions: gh.ListOptions{PerPage: 100},
	}
	for {
		results, resp, err := c.ghc.Checks.ListCheckRunsForRef(ctx, owner, repo, sha, opts)
		if err != nil {
			return nil, err
		}
		for _, run := range results.CheckRuns {
			runs = append(runs, CheckRun{
				ID:         run.GetID(),
				Name:       run.GetName(),
				Conclusion: run.GetConclusion(),
			})
		}
		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}
	return runs, nil
}

// RerunCheckRun triggers GitHub to re-run a single check run.
func (c *realClient) RerunCheckRun(ctx context.Context, owner, repo string, id int64) error {
	_, err := c.ghc.Checks.ReRequestCheckRun(ctx, owner, repo, id)
	return err
}

// ListOpenItems returns all open issues and pull requests in the repository,
// paginating through every page. Both kinds surface via the Issues API.
func (c *realClient) ListOpenItems(ctx context.Context, owner, repo string) ([]Item, error) {
	var all []Item
	opts := &gh.IssueListByRepoOptions{
		State:       "open",
		ListOptions: gh.ListOptions{PerPage: 100},
	}
	for {
		items, resp, err := c.ghc.Issues.ListByRepo(ctx, owner, repo, opts)
		if err != nil {
			return nil, err
		}
		for _, it := range items {
			labels := make([]string, 0, len(it.Labels))
			for _, l := range it.Labels {
				labels = append(labels, l.GetName())
			}
			all = append(all, Item{
				Number:       it.GetNumber(),
				Labels:       labels,
				UpdatedAt:    it.GetUpdatedAt().Time,
				IsPR:         it.PullRequestLinks != nil,
				HasMilestone: it.Milestone != nil,
			})
		}
		if resp.NextPage == 0 {
			break
		}
		// IssueListByRepoOptions embeds both ListCursorOptions and ListOptions
		// (both define Page); disambiguate to the integer-typed ListOptions.Page.
		opts.ListOptions.Page = resp.NextPage
	}
	return all, nil
}
