# Quickstart

## 1. Copy the workflow

Copy [`.github/workflows/stern-triggers.yaml`](../.github/workflows/stern-triggers.yaml) into your repository's `.github/workflows/` directory. No changes are needed — it handles `issue_comment`, `pull_request_target`, and `issues` events.

## 2. Initialize config

```sh
go run ./cmd/stern/ config init --org YOUR_ORG --repo YOUR_REPO > .github/stern.yaml
```

Edit `.github/stern.yaml` to enable plugins and adjust settings. See [Configuration](configuration.md).

## 3. Sync labels

Create the required labels in your repository:

```sh
GITHUB_TOKEN=<token> go run ./cmd/stern/ config sync-labels --config .github/stern.yaml
```

Run `stern config sync-labels --check --config .github/stern.yaml` in CI to fail the build if labels drift from `stern.yaml`.

## 4. Enable plugins

In `.github/stern.yaml`:

```yaml
plugins:
  - lgtm
  - approve
  - hold
  - wip
```

Add more as needed: `cherry-pick`, `review_assignment`, `size`, `lifecycle`, `kind`, `area`, `priority`, `assign`, `retest`, `milestone`, `close`. See [Configuration → plugins](configuration.md#plugins).

## 5. Enable auto-merge

In your repository settings:

- **General → Allow auto-merge** — required for `merge.strategy: native` (the default).
- **Branches → branch protection** — either configure required status checks (GitHub auto-merges when checks pass) or enable a merge queue.

`merge.strategy: bot` does not need a merge queue; the bot token merges directly.

## Validate

```sh
go run ./cmd/stern/ config check --config .github/stern.yaml
```

## Solo / no-OWNERS

If you are a solo developer or your repo does not use OWNERS files, you can use stern out of the box. `/lgtm` and `/approve` fall back to repo write access when no OWNERS file covers the changed paths, so a single account with write access can run the full workflow (subject to `allow_self_lgtm` / `allow_self_approval`). See [Configuration → Solo and no-OWNERS](configuration.md#solo-and-no-owners).
