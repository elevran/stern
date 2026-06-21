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

// Client abstracts GitHub API operations used by stern.
type Client interface {
	// Label management
	ListRepoLabels(ctx context.Context, owner, repo string) ([]Label, error)
	CreateLabel(ctx context.Context, owner, repo string, label Label) error
	UpdateLabel(ctx context.Context, owner, repo, name string, label Label) error
	DeleteLabel(ctx context.Context, owner, repo, name string) error
	AddLabels(ctx context.Context, owner, repo string, number int, labels []string) error
	RemoveLabel(ctx context.Context, owner, repo string, number int, label string) error

	// Pull requests
	GetPullRequest(ctx context.Context, owner, repo string, number int) (PullRequest, error)
	ListPullRequestFiles(ctx context.Context, owner, repo string, number int) ([]string, error)

	// Reactions and comments
	CreateCommentReaction(ctx context.Context, owner, repo string, commentID int64, content string) error
	CreateIssueComment(ctx context.Context, owner, repo string, number int, body string) error

	// Permissions
	IsOrgMember(ctx context.Context, org, user string) (bool, error)
	HasWriteAccess(ctx context.Context, owner, repo, user string) (bool, error)

	// Auto-merge — uses GraphQL; nodeID is the PR's global node_id.
	EnableAutoMerge(ctx context.Context, nodeID string, method string) error
	DisableAutoMerge(ctx context.Context, nodeID string) error

	// File content (for OWNERS parsing)
	GetFileContent(ctx context.Context, owner, repo, path, ref string) ([]byte, error)
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
