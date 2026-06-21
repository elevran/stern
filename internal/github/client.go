package github

import (
	"context"
	"fmt"
	"os"
	"strings"

	gh "github.com/google/go-github/v72/github"
	"github.com/sirupsen/logrus"
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
	ListFailedCheckRuns(ctx context.Context, owner, repo, sha string) ([]CheckRun, error)
	RerunCheckRun(ctx context.Context, owner, repo string, id int64) error
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

// graphqlResponse holds top-level GraphQL errors (mutations return null data on failure).
type graphqlResponse struct {
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors"`
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
		return fmt.Errorf("graphql: %s", resp.Errors[0].Message)
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

// ListFailedCheckRuns lists check runs on the given ref whose conclusion
// indicates a failure: "failure", "timed_out", "cancelled", or
// "action_required". Skipped and successful runs are excluded.
func (c *realClient) ListFailedCheckRuns(ctx context.Context, owner, repo, sha string) ([]CheckRun, error) {
	var failed []CheckRun
	opts := &gh.ListCheckRunsOptions{
		ListOptions: gh.ListOptions{PerPage: 100},
	}
	for {
		results, resp, err := c.ghc.Checks.ListCheckRunsForRef(ctx, owner, repo, sha, opts)
		if err != nil {
			return nil, err
		}
		for _, run := range results.CheckRuns {
			switch run.GetConclusion() {
			case "failure", "timed_out", "cancelled", "action_required":
				failed = append(failed, CheckRun{
					ID:         run.GetID(),
					Name:       run.GetName(),
					Conclusion: run.GetConclusion(),
				})
			}
		}
		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}
	return failed, nil
}

// RerunCheckRun triggers GitHub to re-run a single check run.
func (c *realClient) RerunCheckRun(ctx context.Context, owner, repo string, id int64) error {
	_, err := c.ghc.Checks.ReRequestCheckRun(ctx, owner, repo, id)
	return err
}

// dryRunClient wraps a Client and logs mutating calls without executing them.
type dryRunClient struct {
	inner  Client
	logger *logrus.Logger
}

// NewDryRun wraps a Client so all mutating calls are logged instead of executed.
func NewDryRun(inner Client, logger *logrus.Logger) Client {
	return &dryRunClient{inner: inner, logger: logger}
}

// Read-through: these methods pass to inner unchanged.
func (c *dryRunClient) ListRepoLabels(ctx context.Context, owner, repo string) ([]Label, error) {
	return c.inner.ListRepoLabels(ctx, owner, repo)
}
func (c *dryRunClient) GetPullRequest(ctx context.Context, owner, repo string, number int) (PullRequest, error) {
	return c.inner.GetPullRequest(ctx, owner, repo, number)
}
func (c *dryRunClient) ListPullRequestFiles(ctx context.Context, owner, repo string, number int) ([]string, error) {
	return c.inner.ListPullRequestFiles(ctx, owner, repo, number)
}
func (c *dryRunClient) IsOrgMember(ctx context.Context, org, user string) (bool, error) {
	return c.inner.IsOrgMember(ctx, org, user)
}
func (c *dryRunClient) HasWriteAccess(ctx context.Context, owner, repo, user string) (bool, error) {
	return c.inner.HasWriteAccess(ctx, owner, repo, user)
}
func (c *dryRunClient) GetFileContent(ctx context.Context, owner, repo, path, ref string) ([]byte, error) {
	return c.inner.GetFileContent(ctx, owner, repo, path, ref)
}
func (c *dryRunClient) ListMilestones(ctx context.Context, owner, repo string) ([]Milestone, error) {
	return c.inner.ListMilestones(ctx, owner, repo)
}
func (c *dryRunClient) ListFailedCheckRuns(ctx context.Context, owner, repo, sha string) ([]CheckRun, error) {
	return c.inner.ListFailedCheckRuns(ctx, owner, repo, sha)
}

// Mutating methods: log and no-op.
func (c *dryRunClient) CreateLabel(ctx context.Context, owner, repo string, label Label) error {
	c.logger.WithFields(logrus.Fields{"owner": owner, "repo": repo, "name": label.Name}).Info("[dry-run] CreateLabel")
	return nil
}
func (c *dryRunClient) UpdateLabel(ctx context.Context, owner, repo, name string, label Label) error {
	c.logger.WithFields(logrus.Fields{"owner": owner, "repo": repo, "name": name}).Info("[dry-run] UpdateLabel")
	return nil
}
func (c *dryRunClient) DeleteLabel(ctx context.Context, owner, repo, name string) error {
	c.logger.WithFields(logrus.Fields{"owner": owner, "repo": repo, "name": name}).Info("[dry-run] DeleteLabel")
	return nil
}
func (c *dryRunClient) AddLabels(ctx context.Context, owner, repo string, number int, labels []string) error {
	c.logger.WithFields(logrus.Fields{"owner": owner, "repo": repo, "number": number, "labels": labels}).Info("[dry-run] AddLabels")
	return nil
}
func (c *dryRunClient) RemoveLabel(ctx context.Context, owner, repo string, number int, label string) error {
	c.logger.WithFields(logrus.Fields{"owner": owner, "repo": repo, "number": number, "label": label}).Info("[dry-run] RemoveLabel")
	return nil
}
func (c *dryRunClient) CreateCommentReaction(ctx context.Context, owner, repo string, commentID int64, content string) error {
	c.logger.WithFields(logrus.Fields{"owner": owner, "repo": repo, "commentID": commentID, "content": content}).Info("[dry-run] CreateCommentReaction")
	return nil
}
func (c *dryRunClient) CreateIssueComment(ctx context.Context, owner, repo string, number int, body string) error {
	c.logger.WithFields(logrus.Fields{"owner": owner, "repo": repo, "number": number}).Info("[dry-run] CreateIssueComment")
	return nil
}
func (c *dryRunClient) EnableAutoMerge(_ context.Context, nodeID string, method string) error {
	c.logger.WithFields(logrus.Fields{"nodeID": nodeID, "method": method}).Info("[dry-run] EnableAutoMerge")
	return nil
}
func (c *dryRunClient) DisableAutoMerge(_ context.Context, nodeID string) error {
	c.logger.WithFields(logrus.Fields{"nodeID": nodeID}).Info("[dry-run] DisableAutoMerge")
	return nil
}
func (c *dryRunClient) CloseIssue(_ context.Context, owner, repo string, number int) error {
	c.logger.WithFields(logrus.Fields{"owner": owner, "repo": repo, "number": number}).Info("[dry-run] CloseIssue")
	return nil
}
func (c *dryRunClient) ReopenIssue(_ context.Context, owner, repo string, number int) error {
	c.logger.WithFields(logrus.Fields{"owner": owner, "repo": repo, "number": number}).Info("[dry-run] ReopenIssue")
	return nil
}
func (c *dryRunClient) SetMilestone(_ context.Context, owner, repo string, number int, milestoneID int) error {
	c.logger.WithFields(logrus.Fields{"owner": owner, "repo": repo, "number": number, "milestoneID": milestoneID}).Info("[dry-run] SetMilestone")
	return nil
}
func (c *dryRunClient) ClearMilestone(_ context.Context, owner, repo string, number int) error {
	c.logger.WithFields(logrus.Fields{"owner": owner, "repo": repo, "number": number}).Info("[dry-run] ClearMilestone")
	return nil
}
func (c *dryRunClient) AddAssignees(_ context.Context, owner, repo string, number int, users []string) error {
	c.logger.WithFields(logrus.Fields{"owner": owner, "repo": repo, "number": number, "users": users}).Info("[dry-run] AddAssignees")
	return nil
}
func (c *dryRunClient) RemoveAssignees(_ context.Context, owner, repo string, number int, users []string) error {
	c.logger.WithFields(logrus.Fields{"owner": owner, "repo": repo, "number": number, "users": users}).Info("[dry-run] RemoveAssignees")
	return nil
}
func (c *dryRunClient) RequestReviewers(_ context.Context, owner, repo string, number int, users []string) error {
	c.logger.WithFields(logrus.Fields{"owner": owner, "repo": repo, "number": number, "users": users}).Info("[dry-run] RequestReviewers")
	return nil
}
func (c *dryRunClient) RemoveReviewers(_ context.Context, owner, repo string, number int, users []string) error {
	c.logger.WithFields(logrus.Fields{"owner": owner, "repo": repo, "number": number, "users": users}).Info("[dry-run] RemoveReviewers")
	return nil
}
func (c *dryRunClient) RerunCheckRun(_ context.Context, owner, repo string, id int64) error {
	c.logger.WithFields(logrus.Fields{"owner": owner, "repo": repo, "id": id}).Info("[dry-run] RerunCheckRun")
	return nil
}
