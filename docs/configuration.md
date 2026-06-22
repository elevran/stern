# Configuration reference

Config is loaded from `.github/stern.yaml`. Generate a starter file with:

```sh
go run ./cmd/stern/ config init --org YOUR_ORG --repo YOUR_REPO > .github/stern.yaml
```

## Top-level fields

| Field | Default | Description |
|-------|---------|-------------|
| `org` | — | GitHub organization name |
| `repo` | — | Repository name |
| `bot_login` | `github-actions[bot]` | Login of the GitHub Actions bot; used to prevent bot-triggered event loops |

## plugins

List of plugin names to enable. Available plugins:

| Plugin | Commands | Notes |
|--------|----------|-------|
| `lgtm` | `/lgtm`, `/lgtm cancel` | LGTM label management |
| `approve` | `/approve`, `/approve cancel` | Always requires an OWNERS-listed approver, or repo write access if no OWNERS covers the changed paths |
| `hold` | `/hold`, `/hold cancel` | Blocks auto-merge |
| `wip` | `/wip` | WIP detection from title and draft state |
| `cherry-pick` | `/cherry-pick <sha>` | Backport commits to other branches |
| `review_assignment` | — | Automatic reviewer assignment via OWNERS |
| `size` | — | Automatic `size/*` labels based on diff size |
| `lifecycle` | — | Stale/rotten label management via scheduled runs |

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

`strategy: native` uses GitHub's built-in auto-merge (requires branch protection). `strategy: bot` has the bot account merge directly when eligible.

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

## label_definitions

Defines labels that `config sync-labels` will create or update in the repository. See the generated `stern.yaml` for the full default set.
