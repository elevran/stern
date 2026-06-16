# Dependency Spike Results

Spike run: 2026-06-16

## Question 1: `google/go-github`

**Result: ✓ Works cleanly.**

`github.com/google/go-github/v72` resolves, compiles, and the client constructs
correctly. Key API surface confirmed:

- `client.Issues.ListLabels(ctx, owner, repo, opts)` — list repo labels
- `client.Issues.CreateLabel / EditLabel / DeleteLabel` — label management
- `client.Issues.AddLabelsToIssue / RemoveLabelForIssue` — label on issue/PR
- `client.PullRequests.Get / List / ListFiles / ListCommits / Merge` — PR ops
- `client.Reactions.CreateCommentReaction` — reaction on comment (confirmed)
- `client.Orgs.IsMember` — org membership check
- `client.Repositories.GetCollaborator` — write access check
- `client.NewRequest(method, url, body) + Do(ctx, req, v)` — raw API calls

**Confirmed dependency for `go.mod`:** `github.com/google/go-github/v72`

## Question 2: GitHub Auto-Merge API

**Result: REST endpoint accessible via `go-github`'s `NewRequest`/`Do`.**

go-github v72 does not wrap the auto-merge endpoints as named methods, but it
does define the `PullRequestAutoMerge` struct (used in PR response bodies). The
REST endpoints are callable directly:

```go
// Enable
body := map[string]string{"merge_method": "squash"}
req, _ := client.NewRequest("PUT", "repos/"+owner+"/"+repo+"/pulls/"+strconv.Itoa(num)+"/auto_merge", body)
_, err := client.Do(ctx, req, nil)

// Disable
req, _ := client.NewRequest("DELETE", "repos/"+owner+"/"+repo+"/pulls/"+strconv.Itoa(num)+"/auto_merge", nil)
_, err := client.Do(ctx, req, nil)
```

`shurcooL/githubv4` also works and could be used for the GraphQL mutation
`enablePullRequestAutoMerge`, but it's an unnecessary extra dependency since
the REST approach works within go-github's existing `NewRequest`/`Do` pattern.

**Decision: use REST via `client.NewRequest`/`Do`. No GraphQL dependency needed.**

## Question 3: Prow Packages

**Result: ✗ Not viable — massive transitive dependency tree.**

`sigs.k8s.io/prow/pkg/repoowners` transitively imports:
- `k8s.io/client-go`, `k8s.io/api` (kubernetes client)
- `github.com/tektoncd/pipeline` (Tekton)
- `github.com/aws/aws-sdk-go-v2` (AWS SDK)
- `gocloud.dev` (Google Cloud abstraction layer)
- `google.golang.org/grpc` (gRPC)
- `github.com/andygrunwald/go-jira` (Jira client)
- Many more (>50 additional transitive deps)

This is the full Prow infrastructure dependency graph. Importing any of the
Kubernetes-adjacent Prow packages brings in the entire ecosystem.

`sigs.k8s.io/prow/pkg/github` (the client) is in the same module and has the
same dependency issue.

**Decision: do not use prow. All implementations are local.**

## Confirmed Dependency Decisions

| Concern | Decision |
|---|---|
| GitHub API client | `github.com/google/go-github/v72` |
| Auto-merge enable/disable | REST via `client.NewRequest`/`Do` |
| GraphQL | Not needed |
| OWNERS parsing | Local `internal/owners` package |
| Label constants | Local `internal/labels/labels.go` |
| Git operations (cherry-pick) | `exec.Command("git", ...)` |
| Prow packages | None |

## `go.mod` Direct Dependencies

```
github.com/google/go-github/v72
golang.org/x/oauth2
github.com/spf13/cobra
github.com/sirupsen/logrus
gopkg.in/yaml.v3
```
