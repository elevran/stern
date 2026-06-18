# Slash commands

Commands are posted as issue comments on a pull request. stern reacts with `+1` on success and `-1` on failure or permission denied.

## Review

| Command | Who can use | Effect |
|---------|-------------|--------|
| `/lgtm` | Any org member | Adds `lgtm` label; enables auto-merge when PR is also approved |
| `/lgtm cancel` | Any org member | Removes `lgtm` label; disables auto-merge |
| `/approve` | OWNERS approver (any org member if no OWNERS) | Adds `approved` label; enables auto-merge when PR also has lgtm |
| `/approve cancel` | OWNERS approver | Removes `approved` label; disables auto-merge |

Self-approval (`/lgtm` or `/approve` by the PR author) is blocked by default. See `allow_self_lgtm` and `allow_self_approval` in [Configuration](configuration.md).

## Blocking

| Command | Who can use | Effect |
|---------|-------------|--------|
| `/hold` | Any org member | Adds `do-not-merge/hold`; disables auto-merge |
| `/hold cancel` | Write access required | Removes `do-not-merge/hold`; re-evaluates auto-merge eligibility |
| `/wip` | PR author | Toggles `do-not-merge/wip`; disables auto-merge when added, re-evaluates when removed |

WIP status is also detected automatically from PR title prefixes (`WIP:`, `[WIP]`, `[Draft]`, `Draft:`) and the GitHub draft state.

## Utility

| Command | Who can use | Effect |
|---------|-------------|--------|
| `/ping` | Anyone | Bot health check — stern replies with a comment |

## Auto-merge eligibility

Auto-merge is enabled when ALL of the following are true:

- The PR has the `lgtm` label
- The PR has the `approved` label
- The PR has none of the `blocking_labels` listed in config (default: `do-not-merge/hold`, `do-not-merge/wip`, `needs-rebase`)

It is disabled whenever any condition is no longer met.
