# Development

How to run, debug, and replay stern locally.

## Build

```sh
go build -o stern ./cmd/stern/
```

This produces a `stern` binary in the current directory. Alternatively, `go run ./cmd/stern/ ...` runs without a separate binary.

## Authentication

Three of the four trigger subcommands (`slash-command`, `pr-event`, `lifecycle`) and `config sync-labels` require a GitHub token. Without one they fail immediately:

```
building GitHub client: GITHUB_TOKEN is not set
```

The simplest source is your existing `gh` CLI session:

```sh
export GITHUB_TOKEN=$(gh auth token)
./stern ...
```

`gh auth token` prints the token for the account you're already logged in as, so no separate credential is needed. Run `gh auth status` to confirm you're authenticated, or `gh auth login` if not.

The token needs the same scopes the GitHub Actions workflow uses — see [`.github/workflows/stern-triggers.yaml`](../.github/workflows/stern-triggers.yaml) for the current `permissions:` blocks (typically `contents: write`, `issues: write`, `pull-requests: write` for the trigger jobs, plus `checks: write` for `/retest` and `id-token: write` for some merge modes). Your personal account already has these against any repo you collaborate on.

If you want a dedicated token, a classic PAT with the `repo` scope is the easiest; for fine-grained tokens, grant Contents (read/write), Issues (read/write), Pull requests (read/write), and (for auto-merge) the GraphQL `contents:write` permission.

## Dry-run mode

Two ways to enable dry-run:

```sh
STERN_DRY_RUN=true ./stern slash-command --config .github/stern.yaml
# or
./stern slash-command --config .github/stern.yaml --dry-run
```

In dry-run, all mutating calls are logged and skipped; reads still hit the live API. Use this to verify wiring before letting a real run touch your repo.

## Replaying an event from CI

The trigger subcommands parse a GitHub event payload from `GITHUB_EVENT_PATH`. In CI this is auto-populated by the workflow; locally you provide it yourself.

The workflow does not upload event payloads as artifacts, so a past run's webhook is not directly downloadable. The closest substitute is the GitHub API — not byte-identical to the webhook (no `action` / `sender` wrapper) but sufficient for exercising the trigger:

- `pull_request_target`: `gh api repos/OWNER/REPO/pulls/<n> > /tmp/event.json`
- `issue_comment` (slash commands): the original payload (comment body, sender, action) is only held by GitHub. Reconstruct a fixture JSON by hand, or use [`act`](https://github.com/nektos/act) to drive the workflow locally.

Point stern at the file:

```sh
GITHUB_EVENT_PATH=/tmp/event.json \
GITHUB_REPOSITORY=OWNER/REPO \
./stern pr-event --config .github/stern.yaml --dry-run
```

## `config` subcommands

`config init` and `config check` do not require a token — they read/write local files only:

```sh
go run ./cmd/stern/ config init --org YOUR_ORG --repo YOUR_REPO > .github/stern.yaml
go run ./cmd/stern/ config check --config .github/stern.yaml
```

`config sync-labels` does need a token because it calls the GitHub API to read/create/update labels.

## Common errors

| Error | Cause |
|---|---|
| `GITHUB_TOKEN is not set` | Set `GITHUB_TOKEN` or use `--dry-run` for non-mutating commands. |
| `401 Bad credentials` | Token expired or lacks scope. Re-run `gh auth login` or rotate the PAT. |
| `org/repo not set` | `globalOpts.Org` / `Repo` are empty. Set them in `.github/stern.yaml` or export `GITHUB_REPOSITORY=OWNER/REPO`. |
| `lifecycle: plugin not enabled` | `plugins: [lifecycle]` missing from config. |
| `failed to hydrate PR: file does not exist` | Only on `slash-command` for an `/approve` on a PR whose number can't be resolved. Usually a stale event payload. |

## Tests

```sh
go test ./...
golangci-lint run ./...
```

Coverage is recorded by CI and uploaded as a workflow artifact (`coverage-<run_id>`). See the test workflow for the exact command.
