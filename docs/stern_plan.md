# stern: Implementation Plan

> A Go-based GitHub PR bot running entirely via GitHub Actions, reimplementing
> Kubernetes Prow's core plugin functionality. No persistent server. No external
> infrastructure. Fully self-hosted on your own repo.

---

## Name: `stern`

**Recommended name:** `stern`

**Why not the alternatives:**
- `prowlite` — derivative and self-deprecating; implies "worse prow"
- `proacton` — unclear meaning, hard to parse at a glance

**Why `stern`:**
- Nautical complement to "prow" — the prow is the front of a ship, the stern
  is the rear. Together they span the full lifecycle: prow handles incoming
  work, stern guides it out the back to merge. The metaphor is exact.
- Also an adjective: *stern* means strict, serious, and uncompromising — apt
  for a bot that enforces review requirements before allowing a merge.
- Short, lowercase, memorable, works naturally as a CLI command:
  `stern config init`, `stern slash-command`, `stern lifecycle`
- Not a name claimed by any major widely-used tool

**Other candidates considered (and why rejected):**

| Name | Reason rejected |
|---|---|
| `marshal` | Good meaning but a common English word with too many namespace collisions |
| `proctor` | Clever (PR + overseer) but sounds bureaucratic |
| `herald` | Nice but implies announcing, not enforcing |
| `rudder` | Nautical but `rudder` is already a well-known analytics platform |
| `tide` | Perfect meaning but already used by Prow itself for its merge queue |

---

## Table of Contents

