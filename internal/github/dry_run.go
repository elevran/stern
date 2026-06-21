package github

import (
	"context"

	"github.com/sirupsen/logrus"
)

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
func (c *dryRunClient) ListCheckRuns(ctx context.Context, owner, repo, sha string) ([]CheckRun, error) {
	return c.inner.ListCheckRuns(ctx, owner, repo, sha)
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