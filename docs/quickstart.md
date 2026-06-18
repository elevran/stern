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

## 4. Enable plugins

In `.github/stern.yaml`:

```yaml
plugins:
  - lgtm
  - approve
  - hold
  - wip
```

## 5. Enable auto-merge (optional)

In your repository settings, enable **Allow auto-merge** (Settings → General) and configure branch protection rules that require at least one status check. stern uses the GitHub GraphQL API to enable/disable auto-merge as label state changes.

## Validate

```sh
go run ./cmd/stern/ config check --config .github/stern.yaml
```
