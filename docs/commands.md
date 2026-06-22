# Slash commands

Commands are posted as issue comments on a pull request (or, for some, on an issue). stern reacts with `+1` on success and `-1` on failure or permission denied.

## Review

| Command | Who can use | Effect |
|---------|-------------|--------|
| `/lgtm` | Org member; falls back to repo write access when no OWNERS covers the changed files | Adds `lgtm` label |
| `/lgtm cancel` | Repo write access required | Removes `lgtm` label |
| `/approve` | OWNERS approver; falls back to repo write access when no OWNERS covers the changed files | Adds `approved` label |
| `/approve cancel` | Repo write access required | Removes `approved` label |

A PR is auto-merged when it has both the `lgtm` and `approved` labels and none of the configured `blocking_labels`. See [Auto-merge eligibility](#auto-merge-eligibility) below.

Self-approval (`/lgtm` or `/approve` by the PR author) is blocked by default. See `allow_self_lgtm` and `allow_self_approval` in [Configuration](configuration.md#lgtm). LGTM and approval can be invalidated on push via `invalidate_on_push`.

## Blocking

| Command | Who can use | Effect |
|---------|-------------|--------|
| `/hold` | Any org member | Adds `do-not-merge/hold`; disables auto-merge |
| `/hold cancel` | Repo write access required | Removes `do-not-merge/hold`; re-evaluates auto-merge eligibility |
| `/wip` | PR author | Toggles `do-not-merge/wip`; disables auto-merge when added, re-evaluates when removed |
| `/wip cancel` | PR author | Removes `do-not-merge/wip` |
| `/close` | Repo write access required | Closes an issue or PR |
| `/reopen` | Repo write access required | Reopens a closed issue or PR |

WIP status is also detected automatically from PR title prefixes (`WIP:`, `[WIP]`, `[Draft]`, `Draft:`) and the GitHub draft state.

## Backport

| Command | Who can use | Effect |
|---------|-------------|--------|
| `/cherry-pick <sha>` | Repo write access required | Cherry-picks the merged PR onto branches matching `cherry_pick.allowed_branch_pattern` |

`<sha>` is a commit on the merged PR's branch. The bot pushes a branch per target and opens a tracking PR.

## Triage

| Command | Who can use | Effect |
|---------|-------------|--------|
| `/kind <name>` | Anyone | Sets the `kind/<name>` label |
| `/area <name>` | Anyone | Sets the `area/<name>` label |
| `/priority <name>` | Anyone | Sets the `priority/<name>` label |

Each command replaces the previous label in its namespace (e.g. `/area foo` then `/area bar` leaves only `area/bar`).

## Assignment

| Command | Who can use | Effect |
|---------|-------------|--------|
| `/assign @user` | Anyone | Adds the user as an assignee |
| `/unassign @user` | Anyone | Removes the user from assignees |
| `/cc @user` | Anyone | Requests a review from the user |
| `/uncc @user` | Anyone | Removes a review request |

Usernames can be `@user`, bare `user`, or an email address.

## Lifecycle

| Command | Who can use | Effect |
|---------|-------------|--------|
| `/milestone <name>` | Repo write access required | Sets the issue/PR milestone |
| `/retest` | Anyone | Re-runs the failed CI checks on the PR's head commit |
| `/lifecycle <stale\|rotten\|frozen\|active>` | Anyone (issues and PRs) | Manually sets or clears the matching `lifecycle/*` label. Each subcommand is mutually exclusive — setting one clears the other two; `active` clears all three. |

A separate scheduled workflow (cron) calls `stern lifecycle --config …` to automatically promote issues from `lifecycle/stale` → `lifecycle/rotten` and (optionally) close `lifecycle/rotten` issues per `lifecycle` config. That subcommand is independent of the `/lifecycle` slash command above.

## Utility

| Command | Who can use | Effect |
|---------|-------------|--------|
| `/ping` | Anyone | Bot health check — stern replies with a comment |
| `/help` | Anyone | Lists available commands |

## Auto-merge eligibility

Auto-merge is enabled when ALL of the following are true:

- The PR has the `lgtm` label
- The PR has the `approved` label
- The PR has none of the `blocking_labels` listed in config (default: `do-not-merge/hold`, `do-not-merge/wip`, `needs-rebase`)

It is disabled whenever any condition is no longer met. If `invalidate_on_push` is set for `lgtm` or `approve`, a new push removes the label and disables auto-merge.

`merge.strategy: native` (the default) uses GitHub's built-in auto-merge and requires a merge queue or branch protection rule with required status checks. `merge.strategy: bot` has the bot account merge directly when eligible, which does not need a merge queue.

## Solo and no-OWNERS mode

stern works out of the box for solo developers and for repos that do not use OWNERS files. The auth checks degrade gracefully:

- **`/lgtm` and `/approve`** — require the caller to be in OWNERS approvers/reviewers for the changed files, OR have repo write access when no OWNERS file covers any changed file. A solo developer with write access can therefore LGTM and approve their own PRs on a repo with no OWNERS files (subject to `allow_self_lgtm` / `allow_self_approval`).
- **`/hold cancel`, `/approve cancel`, `/close`, `/reopen`, `/milestone`, `/cherry-pick`** — always require repo write access, regardless of OWNERS.

If you want a tighter policy than the write-access fallback (e.g. only OWNERS-listed approvers may approve, even when no OWNERS covers a path), add OWNERS files to your repo.
