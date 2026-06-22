# Configuration reference

Config is loaded from `.github/stern.yaml`. Generate a starter file with:

```sh
go run ./cmd/stern/ config init --org YOUR_ORG --repo YOUR_REPO > .github/stern.yaml
```

Validate any time with:

```sh
go run ./cmd/stern/ config check --config .github/stern.yaml
```

## Top-level fields

| Field | Default | Description |
|-------|---------|-------------|
| `org` | — | GitHub organization name |
| `repo` | — | Repository name |
| `bot_login` | `github-actions[bot]` | Login of the GitHub Actions bot; used to prevent bot-triggered event loops |

## plugins

List of plugin names to enable. Unknown plugin names are rejected by `config check`; a close match (edit distance ≤ 2) is suggested.

| Plugin | Slash commands | Notes |
|--------|----------------|-------|
| `lgtm` | `/lgtm`, `/lgtm cancel` | LGTM label management |
| `approve` | `/approve`, `/approve cancel` | Always requires an OWNERS-listed approver, or repo write access if no OWNERS covers the changed paths |
| `hold` | `/hold`, `/hold cancel` | Adds `do-not-merge/hold`; blocks auto-merge |
| `wip` | `/wip`, `/wip cancel` | WIP detection from title and draft state |
| `close` | `/close`, `/reopen` | Close and reopen issues and PRs |
| `milestone` | `/milestone <name>` | Set the issue/PR milestone |
| `retest` | `/retest` | Re-run failed CI checks on the PR's head commit |
| `assign` | `/assign`, `/unassign`, `/cc`, `/uncc` | Manage assignees and review requests |
| `cherry-pick` | `/cherry-pick <sha>` | Backport commits to other branches |
| `review_assignment` | — | Automatic reviewer assignment via OWNERS |
| `size` | — | Automatic `size/*` labels based on diff size |
| `lifecycle` | — | Stale/rotten label management via scheduled runs |
| `kind` | `/kind <name>` | Set `kind/<name>` label |
| `area` | `/area <name>` | Set `area/<name>` label |
| `priority` | `/priority <name>` | Set `priority/<name>` label |

## merge

```yaml
merge:
  strategy: native   # native | bot
  method: squash     # squash | merge | rebase
  blocking_labels:
    - do-not-merge/hold
    - do-not-merge/wip
    - needs-rebase
```

- `strategy: native` (default) uses GitHub's built-in auto-merge. Requires **Allow auto-merge** to be enabled (Settings → General) and either a merge queue or a branch protection rule with required status checks.
- `strategy: bot` has the bot account merge directly when eligible, using the configured `method`. Does not need a merge queue; relies on the bot token having merge rights.
- `blocking_labels` is the list of labels whose presence disables auto-merge. Defaults to `do-not-merge/hold`, `do-not-merge/wip`, and `needs-rebase`. Names referenced here must exist in `label_definitions` (see below).

## lgtm

```yaml
lgtm:
  allow_self_lgtm: false       # allow PR author to LGTM their own PR
  invalidate_on_push: true     # remove lgtm label when new commits are pushed
```

## approve

```yaml
approve:
  allow_self_approval: false   # allow PR author to approve their own PR
  invalidate_on_push: false    # remove approved label on new commits
```

When `invalidate_on_push` is true and a push invalidates the approval, the bot also dismisses its own `APPROVED` review.

## cherry_pick

```yaml
cherry_pick:
  allowed_branch_pattern: "^release-.*"
  command: cherry-pick         # cherry-pick | cherrypick | cp
```

`allowed_branch_pattern` is a regular expression matched against the names of the branches you want the merged PR backported onto. The plugin is enabled only when this is set.

## review_assignment

```yaml
review_assignment:
  enabled: false
  load_balancing: round-robin  # round-robin | least-busy
```

Automatic reviewer assignment from OWNERS reviewers lists. `load_balancing` controls how the bot picks among multiple matching reviewers.

## size

```yaml
size:
  enabled: false
  buckets:
    XS: 10    # max lines for size/XS (exclusive of next bucket)
    S: 30
    M: 100
    L: 500
    XL: 1000
```

When enabled, the bot assigns `size/*` based on the diff line count. See the generated `stern.yaml` for the default buckets.

## lifecycle

```yaml
lifecycle:
  stale_days: 90       # days idle before adding lifecycle/stale
  rotten_days: 30      # additional days before promoting to lifecycle/rotten
  close_rotten: false  # automatically close issues that reach lifecycle/rotten
```

Runs on a schedule (the workflow in `.github/workflows/` invokes the bot). Per-PR and per-issue overrides are supported via `lifecycle.pull_requests` and `lifecycle.issues`.

## label_definitions

Defines labels that `config sync-labels` will create or update in the repository. See the generated `stern.yaml` for the full default set (lgtm, approved, do-not-merge/hold, do-not-merge/wip, needs-rebase, size/*, lifecycle/*).

The CI gate `stern config sync-labels --check` exits non-zero if the repo is missing any of these labels or has drifted in color/description — recommended for branch protection required status checks.

## Solo and no-OWNERS

No config knob is required: when no OWNERS file covers a PR's changed paths, `/lgtm` and `/approve` fall back to repo write access. A solo developer with write access can therefore run a complete workflow on a repo with no OWNERS files (subject to `allow_self_lgtm` / `allow_self_approval`).

To tighten the policy beyond write access, add OWNERS files at the directories you want to gate. See the `OWNERS` file at the repo root for the format.