1. [Architecture Overview](#architecture-overview)
2. [PR Data Access from the Default Branch](#pr-data-access-from-the-default-branch)
3. [Auto-Merge Design](#auto-merge-design)
4. [GitHub Client & Dependency Strategy](#github-client--dependency-strategy)
5. [File Naming Conventions](#file-naming-conventions)
6. [Per-Repo Configuration](#per-repo-configuration)
7. [GitHub Actions Execution Model](#github-actions-execution-model)
8. [Project Layout](#project-layout)
9. [Feature Specifications](#feature-specifications)
    - [Phase 0.0 — Dependency Spike](#phase-00--dependency-spike)
    - [Phase 0 — Foundation](#phase-0--foundation)
    - [Phase 1 — Core Review Flow](#phase-1--core-review-flow)
    - [Phase 2 — PR Management](#phase-2--pr-management)
    - [Phase 3 — Automation & Scheduling](#phase-3--automation--scheduling)
    - [Phase 4 — Advanced Git Operations](#phase-4--advanced-git-operations)
10. [Cross-Cutting Concerns](#cross-cutting-concerns)
11. [Delivery Sequence & Self-Hosting Strategy](#delivery-sequence--self-hosting-strategy)
12. [Summary Table](#summary-table)

---

## Architecture Overview

### Execution Model

Every bot action is triggered by a GitHub Actions workflow run on an ephemeral
runner. There is no daemon, no server, and no persistent state beyond GitHub
itself (labels, comments, PR metadata).

```
GitHub Event (issue_comment / pull_request_target / check_suite / schedule)
        │
        ▼
GitHub Actions runner spins up (ubuntu-latest, ~5–10s cold start)
        │
        ▼
actions/checkout@v4  ──▶  checks out DEFAULT BRANCH (main)
        │
        ▼
Go binary: stern <subcommand>
        │
        ├── Reads $GITHUB_EVENT_PATH  (event JSON written by GitHub)
        ├── Loads .github/stern.yaml  (feature config, always from main)
        ├── Routes to plugin handler
        └── Calls google/go-github Client
                │
                └── GitHub REST/GraphQL API
```

### The Five Trigger Types

| Trigger | GitHub Event | Stern Subcommand | Typical Latency | Required? |
|---|---|---|---|---|
| Slash command | `issue_comment: created` | `stern slash-command` | 5–15s | Always |
| PR lifecycle | `pull_request_target` | `stern pr-event` | 5–15s | Always |
| Issue lifecycle | `issues: opened` | `stern issue-event` | 5–15s | Always (low-priority placeholder) |
| CI completion | `check_suite: completed` | `stern merge-check` | 5–15s | Only if `merge.strategy: bot` |
| Scheduled sweep | `schedule` (cron) | `stern lifecycle` | runs daily | Only if `lifecycle` plugin enabled |

Two of the five triggers are conditional, not universal:

- The `check_suite` trigger and `merge-check` subcommand are only active when
  `merge.strategy: bot` in `stern.yaml`. With the recommended `native`
  strategy, GitHub's own auto-merge watches CI completion — this trigger and
  job are omitted entirely.

- **The `schedule` (cron) trigger exists for exactly one reason: the
  lifecycle staleness sweep (Phase 3.2).** Every other stern feature is
  reactive — it fires in response to a GitHub webhook (a comment posted, a
  commit pushed, a PR opened). There is no webhook for "nothing has happened
  to this issue in 90 days," because nothing happening doesn't generate an
  event. Cron is the only mechanism that can proactively ask "scan all open
  issues right now, regardless of recent activity." If the `lifecycle` plugin
  is not in your `plugins:` list, the `schedule` trigger and the
  `lifecycle-sweep` job can be deleted from `stern-triggers.yaml` entirely —
  nothing else depends on it. (The manual `/lifecycle stale|frozen|active`
  slash commands from Phase 3.1 are comment-triggered and need no cron of
  their own — only the *automated* sweep does.)

---

## PR Data Access from the Default Branch

This is the most commonly misunderstood aspect of the architecture and must be
understood clearly before implementing any feature.

### What the Bot Checks Out

When `issue_comment` or `pull_request_target` fires, `actions/checkout@v4`
checks out the **default branch (`main`)** — not the PR branch. This is
intentional: it means a malicious PR cannot modify the bot's workflow files or
its own source code to alter behavior.

```
Checked out on runner:   main branch
  ├── cmd/stern/main.go       ← the bot binary (always the main version)
  ├── .github/stern.yaml      ← feature config (always the main version)
  └── OWNERS                  ← root OWNERS file (main version)

NOT checked out:         PR branch
  └── (any files changed in the PR — not present on disk)
```

### What the Bot Needs About the PR

Despite running on main, `stern` needs full information about the PR to do its
work: who the author is, what files were changed, what labels are currently
applied, what the head SHA is, who has reviewed it, and what the OWNERS files
say for the changed paths.

**None of this requires checking out the PR branch.** All of it comes from
either the event JSON payload or additional GitHub API calls using the PR number.

### Where Each Piece of Data Comes From

| Data needed | Source | API call required? |
|---|---|---|
| PR number | `$GITHUB_EVENT_PATH` → `issue.number` | No |
| PR author login | `$GITHUB_EVENT_PATH` → `issue.user.login` | No |
| Current labels on the PR | `$GITHUB_EVENT_PATH` → `issue.labels` | No (but re-fetch to avoid staleness) |
| Comment author login | `$GITHUB_EVENT_PATH` → `comment.user.login` | No |
| Comment body | `$GITHUB_EVENT_PATH` → `comment.body` | No |
| PR head SHA | Not in `issue_comment` payload | Yes — `GetPullRequest(org, repo, num)` |
| PR base branch | Not in `issue_comment` payload | Yes — `GetPullRequest(org, repo, num)` |
| List of changed files | Not in `issue_comment` payload | Yes — `GetPullRequestChanges(org, repo, num)` |
| Requested reviewers | Not in `issue_comment` payload | Yes — `GetPullRequest()` |
| OWNERS file at `pkg/foo/OWNERS` on the PR branch | Not on disk | Yes — `GetFile(org, repo, path, headSHA)` via prow `repoowners` |
| Merge status / check results | Not in payload | Yes — `ListStatuses()` or branch protection API |

The practical implication: most handlers make **one or two additional API
calls** after reading the event JSON, then proceed with full information.
The prow `github.Client` wraps all of these calls with rate-limit handling
and typed return values.

### How OWNERS Files Are Read from the PR Branch

The `repoowners` package does not read OWNERS files from disk. It fetches them
from GitHub using `github.Client.GetFile(org, repo, path, ref)` where `ref` is
the PR's head SHA. This means:

1. The bot reads the event JSON to get the PR number
2. Calls `GetPullRequest()` to get the head SHA
3. Passes the head SHA to `repoowners.NewRepoOwners()` as the ref
4. `repoowners` fetches each OWNERS file from the PR branch via API as needed

This is correct behavior: it checks who owns the files *as they are in the PR*,
not who owned them on main before the PR was opened. For OWNERS file changes
in the PR itself, the new OWNERS content takes effect immediately.

### The `pull_request_target` Trigger Is Different

For the `pr-event` subcommand, the trigger is `pull_request_target`, not
`issue_comment`. The key difference: the event payload for
`pull_request_target` **does** contain the full PR object including head SHA,
base branch, changed file count, and draft status. Fewer additional API calls
are needed for PR-triggered handlers.

```
pull_request_target payload contains:
  pull_request.head.sha        ← available directly
  pull_request.base.ref        ← target branch name
  pull_request.additions       ← for /size
  pull_request.deletions       ← for /size
  pull_request.draft           ← for /wip
  pull_request.labels          ← current labels
  pull_request.user.login      ← PR author
```

### Security Boundary Summary

```
                    ┌─────────────────────────────────────┐
                    │          ALWAYS FROM MAIN            │
                    │  stern binary, stern.yaml config,    │
                    │  workflow YAML trigger definitions   │
                    └─────────────────────────────────────┘
                                      │
                         fetched via GitHub API
                                      │
                    ┌─────────────────────────────────────┐
                    │        FROM PR BRANCH (via API)      │
                    │  OWNERS files, changed file list,    │
                    │  head SHA, PR metadata               │
                    └─────────────────────────────────────┘
```

A PR author can change OWNERS files to grant themselves approval rights, but
this only takes effect for that PR — and is itself subject to review before
it can merge into main.

---

## Auto-Merge Design

Auto-merge requires two independent state machines (label state and CI state)
to simultaneously reach their ready conditions.

### The Core Problem

```
Label state:    lgtm ✓  +  approved ✓  +  hold absent ✓  +  wip absent ✓
CI state:       all required status checks passed ✓
```

These are satisfied by different events at different times. The bot must act
on both triggers: label changes and CI completion.

### Option A — GitHub Native Auto-Merge (Recommended)

When enabled on a PR, GitHub watches for required status checks to pass and
merges automatically — no polling, no race conditions, no `check_suite` handler.

**How stern uses it:** Every handler that modifies labels ends by calling
`merge.CheckEligibility()`. If the result is `Ready`, the handler calls
`EnableAutoMerge()`. If the result transitions to not-ready (hold added,
lgtm removed, new commit pushed), it calls `DisableAutoMerge()`.

```
Timeline A — /approve arrives after CI is already green:

 t=0   PR opened, CI starts
 t=1   CI finishes (all checks pass)     ← nothing happens, labels absent
 t=2   Reviewer posts /lgtm              → lgtm added, approve still missing
 t=3   Approver posts /approve           → approved added
       CheckEligibility() → Ready ✓
       stern calls EnableAutoMerge(squash)
       GitHub sees checks already green  → merges within seconds

Timeline B — CI finishes after /approve:

 t=0   PR opened
 t=1   Reviewer posts /lgtm             → lgtm added
 t=2   Approver posts /approve          → approved added
       CheckEligibility() → Ready ✓
       stern calls EnableAutoMerge(squash)   ← queued, waiting for CI
 t=3   CI finishes                      → GitHub auto-merges, no stern needed
```

**GitHub API:** Auto-merge is controlled via GraphQL mutations
`enablePullRequestAutoMerge` / `disablePullRequestAutoMerge`, which require
the PR's GraphQL node ID. The node ID is not in the `issue_comment` event
payload — fetch it once via `GetPullRequest()` alongside the head SHA.

**Hard requirements:**
- Branch protection must define at least one required status check. Without it,
  GitHub auto-merge triggers immediately on enablement (before CI runs).
- "Allow auto-merge" must be enabled in repo Settings → General.
- `GITHUB_TOKEN` needs `pull-requests: write` (already required for labels).

### Option B — Bot-Driven Merge (Fallback)

For environments where native auto-merge is unavailable.

1. `check_suite: completed` fires when a CI suite finishes.
2. `stern merge-check` finds open PRs whose head SHA matches the suite.
3. For each: calls `CheckEligibility()`. If ready, calls `Merge()`.
4. `/approve` and `/lgtm` also call `MergeIfReady()` inline in case CI was
   already green when the label arrived.

**Race condition:** Between eligibility check and merge call, a new commit
could arrive or a label change. GitHub's merge API validates branch protection
server-side and returns `405` if not truly ready — treat `405` and `409` as
debug-level log lines, not user-facing errors.

### Shared: Merge-Eligibility Check

Single source of truth for "is this PR ready to merge?" — shared by all
label-modifying handlers:

```go
// internal/merge/eligibility.go

type EligibilityResult struct {
    Ready          bool
    MissingLabels  []string   // e.g. ["lgtm"]
    BlockingLabels []string   // e.g. ["do-not-merge/hold"]
}

func CheckEligibility(pr *github.PullRequest, cfg *config.Options) EligibilityResult
```

---

## GitHub Client & Dependency Strategy

### Phase 0.0 Spike Validates These Decisions

Dependency choices are validated by the Phase 0.0 spike before feature work
begins. The spike tests four things in order:

1. `github.com/google/go-github` — constructs a client, lists labels on the
   stern repo. Expected to work.
2. GitHub auto-merge — does a REST endpoint exist (`PUT /repos/.../pulls/{n}/auto_merge`)?
   If not, test `shurcooL/githubv4` for the GraphQL mutation
   `enablePullRequestAutoMerge`.
3. Prow packages (`sigs.k8s.io/prow/pkg/github`, `pkg/repoowners`, `pkg/labels`)
   — optional: do they import cleanly and compile? If yes, evaluate whether
   they offer enough over the local implementations to justify the dependency.

### Decision: `google/go-github` as GitHub Client

Stern defines a narrow `github.Client` interface in `internal/github/client.go`
containing only the methods stern actually calls. This interface is backed by
`google/go-github` in production and a hand-written mock in tests.

| Concern | `gh` CLI | `google/go-github` |
|---|---|---|
| Unit testing | Requires exec mocking | Interface mock or `go-github-mock` |
| Type safety | String parsing of stdout | Typed structs |
| Rate limiting | Manual | Built-in |
| IDE support | None | Full autocomplete |
| Dependency risk | External binary | Well-maintained standard library |

The `gh` CLI is not used anywhere in business logic.

A `DryRunClient` wraps the interface and logs all mutating calls without
executing them. Enabled via `STERN_DRY_RUN=true`.

### Decision: Shell `git` for Git Operations

`/cherry-pick` shells out to `exec.Command("git", "cherry-pick", "-x", sha)`.
The `git` binary is present on all `ubuntu-latest` runners. Testable via a
temp git repo with `exec.Command`. No additional dependency needed.

| Concern | Shell `git` | `go-git` | `prow/pkg/git/v2` |
|---|---|---|---|
| Availability on runners | Always present | Separate dep | Prow dep |
| Testing | Temp repo + exec | In-memory repos | `NewLocalClientFactory` |
| Cherry-pick | Full (`-x` flag) | Partial | Full via `Am` |

**Decision:** Shell `git`.

### Decision: Local OWNERS Parser

Stern implements a minimal OWNERS file parser in `internal/owners/`. It fetches
OWNERS files from the PR's head SHA via the GitHub API (using the stern
`github.Client` interface), traverses the directory hierarchy, and resolves
`OWNERS_ALIASES` from the single root-level aliases file.

**Scope:**

| Feature | Phase 1 | Deferred |
|---|---|---|
| `approvers:` / `reviewers:` lists (individual users) | ✓ | |
| Directory hierarchy + `no_parent_owners` | ✓ | |
| Root `OWNERS_ALIASES` (`map[string][]string`) | ✓ | |
| GitHub team references (`org/team-slug`) | | deferred |

**Prow `repoowners` as an alternative:** If the Phase 0.0 spike shows that
`sigs.k8s.io/prow/pkg/repoowners` imports cleanly and handles GitHub team
expansion well, it may replace the local parser. If not (or if the coupling
to prow's own `github.Client` interface is too tight), the local parser is
the path forward.

### Decision: Local Label Constants

Label strings are defined in `internal/labels/labels.go` as typed constants:

```go
const (
    LGTM             = "lgtm"
    Approved         = "approved"
    Hold             = "do-not-merge/hold"
    WIP              = "do-not-merge/work-in-progress"
    NeedsRebase      = "needs-rebase"
    LifecycleStale   = "lifecycle/stale"
    LifecycleRotten  = "lifecycle/rotten"
    LifecycleFrozen  = "lifecycle/frozen"
)
```

No prow dependency required for label names.

---

## File Naming Conventions

Stern uses two distinct YAML files with clearly separated responsibilities.
All config files use `.yaml` (not `.yml`) throughout.

### `.github/stern.yaml` — Feature Configuration

**Purpose:** Declares which plugins are enabled and how each behaves. This is
the file operators edit to configure stern for their repo. It is always read
from the default branch. It is validated by `stern config check`.

```
.github/
└── stern.yaml          ← feature config (operators edit this)
```

This file never contains GitHub Actions syntax. It is pure bot configuration.

### `.github/workflows/stern-triggers.yaml` — GitHub Actions Workflow

**Purpose:** Defines *when* GitHub Actions invokes the `stern` binary. It
contains workflow triggers (`on:`), job definitions, permissions, and the
single shell command that runs `stern`. Operators rarely edit this file after
initial setup — it is largely boilerplate.

```
.github/
├── stern.yaml
└── workflows/
    └── stern-triggers.yaml   ← Actions workflow (rarely edited)
```

This file contains no plugin configuration. If a new plugin is added to stern,
the triggers file does not change — only `stern.yaml` changes.

### Why Two Files

Combining both into one location creates confusion about what is configuration
(feature flags, thresholds, label names) versus infrastructure (how GitHub
invokes the binary). Separating them makes the operator's mental model clear:

- "Which features are on?" → edit `stern.yaml`
- "When does stern run?" → edit `stern-triggers.yaml` (almost never)
- "Why did stern do X?" → the answer is always in `stern.yaml`

### Additional Config-Adjacent File

```
.github/
├── stern.yaml              ← feature config
├── workflows/
│   └── stern-triggers.yaml ← actions workflow
└── OWNERS                  ← repo-root OWNERS file (used by repoowners, not stern-specific)
```

The `OWNERS` file is not part of stern's configuration — it is read by the
`repoowners` package and follows prow's standard format.

---

## Per-Repo Configuration

`.github/stern.yaml` is the single file operators interact with. It is always
read from the default branch. Every section is optional — absence disables
the plugin even if the binary supports it.

```yaml
# .github/stern.yaml

org: elevran
repo: stern

# Explicit allowlist of enabled plugins.
# Plugins not listed here are completely ignored.
plugins:
  - lgtm
  - approve
  - hold
  - assign
  - label
  - size
  - lifecycle
  - wip
  - cherry-pick
  - review_assignment

# --- Merge Configuration ---

merge:
  # "native": use GitHub's built-in auto-merge (recommended)
  # "bot":    use check_suite event-driven merge (fallback for GHES)
  strategy: native
  method: squash          # squash | merge | rebase
  # PRs with any of these labels will never have auto-merge enabled
  blocking_labels:
    - "do-not-merge/hold"
    - "do-not-merge/work-in-progress"
    - "needs-rebase"

# --- Plugin Configuration ---

lgtm:
  allow_self_lgtm: false
  invalidate_on_push: true

approve:
  require_associated_issue: false
  allow_self_approval: false
  # Label added when lgtm + approved are both present (empty = no extra label)
  merge_label: "ready-to-merge"
  invalidate_on_push: false

size:
  thresholds:
    xs: 10
    s:  30
    m:  100
    l:  500
    xl: 1000
    # xxl = anything above xl
  exclude_paths:
    - "vendor/**"
    - "**/*.pb.go"
    - "**/*_generated.go"
    - "go.sum"

lifecycle:
  stale_after_days:  90
  rotten_after_days: 30
  close_after_days:  30
  stale_comment: |
    This issue has been inactive for 90 days and is now marked stale.
    Use `/lifecycle frozen` to permanently exempt it.
  rotten_comment: |
    This issue is rotten and will close in 30 days without activity.
  close_comment: |
    Closing due to inactivity. Reopen with `/reopen` if still relevant.
  exempt_labels:
    - "lifecycle/frozen"
    - "priority/critical"
    - "keep-open"

review_assignment:
  max_reviewers: 2
  skip_if_assigned: true
  use_reviewers_section: true
  load_balancing:
    enabled: true
    lookback_weeks: 3
    open_pr_weight: 2.0
    exclude_users:
      - "dependabot[bot]"
      - "renovate[bot]"

cherry_pick:
  allowed_branch_pattern: "^release-.*"
  label: "cherry-pick"
  branch_prefix: "cherry-pick"

# Label name overrides — adapts to existing repo label conventions.
# stern uses these strings in all API calls.
labels:
  lgtm:             "lgtm"
  approved:         "approved"
  hold:             "do-not-merge/hold"
  wip:              "do-not-merge/work-in-progress"
  merge_ready:      "ready-to-merge"
  needs_triage:     "needs-triage"
  lifecycle_stale:  "lifecycle/stale"
  lifecycle_rotten: "lifecycle/rotten"
  lifecycle_frozen: "lifecycle/frozen"
  size_prefix:      "size/"

# Label definitions: ground truth used by `stern config sync-labels`.
# All labels that stern may ever create or reference must appear here.
label_definitions:
  - name: "lgtm"
    color: "0e8a16"
    description: "Indicates a PR is ready to merge from a review standpoint"
  - name: "approved"
    color: "0e8a16"
    description: "Indicates a PR has been approved by an OWNERS approver"
  - name: "do-not-merge/hold"
    color: "e11d48"
    description: "Prevents merge. Apply with /hold, remove with /hold cancel"
  - name: "do-not-merge/work-in-progress"
    color: "e11d48"
    description: "Prevents merge. Apply with /wip"
  - name: "ready-to-merge"
    color: "0e8a16"
    description: "Both lgtm and approved — will merge once CI passes"
  - name: "needs-triage"
    color: "ededed"
    description: "Awaiting triage"
  - name: "lifecycle/stale"
    color: "795548"
    description: "No activity for 90 days"
  - name: "lifecycle/rotten"
    color: "795548"
    description: "Stale for 30 more days — will close soon"
  - name: "lifecycle/frozen"
    color: "1d76db"
    description: "Exempt from auto-close due to staleness"
  - name: "size/XS"
    color: "009900"
    description: "Changes 0–9 lines"
  - name: "size/S"
    color: "77bb00"
    description: "Changes 10–29 lines"
  - name: "size/M"
    color: "eebb00"
    description: "Changes 30–99 lines"
  - name: "size/L"
    color: "ee9900"
    description: "Changes 100–499 lines"
  - name: "size/XL"
    color: "ee5500"
    description: "Changes 500–999 lines"
  - name: "size/XXL"
    color: "ee0000"
    description: "Changes 1000+ lines"

# Permission requirements are enforced by each handler, not by config.
# Handlers define their own minimum permission floor (org member vs write access).
# There is no operator-configurable access section.
```

---

## GitHub Actions Execution Model

### Workflow File: `.github/workflows/stern-triggers.yaml`

This file defines *when* stern runs. It is not feature configuration.
Operators edit `stern.yaml` for features; this file is largely static after
initial setup.

```yaml
# .github/workflows/stern-triggers.yaml
# Defines when GitHub Actions invokes stern. Edit stern.yaml for feature config.
name: stern

on:
  issue_comment:
    types: [created]
  pull_request_target:
    types: [opened, synchronize, edited, closed, reopened]
  issues:
    types: [opened]
  # OPTIONAL — only needed if stern.yaml sets merge.strategy: bot
  # Delete this trigger and the merge-check job below if using strategy: native
  check_suite:
    types: [completed]
  # OPTIONAL — only needed if the "lifecycle" plugin is enabled in stern.yaml
  # Every other stern feature reacts to a webhook (comment, push, PR open).
  # There is no webhook for "an issue has been quiet for 90 days" — cron is
  # the only way to proactively scan for inactivity. Delete this trigger and
  # the lifecycle-sweep job below if you are not using the lifecycle plugin.
  schedule:
    - cron: '0 2 * * *'

jobs:

  slash-command:
    # Fires on all issue/PR comments starting with /
    # Bot-identity check is performed in the binary — if the comment author
    # is the bot itself, stern exits 0 without processing.
    if: |
      github.event_name == 'issue_comment' &&
      startsWith(github.event.comment.body, '/')
    runs-on: ubuntu-latest
    # Serialize runs for the same issue/PR to prevent label-race conditions.
    # cancel-in-progress: false ensures every command is processed in order.
    concurrency:
      group: stern-comment-${{ github.event.issue.number }}
      cancel-in-progress: false
    permissions:
      pull-requests: write   # add/remove labels, comments, reviewers
      issues: write          # add/remove labels and comments on issues
      contents: write        # required for /cherry-pick (git push)
    steps:
      - uses: actions/checkout@v4   # checks out main branch
      - uses: actions/setup-go@v5
        with: { go-version: '1.23', cache: true }
      - run: go run ./cmd/stern slash-command
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}

  pr-event:
    # Fires on all PR state changes (push, title edit, open, close, reopen)
    if: github.event_name == 'pull_request_target'
    runs-on: ubuntu-latest
    permissions:
      pull-requests: write
      issues: write
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with: { go-version: '1.23', cache: true }
      - run: go run ./cmd/stern pr-event
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}

  issue-event:
    # Fires when a new issue is opened.
    # Low-priority placeholder — handles needs-triage labeling, first-time
    # contributor checks, and welcome messages as plugins are enabled.
    if: github.event_name == 'issues'
    runs-on: ubuntu-latest
    permissions:
      issues: write
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with: { go-version: '1.23', cache: true }
      - run: go run ./cmd/stern issue-event
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}

  merge-check:
    # OPTIONAL JOB — only needed when stern.yaml sets merge.strategy: bot
    # Delete this entire job (and the check_suite trigger above) when using
    # the recommended merge.strategy: native — GitHub's own auto-merge
    # handles CI-completion timing with no bot involvement.
    if: |
      github.event_name == 'check_suite' &&
      github.event.check_suite.conclusion == 'success'
    runs-on: ubuntu-latest
    permissions:
      pull-requests: write
      contents: write
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with: { go-version: '1.23', cache: true }
      - run: go run ./cmd/stern merge-check
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}

  lifecycle-sweep:
    # OPTIONAL JOB — only needed when the "lifecycle" plugin is enabled in
    # stern.yaml. Delete this entire job (and the schedule trigger above) if
    # you are not using automated staleness management. The manual
    # /lifecycle stale|frozen|active slash commands (Phase 3.1) do not
    # depend on this job — they run through the slash-command job above.
    if: github.event_name == 'schedule'
    runs-on: ubuntu-latest
    permissions:
      issues: write
      pull-requests: write
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with: { go-version: '1.23', cache: true }
      - run: go run ./cmd/stern lifecycle
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```

> **Trimming the workflow for your setup:** Most repos will only need three
> of the five jobs. If you adopt the recommended `merge.strategy: native` and
> do not enable the `lifecycle` plugin, delete the `check_suite` trigger, the
> `schedule` trigger, the `merge-check` job, and the `lifecycle-sweep` job —
> leaving only `slash-command` and `pr-event`. Add `schedule` back the moment
> you enable `lifecycle` in `stern.yaml`; add `check_suite` back only if you
> later need to fall back to `merge.strategy: bot`.

### Config-Check Workflow: `.github/workflows/stern-config-check.yaml`

A second workflow that validates `stern.yaml` on every PR that touches it:

```yaml
# .github/workflows/stern-config-check.yaml
# Validates stern.yaml on PRs that change it.
name: stern config check

on:
  pull_request:
    paths:
      - '.github/stern.yaml'

jobs:
  check:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with: { go-version: '1.23', cache: true }
      - run: go run ./cmd/stern config check --config .github/stern.yaml
```

Note this uses `pull_request` (not `pull_request_target`) because it is
read-only validation — no write permissions needed, and running on the PR
branch is correct here (we want to check the proposed config).

### PR-Event Handler Responsibilities

| Event Subtype | Handlers |
|---|---|
| `opened` | size labeling, wip detection, review_assignment auto-assign |
| `synchronize` | remove lgtm if `invalidate_on_push`, disable auto-merge, recompute size, re-evaluate wip |
| `edited` (title change) | re-evaluate wip label |
| `closed` (merged) | trigger pending cherry-picks |
| `reopened` | re-apply size label |

---

## Project Layout

```
stern/
├── cmd/
│   └── stern/
│       └── main.go              # Cobra root, subcommand wiring, client construction
│
├── internal/
│   ├── config/
│   │   ├── options.go           # Options struct, LoadFromFile(), applyDefaults(), validate()
│   │   ├── options_test.go
│   │   └── template.yaml.tmpl   # Embedded template for `stern config init`
│   │
│   ├── event/
│   │   ├── parse.go             # Unmarshal $GITHUB_EVENT_PATH → typed structs
│   │   ├── types.go             # CommentEvent, PREvent, IssueEvent, CheckSuiteEvent
│   │   └── parse_test.go
│   │
│   ├── github/
│   │   ├── client.go            # Client interface, NewFromEnv(), DryRunClient
│   │   └── mock.go              # In-process mock for tests
│   │
│   ├── labels/
│   │   └── labels.go            # Label name constants (LGTM, Approved, Hold, ...)
│   │
│   ├── owners/
│   │   ├── parse.go             # OWNERS + OWNERS_ALIASES parser (fetches via GitHub API)
│   │   └── parse_test.go
│   │
│   ├── merge/
│   │   ├── eligibility.go       # CheckEligibility() — shared by all label handlers
│   │   ├── eligibility_test.go
│   │   ├── automerge.go         # EnableAutoMerge / DisableAutoMerge
│   │   └── check_suite.go       # MergeIfReady — check_suite handler (Option B only)
│   │
│   ├── commands/
│   │   ├── registry.go          # map[string]Handler, dispatch loop, bot-identity check
│   │   ├── lgtm.go
│   │   ├── approve.go
│   │   ├── hold.go
│   │   ├── label.go             # /label, /kind, /area, /priority, /good-first-issue
│   │   ├── assign.go            # /assign, /unassign, /cc, /uncc
│   │   ├── milestone.go
│   │   ├── close.go
│   │   ├── retest.go
│   │   ├── lifecycle.go
│   │   ├── wip.go
│   │   └── cherrypick.go        # shells out to git cherry-pick; post-merge PRs only
│   │
│   ├── pr/
│   │   ├── events.go            # PR event dispatcher (size, lgtm-invalidate, wip-sync)
│   │   └── events_test.go
│   │
│   ├── scheduler/
│   │   ├── lifecycle.go         # Stale/rotten/close sweep
│   │   └── lifecycle_test.go
│   │
│   ├── review/
│   │   ├── select.go            # Load-balanced reviewer scoring algorithm
│   │   ├── select_test.go
│   │   └── owners.go            # Bridge: OWNERS candidates → scored list
│   │
│   └── permissions/
│       ├── checker.go           # IsOrgMember(), HasWriteAccess(), IsPRAuthor(), IsBot()
│       └── checker_test.go
│
├── .github/
│   ├── stern.yaml               # Feature config (operators edit this)
│   └── workflows/
│       ├── stern-triggers.yaml  # Actions workflow (rarely edited after setup)
│       └── stern-config-check.yaml  # Validates stern.yaml on PRs
│
├── go.mod                       # module github.com/elevran/stern
└── go.sum
```

### Handler Interface

The registry dispatches on the command verb (first word). Each handler
receives the full command line and parses its own subcommands/arguments.

```go
// internal/commands/registry.go

type Context struct {
    Event      *event.CommentEvent
    PR         *github.PullRequest  // nil for plain issue comments
    Config     *config.Options
    GitHub     github.Client
    BotLogin   string               // from GitHub API; used for bot-identity check
    Log        *logrus.Entry
}

// Handler handles all slash commands beginning with its verb.
// Commands() returns the verb (e.g. "/lgtm") — one entry per handler.
// Handle receives the full command line; it parses subcommands/args itself.
type Handler interface {
    Commands() []string   // e.g. []string{"/lgtm"}
    Handle(ctx Context, line string) error
}
```

Handlers own their permission checks. Each handler enforces its own minimum
permission floor; there is no central access-control configuration.

Handlers signal outcomes via GitHub reactions on the triggering comment:
- Success: `+1` reaction
- Permission denied / invalid input: `-1` reaction + explanatory comment
- Internal error: `confused` reaction + comment with Actions run link

---

## Feature Specifications

---

### Phase 0 — Foundation

> **Goal:** Bootstrappable skeleton that stern can use on its own repo.
> No plugins active yet. Includes all config tooling for new repo setup.

---

#### 0.0 — Dependency Spike

**What it does:**
Validates dependency choices before any feature code is written. A single
throwaway commit (not kept in the main branch) that answers four questions:

1. **`google/go-github`** — `go get`, construct a client from `$GITHUB_TOKEN`,
   call `ListLabels` on `elevran/stern`. Expected to work.
2. **Auto-merge API** — does `PUT /repos/{org}/{repo}/pulls/{n}/auto_merge`
   exist as a REST endpoint? If yes, no GraphQL needed. If no, test
   `shurcooL/githubv4` with the `enablePullRequestAutoMerge` mutation.
3. **Prow packages (optional)** — `go get sigs.k8s.io/prow/pkg/github`,
   `pkg/repoowners`, `pkg/labels`. Do they compile? If yes, does
   `repoowners` handle team expansion without requiring prow's full config
   infrastructure? This informs whether to use prow's OWNERS parser instead
   of the local implementation.

**Deliverable:** A short written note (committed to `docs/`) recording the
outcome of each question and the confirmed dependency decisions. The note
informs Phases 0.1 onward.

**Complexity:** 🟢 Easy | **Estimate:** 1–2h

---

#### 0.1 — Project Scaffold & CLI Entry Point

**What it does:**
Go module setup (`github.com/elevran/stern`), Cobra CLI (`slash-command`,
`pr-event`, `issue-event`, `merge-check`, `lifecycle`, `config`), event JSON
parsing from `$GITHUB_EVENT_PATH`, Options struct with `applyDefaults()` and
`validate()`, and `google/go-github` client construction from `$GITHUB_TOKEN`.

- Logging: `logrus`
- Dry-run mode: `STERN_DRY_RUN=true` wraps the GitHub client in
  `DryRunClient` — all mutating API calls logged but not executed
- `--config` flag overrides `.github/stern.yaml` default path
- On `issue_comment` events: read `comment.user.login`, compare to
  `BotName()` — exit 0 immediately if the bot posted the comment
- On `issue_comment` events: parse all lines starting with `/`; dispatch
  each by its verb to the registered handler
- On `issue_comment` events for PRs: call `GetPullRequest()` to hydrate
  head SHA, base branch, and label list; store as `Context.PR`

**Deliverable:** `stern slash-command` reads event, reacts with `+1` to
`/ping`, logs "no handler for /unknown" for unknown commands, exits 0.

**Complexity:** 🟢 Easy | **Estimate:** 2–3h

---

#### 0.2 — Permissions Checker

**What it does:**
Centralizes access control. Every handler calls one helper.

```go
type Checker interface {
    IsOrgMember(org, user string) (bool, error)
    HasWriteAccess(org, repo, user string) (bool, error)
    IsPRAuthor(pr *github.PullRequest, user string) bool
    CanRunCommand(cmd string, user string) (bool, error)
}
```

`CanRunCommand` consults `config.Access` to check if the user's permission
level is sufficient. All tests use `mock client` — no network.

**Complexity:** 🟢 Easy | **Estimate:** 1–2h

---

#### 0.3 — Actions Workflow Skeleton

**What it does:**
Creates `.github/workflows/stern-triggers.yaml` with the two **always-needed**
triggers and jobs: `issue_comment` → `slash-command` and `pull_request_target`
→ `pr-event`. Committed immediately — even before any handlers exist — so the
self-hosting loop starts as early as possible. With `plugins: []` in
`stern.yaml`, stern exits cleanly without taking any action.

The `check_suite` trigger/`merge-check` job and the `schedule` trigger/
`lifecycle-sweep` job are **not** included at this stage — both are
conditional on later features (`merge.strategy: bot` and the `lifecycle`
plugin respectively) and are added in Phase 1.5 and Phase 3.2 if and when
those features are actually enabled. Starting trimmed avoids a redundant
cron job running daily with nothing for it to do.

Also creates `.github/workflows/stern-config-check.yaml` as a separate
workflow (uses `pull_request`, not `pull_request_target` — it's read-only
validation on the PR branch's version of `stern.yaml`).

**Complexity:** 🟢 Easy | **Estimate:** 30min

---

#### 0.4 — `stern config init`

**CLI:** `stern config init [--output .github/stern.yaml] [--org ORG] [--repo REPO]`

**What it does:**
Generates a complete, well-commented `.github/stern.yaml`. Designed as the
first command a new user runs on a repo.

- Detects org/repo from `$GITHUB_REPOSITORY` env var if not provided via flags
- Template is embedded in the binary (`//go:embed`) as a YAML file with inline
  comments — preserves comments that programmatic marshaling would destroy
- All plugins are listed but disabled by default (`plugins: []`). Operators
  uncomment/add what they want.
- After writing the file, prints numbered next-step instructions

**Complexity:** 🟢 Easy | **Estimate:** 1–2h

**Initial prompt:**
```
Implement `stern config init` as a Cobra subcommand under `config`.
Write .github/stern.yaml by rendering a Go text/template embedded with //go:embed.
Template must include YAML comments explaining every field. All plugins disabled
by default. Auto-detect org/repo from $GITHUB_REPOSITORY if flags not provided.
After writing, print:
  ✓ Generated .github/stern.yaml
  Next steps:
    1. Edit plugins: list to enable features
    2. Run: stern config check
    3. Run: stern config sync-labels --dry-run
    4. Run: stern config sync-labels
    5. Commit .github/stern.yaml
Include a test that renders the template and parses it back to Config with zero errors.
```

---

#### 0.5 — `stern config check`

**CLI:** `stern config check [--config .github/stern.yaml]`

**What it does:**
Validates `stern.yaml` and reports **all** issues before exiting. Exits
non-zero if any ERROR. Suitable as a CI gate on PRs touching `stern.yaml`
(via `stern-config-check.yaml` workflow).

**Validation rules:**
- All names in `plugins:` are recognized (typo detection with "did you mean?"
  suggestion using edit distance)
- `merge.method` is `merge`, `squash`, or `rebase`
- `merge.strategy` is `native` or `bot`
- All label names referenced in plugin configs appear in `label_definitions`
- Lifecycle thresholds are positive integers
- `cherry_pick.allowed_branch_pattern` compiles as a valid Go regex
- ERROR if `cherry-pick` in plugins but `allowed_branch_pattern` is empty
  (dangerous: all branches would be valid targets)
- WARN if `merge.strategy: native` and `merge.blocking_labels` is empty
- WARN if `merge.strategy: native` and `blocking_labels` is empty (hold
  labels will not prevent auto-merge)

**Output format:**
```
.github/stern.yaml — 3 issues found

  ERROR  plugins[2]: unknown plugin "lgmt" (did you mean "lgtm"?)
  ERROR  cherry_pick.allowed_branch_pattern: empty — cherry-pick is enabled
         but all branches are allowed targets. Set "^release-.*" or similar.
  WARN   merge.blocking_labels is empty — hold labels will not block auto-merge

Run `stern config init` to regenerate a baseline config.
Exit code: 1
```

**Complexity:** 🟢 Easy | **Estimate:** 2–3h

**Initial prompt:**
```
Implement `stern config check` for stern. Read stern.yaml, run all validation
checks, collect ALL issues ([]ValidationIssue{Level, Field, Message, Suggestion})
before reporting — no early exit. Each check is a separate function returning
[]ValidationIssue. Print with color: red ERROR, yellow WARN. Exit 1 if any
ERROR, 0 if only WARNs or clean. Table-driven tests — one test case per rule.
```

---

#### 0.6 — `stern config sync-labels`

**CLI:** `stern config sync-labels [--dry-run] [--prune] [--yes] [--config .github/stern.yaml]`

**What it does:**
Reads `label_definitions:` from `stern.yaml`, fetches existing repo labels,
and reconciles:

- **CREATE:** In config, not in repo — always applied
- **UPDATE:** In both, but color or description differs — always applied
- **OK:** Exact match — no-op
- **EXTRA:** In repo, not in config — reported but not touched unless `--prune`

`--prune` also deletes EXTRA labels. Requires confirmation prompt unless `--yes`.

**Dry-run output:**
```
Label sync plan for elevran/stern (dry-run — no changes made):

  CREATE   lgtm                      #0e8a16  "Indicates a PR is ready to merge..."
  CREATE   approved                  #0e8a16  "Indicates a PR has been approved..."
  UPDATE   do-not-merge/hold         color: #cc0000 → #e11d48
  OK       size/XS                   (no changes)
  EXTRA    legacy-needs-review       (present in repo, not in config — use --prune to remove)

  2 to create, 1 to update, 6 unchanged, 1 extra
Run without --dry-run to apply.
```

**Note:** GitHub requires labels to exist before the bot can add them to a PR.
If `stern` tries to add a label that doesn't exist, the API returns a 422.
Running `stern config sync-labels` before enabling any plugin prevents this.
The bot will also post a clear error comment if it encounters a missing label,
listing the `sync-labels` command as the fix.

**Complexity:** 🟢 Easy | **Estimate:** 2–3h

**Initial prompt:**
```
Implement `stern config sync-labels`. Read label_definitions from stern.yaml.
Fetch all repo labels paginated via github.Client.GetRepoLabels(). Compute diff:
CREATE / UPDATE / OK / EXTRA. In --dry-run: print table and exit 0. Without
--dry-run: execute creates/updates, print results, exit 0. Never delete unless
--prune: prompt "Delete N extra labels? [y/N]" unless --yes is also passed.
Color output: green for CREATE/OK, yellow UPDATE, grey EXTRA.
Tests with mock client: bulk create, partial update, extra labels
reported not deleted, --prune + --yes deletes extras.
```

---

### Phase 1 — Core Review Flow

> **Goal:** Minimal feature set for real PR review on stern's own repo.
> Build 1.5 first — it is shared infrastructure for 1.1 through 1.4.

---

#### 1.5 — Merge Eligibility & Auto-Merge Module

**No slash command — shared infrastructure**

**What it does:**
Implements `internal/merge`:
- `CheckEligibility(pr, config)` — checks required labels present, blocking
  labels absent; returns `EligibilityResult`
- `EnableAutoMerge(ghc, pr, method)` — GraphQL `enablePullRequestAutoMerge`
  mutation. Requires PR node ID (fetched via `GetPullRequest()` if not already
  in context). If "already enabled" is returned, treats as success.
- `DisableAutoMerge(ghc, pr)` — GraphQL `disablePullRequestAutoMerge`. If
  "not enabled", treats as success.
- `MergeIfReady(ghc, pr, config)` — Option B only: calls `github.Client.Merge()`;
  handles 405 (branch protection failed) and 409 (conflict) as debug-level
  log lines, not errors

**Complexity:** 🟡 Medium | **Estimate:** 2–3h

**Initial prompt:**
```
Implement internal/merge for stern:
1. CheckEligibility(pr *github.PullRequest, cfg *config.Options) EligibilityResult
   {Ready bool, MissingLabels []string, BlockingLabels []string}
2. EnableAutoMerge(ghc github.Client, pr *github.PullRequest, method string) error
   Call enablePullRequestAutoMerge GraphQL mutation via ghc.GraphQL().
   "already enabled" response → return nil.
3. DisableAutoMerge(ghc github.Client, pr *github.PullRequest) error
   disablePullRequestAutoMerge mutation. "not enabled" → nil.
4. MergeIfReady(ghc github.Client, pr *github.PullRequest, cfg *config.Options) error
   CheckEligibility first. On ready: Merge(). On HTTP 405/409: log.Debug, return nil.

Unit tests for CheckEligibility — all combinations: ready, missing lgtm,
missing approved, hold present, wip present, multiple blockers. Table-driven.
```

---

#### 1.1 — `/lgtm` and LGTM Invalidation on Push

**Slash commands:** `/lgtm`, `/lgtm cancel`
**PR event trigger:** `synchronize`

**What it does:**
- `/lgtm`: Checks commenter is not PR author (per `allow_self_lgtm`). Checks
  OWNERS `reviewers:` via `internal/owners` if available, passing the PR's
  head SHA so the PR branch's OWNERS files are used. Falls back to any org
  member if no OWNERS. Adds `lgtm` label. Calls `CheckEligibility()` — if
  ready, calls `EnableAutoMerge()` (auto-merge state visible in merge button).
  Reacts `+1` on success.
- `/lgtm cancel`: Removes label, calls `DisableAutoMerge()`. Reacts `+1`.
- On `synchronize`: If `invalidate_on_push: true`, removes `lgtm`, calls
  `DisableAutoMerge()`. Silent — label removal is recorded in the PR timeline.

**OWNERS access:** `internal/owners` fetches OWNERS files from the PR's head
SHA via the GitHub API. OWNERS files are read from the PR branch, not main.

**What to skip initially:** Per-directory OWNERS scoping.

**Complexity:** 🟢 Easy | **Estimate:** 2–3h

**Initial prompt:**
```
Implement /lgtm handler for stern:
1. Parse "/lgtm" and "/lgtm cancel" from issue_comment events
2. /lgtm: check not-PR-author. Initialize repoowners.RepoOwner with PR head
   SHA (from Context.PR.Head.SHA) so OWNERS files are read from the PR branch.
   If OWNERS present, verify commenter in Reviewers(). Add lgtm label.
   Call merge.CheckEligibility() — if Ready: call merge.EnableAutoMerge() and
   post config.Merge.NotifyComment.
3. /lgtm cancel: remove label, call DisableAutoMerge, post confirmation.
4. PREventHandler for synchronize: if config.LGTM.InvalidateOnPush, remove lgtm
   label, call DisableAutoMerge, post ":recycle: LGTM invalidated — re-review needed."

Table-driven tests with mock client.
```

---

#### 1.2 — `/approve`

**Slash commands:** `/approve`, `/approve cancel`, `/approve no-issue`
**PR event trigger:** `synchronize` (if `invalidate_on_push: true`)

**What it does:**
- `/approve`: Fetches changed files via `GetPullRequestChanges()`. Calls
  `repoowners.LeafApprovers(path)` for each changed file using the PR's head
  SHA. Commenter must appear in at least one returned approvers set. Blocks
  self-approval if `allow_self_approval: false`. Adds `approved` label. Calls
  `CheckEligibility()` — if ready, enables auto-merge.
- `/approve cancel`: Removes `approved`, disables auto-merge.
- `/approve no-issue`: Marks this PR (via a label or comment marker) to
  suppress the linked-issue requirement.
- On `synchronize`: If `invalidate_on_push: true`, removes `approved`,
  disables auto-merge.

**OWNERS access:** `internal/owners` fetches files from the PR's head SHA, same as `/lgtm`.

**What to skip initially:** Exact-cover approver set suggestion; enforcing
`require_associated_issue`.

**Complexity:** 🟡 Medium | **Estimate:** 3–5h

---

#### 1.3 — `/hold`

**Slash commands:** `/hold`, `/hold cancel`

Adds/removes `do-not-merge/hold`. Any contributor may apply a hold; removal
requires the original holder or write access. Adding calls `DisableAutoMerge()`.
Removing calls `CheckEligibility()` and potentially re-enables auto-merge.

**Complexity:** 🟢 Easy | **Estimate:** 1–2h

---

#### 1.4 — `/wip` and Title-Based WIP Detection

**Slash commands:** `/wip`
**PR event trigger:** `opened`, `edited`, `synchronize`

Toggles `do-not-merge/work-in-progress`. Title-based detection on PR events.
Draft PR sync: Draft → add WIP label; un-drafted → remove and re-check
eligibility. All state transitions call `EnableAutoMerge` or
`DisableAutoMerge` accordingly.

**Complexity:** 🟢 Easy | **Estimate:** 1h

---

### Phase 2 — PR Management

> **Goal:** Day-to-day ergonomics. Each feature is an independent PR.

---

#### 2.1 — `/assign`, `/unassign`, `/cc`, `/uncc`

**Slash commands:** `/assign [@user...]`, `/unassign [@user...]`,
`/cc @user...`, `/uncc @user...`

`/assign` with no args assigns the commenter. `/cc` calls `RequestReview`
(reviewer list). `/assign` calls `AddAssignees` (assignee list — distinct
concepts in GitHub). Validates users exist. Shares `@mention` parsing.

**Complexity:** 🟢 Easy | **Estimate:** 1–2h

---

#### 2.2 — Balanced Load Assignment (Auto-Reviewer)

**Trigger:** `pull_request_target: opened` (automatic)
**Also used by:** `/assign` with no explicit usernames

**What it does:**
Automatically requests reviewers on PR open, selected from OWNERS
`reviewers:` for touched files, scored to avoid overloading active contributors.

**Scoring algorithm (`internal/assign/load.go`):**
1. Build candidate set from `repoowners.Reviewers(path)` for all changed files.
   OWNERS are fetched from the PR's head SHA.
2. For each candidate, compute a load score:
   - Fetch open PRs in repo (paginated): for each, scan `RequestedReviewers`.
     Add `open_pr_weight` (default 2.0) per open review assignment.
   - Fetch closed PRs from last `lookback_weeks` weeks: add 1.0 per assignment.
     (Indicates activity without current-overload penalty.)
3. Filter: remove PR author, `exclude_users`, anyone who already reviewed this PR.
4. Sort ascending by score; break ties alphabetically for determinism.
5. Return top `review_assignment.max_reviewers` candidates.

**What to skip:** Cross-repo load balancing; availability/OOO signals.

**Complexity:** 🟡 Medium | **Estimate:** 4–5h

**Signature:**
```go
// internal/review/select.go
func SelectReviewers(
    ctx context.Context,
    candidates []string,
    prAuthor string,
    prNumber int,
    cfg *config.Options,
    ghc github.Client,
    org, repo string,
) ([]string, error)
```

Unit tests with mock client: 5 open + 10 closed PRs, uneven distribution.
Verify least-loaded selected; author excluded; excluded_users skipped; ties
broken alphabetically.

---

#### 2.3 — `/label` Family

**Slash commands:** `/label <n>`, `/remove-label <n>`, `/kind <type>`,
`/area <area>`, `/priority <p>`, `/good-first-issue`, `/help-wanted`,
`/remove-good-first-issue`, `/remove-help-wanted`

Adds or removes labels. Semantic shortcuts prepend their prefix. Validates
label exists; creates with deterministic color (hash → pastel hex) if
`allow_label_creation: true`. Posts clear error if label missing and creation
disabled, with hint to run `stern config sync-labels`.

**Complexity:** 🟢 Easy | **Estimate:** 1h

---

#### 2.4 — `/milestone`

**Slash commands:** `/milestone <title>`, `/milestone clear`

Case-insensitive milestone lookup. Posts error comment with available
milestone titles if no match found.

**Complexity:** 🟢 Easy | **Estimate:** 1h

---

#### 2.5 — `/close` and `/reopen`

**Slash commands:** `/close`, `/close <reason>`, `/reopen`

Requires commenter is issue/PR author OR has write access. Posts reason as
comment before closing if provided. Handles both issues and PRs.

**Complexity:** 🟢 Easy | **Estimate:** 1h

---

#### 2.6 — `/size` Auto-Labeling

**Trigger:** `pull_request_target: opened, synchronize` (automatic)

Computes `additions + deletions`, subtracts lines matching `exclude_paths`
globs, maps to XS/S/M/L/XL/XXL bucket, removes previous `size/*` labels,
applies new one. Silent — no comment posted.

Note: `additions`/`deletions` are in the `pull_request_target` payload
directly. No extra API call needed for size computation.

**Complexity:** 🟢 Easy | **Estimate:** 1–2h

---

#### 2.7 — `/retest`

**Slash commands:** `/retest`, `/retest-required`

Finds failed workflow runs for the PR's head SHA via the Actions API and
re-runs them. `/retest-required` narrows to required status checks only.
Restricted to org members. Posts a comment listing which runs were restarted.

**What to skip:** Per-job-name targeting; Prow job config YAML.

**Complexity:** 🟡 Medium | **Estimate:** 3–4h

---

### Phase 3 — Automation & Scheduling

> **Goal:** Hands-off automation added after Phase 2 is stable.

---

#### 3.1 — `/lifecycle` Commands

**Slash commands:** `/lifecycle stale`, `/lifecycle rotten`, `/lifecycle frozen`,
`/lifecycle active`

Manual overrides. `/lifecycle active` removes all lifecycle labels.
`/lifecycle frozen` exempts the item from the sweep. Permission: any org member.

**Complexity:** 🟢 Easy | **Estimate:** 1h

---

#### 3.2 — Lifecycle Sweep (Scheduled)

**Trigger:** `schedule` cron, daily | **Subcommand:** `stern lifecycle`

This is the one feature in all of stern that requires the `schedule` trigger.
Everything built in Phases 0–2 reacts to webhooks; staleness detection cannot,
since "no activity occurred" never generates an event. This is the point in
the build where the `schedule:` block and `lifecycle-sweep` job get added (or
un-commented) in `stern-triggers.yaml` — they should not exist before this
feature is implemented and should be removed if this feature is later disabled.

Paginated sweep of all open issues and PRs:
1. Skip items with any `lifecycle.exempt_labels` label.
2. Determine lifecycle state from labels. Use `ListIssueEvents` to find
   when the current lifecycle label was applied — not `updated_at`, which
   would be reset by the bot's own stale comment.
3. Identify non-bot activity: comments by users other than the bot's own
   login (determined via `github.Client.BotName()`). This prevents stern
   from treating its own comments as "activity" that resets the stale clock.
4. State transitions per configured thresholds. Post configured comment text.
5. Rate limit: if `X-RateLimit-Remaining` < 10, sleep until reset.

**Complexity:** 🟡 Medium | **Estimate:** 4–6h

---

### Phase 4 — Advanced Git Operations

---

#### 4.1 — `/cherry-pick`

**Slash command:** `/cherry-pick <target-branch>`
**Trigger condition:** Commenter has write access; source PR already merged

`/cherry-pick` is **post-merge only**. Issuing it on an open PR returns a
`-1` reaction and a comment: "PR must be merged before cherry-picking."

1. Validates branch matches `allowed_branch_pattern` — error if not.
2. Verifies PR merged via `GetPullRequest()` — error if not merged.
3. Checks target branch exists via `git ls-remote` — error if not.
4. Clones/fetches repo, creates branch `{prefix}/{target}/{pr-num}` off target.
5. For each commit SHA from `ListPRCommits()`: `exec.Command("git", "cherry-pick", "-x", sha)`.
6. **Success:** `git push`, `CreatePullRequest` (title: `[cherry-pick {target}] {original}`; body references original PR), apply `cherry_pick.label`.
7. **Conflict:** `git cherry-pick --abort`, delete remote branch if pushed, post comment with failing commit SHA, conflicting files from stderr, and manual steps.

**Git operations:** Shell out to `git` binary via `exec.Command`.
The `git` binary is always present on `ubuntu-latest` runners. Tests use a
temp directory with `git init` — no network required.

**Requires:** `contents: write` permission.

**What to skip:** Automatic conflict resolution; batch cherry-picks; pre-merge queueing.

**Complexity:** 🔴 Difficult | **Estimate:** 5–8h

---

## Cross-Cutting Concerns

These apply to all phases and should be implemented in Phase 0 and extended
as each feature is added.

### Bot Identity

Stern needs to know its own GitHub login in several places:
- Lifecycle sweep: skip comments by the bot itself when checking for "activity"
- `/lgtm`: if the bot itself is somehow listed in OWNERS, it should not be
  able to self-approve
- Idempotent comments: find and update existing bot comments rather than
  posting duplicates

**Implementation:** Call `github.Client.BotName()` once at startup and store
in the handler `Context`. The prow client returns the login of the token owner.

### Handler Responses: Reactions, Not Comments

Handlers communicate outcomes via GitHub **reactions** on the triggering
comment, not by posting reply comments. The comment ID is in the
`issue_comment` event payload directly — no additional API call needed.

| Outcome | Response |
|---|---|
| Command succeeded | `+1` reaction on triggering comment |
| Permission denied / invalid input | `-1` reaction + comment explaining why |
| Internal error (API failure) | `confused` reaction + comment with Actions run link |

**Proactive state changes** (LGTM invalidated on push, size label updated,
WIP detected from title, auto-merge enabled/disabled) produce **no comment**.
GitHub's PR timeline records every label add/remove with the actor; the merge
button reflects auto-merge state. These signals are sufficient.

This eliminates the idempotent-comment pattern and the `ListIssueComments`
scan it required. Duplicate runs are instead handled by the workflow's
`concurrency:` group (serialize per issue/PR number, do not cancel).

### Error Handling and User Feedback

When a handler encounters an error that is the user's fault (wrong permissions,
unknown milestone, non-matching branch pattern), it should:
1. Post a brief, clear comment explaining what went wrong and how to fix it.
2. Exit with code 0 (not 1) — the job run was successful; the user's request
   was simply invalid.

When a handler encounters an internal error (API failure, rate limit), it
should:
1. Log the error at ERROR level.
2. Exit with code 1 — the job run failed and should be retried.
3. Optionally post a generic "stern encountered an error" comment with a link
   to the Actions run logs.

### Label Pre-Creation Requirement

GitHub returns a 422 if stern tries to add a label that does not exist in the
repo. This is surfaced as a clear error comment to the user:

```
:warning: stern: label "lgtm" does not exist in this repository.
Run `stern config sync-labels` to create all required labels, then retry.
```

This makes misconfiguration immediately visible rather than silently failing.

### Concurrency and Idempotency

Each Actions run is independent. Two `/lgtm` comments posted within seconds
of each other may result in two simultaneous bot runs. Labels are idempotent
(adding an existing label is a no-op). Comments use the idempotent pattern
above. `EnableAutoMerge` treats "already enabled" as success. These properties
make double-execution safe in all Phase 1–3 features. Cherry-pick (Phase 4) is
not idempotent — it creates a branch — so it checks `BranchExists()` first.

---

## Delivery Sequence & Self-Hosting Strategy

**Key principle:** stern manages stern's own PRs from Phase 1 onward.

```
Week 1 — Foundation
  0.0  Dependency spike (go-github, auto-merge REST/GraphQL, optional prow test)
  0.1  Scaffold + CLI (dry-run, all commands no-op, bot-identity check)
  0.2  Permissions checker (unit tests only)
  0.3  stern-triggers.yaml and stern-config-check.yaml live
       Initial stern-triggers.yaml has only issue_comment and
       pull_request_target — no check_suite, no schedule. Those two are
       added later, only if their corresponding features are enabled.
       (stern runs on every PR comment, exits cleanly — no handlers yet)
  0.4  stern config init   → generate .github/stern.yaml
  0.5  stern config check  → CI gate on stern.yaml changes active
  0.6  stern config sync-labels → run once to create all labels in repo

Week 1–2 — Core Review Flow
  1.5  Merge eligibility + auto-merge module (shared infra — build first)
  1.1  /lgtm + push invalidation + EnableAutoMerge trigger
  1.2  /approve + auto-merge when lgtm already present
  1.3  /hold (DisableAutoMerge on apply; re-check on cancel)
  1.4  /wip + title detection + draft sync

  ► stern.yaml: plugins: [lgtm, approve, hold, wip]
  ► merge.strategy: native, merge.method: squash
  ► IMPORTANT: ensure branch protection has ≥1 required status check before
    enabling native auto-merge — without it, PRs auto-merge before CI runs
  ► merge.strategy: native means the check_suite trigger is still not
    needed — stern self-hosts with only two workflow jobs through this phase
  ► stern self-hosts from this point forward

Week 2–3 — PR Management (each as a separate PR reviewed via stern)
  2.3  /label family      → categorize issues immediately
  2.6  /size              → every PR labelled automatically
  2.5  /close /reopen     → issue housekeeping
  2.4  /milestone         → milestone management
  2.1  /assign /cc        → formal reviewer requests
  2.2  Balanced auto-assign (review_assignment) → auto-reviewer on PR open

Week 3–4
  2.7  /retest → retrigger CI

Week 4–5 — Lifecycle
  3.1  /lifecycle commands → enable immediately (comment-triggered, no cron needed)
  3.2  Lifecycle sweep → THIS is where the schedule trigger and
       lifecycle-sweep job get added to stern-triggers.yaml for the first
       time. Nothing before this point requires cron.
       Enable with extended thresholds: stale_after_days: 180 (vs normal 90)
       for the first two weeks. Observe output, tighten to 90/30/30 after
       two full sweep cycles.

Week 5–6 — Git Operations
  4.1  /cherry-pick → test manually on release branch before enabling broadly
       cherry_pick.allowed_branch_pattern: "^release-.*"
```

### Rollout Checklist per Phase

Before enabling any new plugin on the live repo:
- [ ] Unit tests with mock client cover happy path and permission-denied path
- [ ] `stern config check` passes after `stern.yaml` changes
- [ ] `STERN_DRY_RUN=true` tested on a draft PR — output matches expectations
- [ ] New plugin added to `plugins:` list in `stern.yaml`
- [ ] `stern config sync-labels --dry-run` shows no unexpected label changes
- [ ] Branch protection configured with required status checks (before
  enabling `merge.strategy: native`)
- [ ] At least one team member reviewed the handler code

---

## Summary Table

| Feature | Phase | Slash Commands | Trigger | Key Dependency | Complexity | Est. Time |
|---|---|---|---|---|---|---|
| Dependency spike | 0.0 | — | manual | `google/go-github` | 🟢 Easy | 1–2h |
| Scaffold & CLI | 0.1 | — | all | `google/go-github` | 🟢 Easy | 2–3h |
| Permissions | 0.2 | — | all | `github.Client` mock | 🟢 Easy | 1–2h |
| Workflow YAMLs | 0.3 | — | all | — | 🟢 Easy | 0.5h |
| `config init` | 0.4 | CLI only | manual | — | 🟢 Easy | 1–2h |
| `config check` | 0.5 | CLI only | manual/CI | — | 🟢 Easy | 2–3h |
| `config sync-labels` | 0.6 | CLI only | manual | `github.Client` | 🟢 Easy | 2–3h |
| Merge eligibility | 1.5 | — | shared module | `github.Client` | 🟡 Medium | 2–3h |
| `/lgtm` + invalidation | 1.1 | `/lgtm`, `/lgtm cancel` | comment + push | `internal/owners` | 🟢 Easy | 2–3h |
| `/approve` | 1.2 | `/approve`, `/approve cancel` | comment + push | `internal/owners` | 🟡 Medium | 3–5h |
| `/hold` | 1.3 | `/hold`, `/hold cancel` | comment | `internal/labels` | 🟢 Easy | 1–2h |
| `/wip` | 1.4 | `/wip` | comment + push | `internal/labels` | 🟢 Easy | 1h |
| `/assign` + `/cc` | 2.1 | `/assign`, `/cc`, `/unassign`, `/uncc` | comment | `github.Client` | 🟢 Easy | 1–2h |
| `review_assignment` | 2.2 | (auto on PR open) | opened | `internal/owners` | 🟡 Medium | 4–5h |
| `/label` family | 2.3 | `/label`, `/kind`, `/area`, `/priority` | comment | `internal/labels` | 🟢 Easy | 1h |
| `/milestone` | 2.4 | `/milestone` | comment | `github.Client` | 🟢 Easy | 1h |
| `/close` + `/reopen` | 2.5 | `/close`, `/reopen` | comment | `github.Client` | 🟢 Easy | 1h |
| `/size` | 2.6 | (automatic) | push/opened | `internal/labels` | 🟢 Easy | 1–2h |
| `/retest` | 2.7 | `/retest` | comment | `github.Client` | 🟡 Medium | 2–3h |
| `/lifecycle` commands | 3.1 | `/lifecycle stale/frozen/active` | comment | `internal/labels` | 🟢 Easy | 1h |
| Lifecycle sweep | 3.2 | (scheduled) | cron | `github.Client` | 🟡 Medium | 4–6h |
| `/cherry-pick` | 4.1 | `/cherry-pick <branch>` | comment (merged PRs only) | `exec.Command("git")` | 🔴 Hard | 5–8h |

**Total estimate: 41–62 hours** (parallelizable after Phase 0; config tooling
can be built concurrently with scaffold)
