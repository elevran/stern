# Phase 0 & Phase 1 — Detailed Task Plan

> Each task has explicit passing criteria. Tasks marked **stern repo test**
> describe how to verify the implementation against the live `elevran/stern`
> repo once the workflow is deployed (Phase 0.3 onward).

---

## Phase 0 — Foundation

---

### Task 0.0 — Dependency Spike

**What:** Validate dependency choices before writing any feature code. One
throwaway script/main that is run manually, not merged.

**Steps:**

1. `go get github.com/google/go-github/v66` — construct a client from
   `$GITHUB_TOKEN`, call `ListLabels("elevran", "stern")`, print results.
2. Test GitHub auto-merge REST endpoint: `PUT /repos/elevran/stern/pulls/{n}/auto_merge`
   with a test PR — does it return 405 (not found) or 200/201? If 405,
   test `go get github.com/shurcooL/githubv4` and run the
   `enablePullRequestAutoMerge` GraphQL mutation.
3. Optional: `go get sigs.k8s.io/prow` — does it compile? If yes, does
   `pkg/repoowners.NewRepoOwners()` work without a full prow config?

**Deliverable:** Commit `docs/spike-results.md` to main with:
- go-github: works / doesn't work
- Auto-merge: REST works / GraphQL needed (and which library)
- Prow: imports cleanly / fails (and whether repoowners is usable)
- Confirmed dependency decisions for all subsequent phases

**Passing criteria:**
- [ ] `docs/spike-results.md` exists and answers all three questions
- [ ] Confirmed: GitHub client library to use
- [ ] Confirmed: Auto-merge implementation approach (REST vs GraphQL)
- [ ] Confirmed: OWNERS parser approach (local vs prow repoowners)

---

### Task 0.1 — Project Scaffold & CLI Entry Point

**What:** Go module, Cobra CLI, event parsing, Options struct, GitHub client
construction, command dispatch skeleton, bot-identity guard.

**Files:**
- `go.mod` — `module github.com/elevran/stern`
- `cmd/stern/main.go` — Cobra root, subcommand wiring
- `internal/config/options.go` — `Options` struct, `LoadFromFile()`,
  `applyDefaults()`, `validate()` (stub — full validation in 0.5)
- `internal/event/types.go` — `CommentEvent`, `PREvent`, `IssueEvent`,
  `CheckSuiteEvent`
- `internal/event/parse.go` — unmarshal `$GITHUB_EVENT_PATH`
- `internal/github/client.go` — `Client` interface, `NewFromEnv()`,
  `DryRunClient` wrapper
- `internal/github/mock.go` — in-process mock for tests
- `internal/commands/registry.go` — `Handler` interface, dispatch loop,
  bot-identity check, `/ping` built-in handler
- `Makefile` — `build`, `test`, `lint` targets
- `.github/stern.yaml` — initial config with `plugins: []`

**Key behaviors:**
- `slash-command`: reads event → checks if author is bot (exit 0 if yes) →
  parses all `/`-prefixed lines → dispatches each by verb
- For `issue_comment` on a PR: calls `GetPullRequest()` to hydrate `Context.PR`
- `STERN_DRY_RUN=true`: wraps client so mutating calls are logged, not executed
- Unknown command verb: log "no handler for /foo", continue to next line

**Passing criteria:**
- [x] `make build` compiles with zero errors
- [x] `make test` passes (coverage of dispatch, bot-guard, event parsing)
- [ ] `make lint` passes (golangci-lint or similar)
- [x] `GITHUB_EVENT_PATH=testdata/comment.json stern slash-command` exits 0
- [ ] `/ping` on a PR comment adds `+1` reaction and exits 0
- [x] Comment from `github-actions[bot]` author: exits 0 without dispatching
- [x] Unknown command `/hello`: logs "no handler", exits 0
- [x] `STERN_DRY_RUN=true`: startup log shows "dry-run mode enabled"
- [x] Multiple commands in one comment body are each dispatched independently

**Stern repo test:**
Deploy workflow from Task 0.3 first, then:
- Post `/ping` on any open PR → Actions job turns green, `+1` reaction appears
- Post `/hello` on a PR → Actions job turns green, no reaction (unknown command)
- Check the Actions run log: event fields are parsed correctly
- Post a comment as the bot (copy `github-actions[bot]` scenario manually) —
  the job should fire but exit immediately with no reaction

---

### Task 0.2 — Permissions Checker

**What:** Centralized permission checks used by all handlers.

**Files:**
- `internal/permissions/checker.go`
- `internal/permissions/checker_test.go`

**Interface:**
```go
type Checker interface {
    IsOrgMember(org, user string) (bool, error)
    HasWriteAccess(org, repo, user string) (bool, error)
    IsPRAuthor(pr *github.PullRequest, user string) bool
    IsBot(user string) bool        // compares to BotLogin in Context
}
```

**Passing criteria:**
- [x] `IsOrgMember`: returns true for org members, false for non-members
- [x] `HasWriteAccess`: returns true for write/maintain/admin collaborators
- [x] `IsPRAuthor`: returns true only for the PR author login
- [x] `IsBot`: returns true only when login matches `Context.BotLogin`
- [x] All tests use the mock client — no network calls
- [x] Table-driven tests cover all combinations

---

### Task 0.3 — Actions Workflow Skeleton

**What:** Deploy the trigger workflows. This is the point where stern begins
running on its own repo — even with `plugins: []`, it exercises the full
invocation path.

**Files:**
- `.github/workflows/stern-triggers.yaml` — all five jobs:
  `slash-command`, `pr-event`, `issue-event`, `merge-check` (commented out),
  `lifecycle-sweep` (commented out)
- `.github/workflows/stern-config-check.yaml` — validates stern.yaml on PRs
- `.github/stern.yaml` — `plugins: []`

**Key details:**
- `slash-command` job: fires on all `issue_comment: created` where body
  starts with `/`; no `pull_request != null` filter
- `slash-command` job: `concurrency: group: stern-comment-${{ github.event.issue.number }}, cancel-in-progress: false`
- `issue-event` job: fires on `issues: [opened]`
- `merge-check` and `lifecycle-sweep` jobs: present but commented out with
  clear instructions for when to enable them
- `stern-config-check.yaml`: uses `pull_request` (not `pull_request_target`)
  since it is read-only validation on the PR branch

**Passing criteria:**
- [ ] `actionlint` (or similar) validates both YAML files with zero errors
- [ ] `slash-command` job fires on a test PR comment starting with `/`
- [x] `slash-command` job does NOT have a `pull_request != null` filter
- [x] `pr-event` job fires on PR opened/synchronize/edited/closed/reopened
- [x] `issue-event` job fires on new issue open
- [x] `stern-config-check` fires on PRs that touch `.github/stern.yaml`
- [ ] With `plugins: []`, all jobs exit 0 (no handlers triggered)
- [x] `concurrency:` group is set correctly on `slash-command` job

**Stern repo test:**
- Post `/ping` on a PR → `slash-command` job runs green
- Push a commit to an open PR → `pr-event` job runs green
- Open a new issue → `issue-event` job runs green
- Open a PR that changes `.github/stern.yaml` → `stern-config-check` runs
- Verify no duplicate job runs for a burst of comments (concurrency serializes)

---

### Task 0.4 — `stern config init`

**CLI:** `stern config init [--output .github/stern.yaml] [--org ORG] [--repo REPO]`

**What:** Generates a complete, commented `stern.yaml` from an embedded
template. First command a new operator runs.

**Files:**
- `cmd/stern/config_init.go` (or `internal/config/init.go`)
- `internal/config/template.yaml.tmpl` — embedded via `//go:embed`

**Key behaviors:**
- Detects org/repo from `$GITHUB_REPOSITORY` if flags absent
- Template uses YAML comments to explain every field
- All plugins listed but disabled by default (`plugins: []`)
- After writing, prints numbered next steps

**Passing criteria:**
- [ ] `stern config init --org elevran --repo stern` writes `stern.yaml`
- [ ] Written file parses back to `Options` with zero `validate()` errors
- [ ] All known plugin names appear in the file (commented out in plugins list)
- [ ] `$GITHUB_REPOSITORY=elevran/stern` auto-detects org and repo correctly
- [ ] Existing file: prompts before overwriting (or respects `--force`)
- [ ] Test: render template, parse back, assert zero validation errors

---

### Task 0.5 — `stern config check`

**CLI:** `stern config check [--config .github/stern.yaml]`

**What:** Calls `Options.validate()`, reports all issues with color output,
exits non-zero on any ERROR. Used as a CI gate via `stern-config-check.yaml`.

**Validation rules (each a separate function in `validate()`):**
- Unknown plugin name → ERROR with "did you mean X?" (edit distance)
- Invalid `merge.method` (not squash/merge/rebase) → ERROR
- Invalid `merge.strategy` (not native/bot) → ERROR
- `cherry-pick` in plugins but `cherry_pick.allowed_branch_pattern` empty → ERROR
- `cherry_pick.allowed_branch_pattern` doesn't compile as Go regex → ERROR
- Label names in plugin config not in `label_definitions` → ERROR
- Lifecycle thresholds not positive integers → ERROR
- `merge.strategy: native` with empty `blocking_labels` → WARN

**Output format:**
```
.github/stern.yaml — 2 issues found

  ERROR  plugins[2]: unknown plugin "lgmt" (did you mean "lgtm"?)
  WARN   merge.blocking_labels is empty — hold labels will not block auto-merge

Exit code: 1
```

**Passing criteria:**
- [ ] Each validation rule has its own test case (table-driven)
- [ ] ALL issues collected before reporting — no early exit
- [ ] Exits 1 if any ERROR, 0 if WARN-only or clean
- [ ] Color output: red ERROR, yellow WARN (skipped in non-TTY)
- [ ] "Did you mean?" fires for plugin names within edit distance 2
- [ ] Valid `stern.yaml` exits 0 with "No issues found"

**Stern repo test:**
- Open a PR that sets `plugins: [lgmt]` (typo) → `stern-config-check` fails
  with the ERROR displayed in the job log
- Open a PR with valid `stern.yaml` → `stern-config-check` passes

---

### Task 0.6 — `stern config sync-labels`

**CLI:** `stern config sync-labels [--dry-run] [--prune] [--yes] [--config .github/stern.yaml]`

**What:** Reads `label_definitions` from `stern.yaml`, fetches existing repo
labels, and reconciles. Required before enabling any plugin.

**Diff categories:**
- **CREATE** — in config, not in repo
- **UPDATE** — in both, color or description differs
- **OK** — exact match
- **EXTRA** — in repo, not in config (reported; deleted only with `--prune`)

**Passing criteria:**
- [ ] Dry-run: prints correct CREATE/UPDATE/OK/EXTRA table, exits 0, no API mutations
- [ ] Creates labels not present in repo
- [ ] Updates labels with changed color or description
- [ ] Does not delete EXTRA labels without `--prune`
- [ ] `--prune --yes` deletes EXTRA labels without prompting
- [ ] `--prune` without `--yes` prompts "Delete N extra labels? [y/N]"
- [ ] Color output: green CREATE/OK, yellow UPDATE, grey EXTRA
- [ ] Tests use mock client (no network)

**Stern repo test:**
- Run `stern config sync-labels --dry-run` on `elevran/stern` — review the
  label plan
- Run without `--dry-run` — all `label_definitions` labels now exist in repo
- Verify in GitHub → Labels that all expected labels are present with correct
  colors and descriptions

---

## Phase 1 — Core Review Flow

> Build 1.5 first — it is shared infrastructure for 1.1 through 1.4.

---

### Task 1.5 — Merge Eligibility & Auto-Merge Module

**What:** Shared infrastructure for all label-modifying handlers. No slash
command — this is a library package.

**Files:**
- `internal/merge/eligibility.go` + `eligibility_test.go`
- `internal/merge/automerge.go`

```go
type EligibilityResult struct {
    Ready          bool
    MissingLabels  []string  // e.g. ["lgtm"]
    BlockingLabels []string  // e.g. ["do-not-merge/hold"]
}

func CheckEligibility(pr *github.PullRequest, opts *config.Options) EligibilityResult

func EnableAutoMerge(ghc github.Client, pr *github.PullRequest, method string) error
func DisableAutoMerge(ghc github.Client, pr *github.PullRequest) error
```

`EnableAutoMerge` / `DisableAutoMerge` implementation depends on spike result:
- REST (`PUT /pulls/{n}/auto_merge`) if endpoint exists
- GraphQL (`enablePullRequestAutoMerge` mutation via `githubv4`) otherwise

`EnableAutoMerge` treats "already enabled" as success. `DisableAutoMerge`
treats "not enabled" as success.

**Passing criteria:**
- [ ] `CheckEligibility`: ready when `lgtm` + `approved` present, no blockers
- [ ] `CheckEligibility`: not ready when `lgtm` missing
- [ ] `CheckEligibility`: not ready when `approved` missing
- [ ] `CheckEligibility`: not ready when `do-not-merge/hold` present
- [ ] `CheckEligibility`: not ready when `do-not-merge/work-in-progress` present
- [ ] `CheckEligibility`: multiple blockers reported together
- [ ] `EnableAutoMerge`: "already enabled" response → returns nil
- [ ] `DisableAutoMerge`: "not enabled" response → returns nil
- [ ] All tests use mock client, no network
- [ ] Table-driven tests for all `CheckEligibility` combinations

---

### Task 1.1 — `/lgtm` and LGTM Invalidation on Push

**Slash commands:** `/lgtm`, `/lgtm cancel`
**PR event:** `synchronize`

**What:**
- `/lgtm`: checks commenter is not PR author (if `allow_self_lgtm: false`);
  if OWNERS file present for touched paths, checks commenter is in
  `reviewers`; adds `lgtm` label; calls `CheckEligibility()` — if ready,
  calls `EnableAutoMerge()`; reacts `+1` on success
- `/lgtm cancel`: removes `lgtm` label; calls `DisableAutoMerge()`; reacts `+1`
- On `synchronize`: if `invalidate_on_push: true`, removes `lgtm` label,
  calls `DisableAutoMerge()` (silent — label removal is visible in timeline)

OWNERS files are fetched from the PR's head SHA via `internal/owners`.

**Files:**
- `internal/commands/lgtm.go` + `lgtm_test.go`
- `internal/owners/parse.go` + `parse_test.go` (OWNERS parser)
- `internal/pr/events.go` — `synchronize` handler calling lgtm invalidation

**Passing criteria:**
- [ ] `/lgtm` adds `lgtm` label
- [ ] `/lgtm cancel` removes `lgtm` label
- [ ] `/lgtm` by PR author → `-1` reaction + error comment (if `allow_self_lgtm: false`)
- [ ] `/lgtm` by non-reviewer when OWNERS present → `-1` reaction + error comment
- [ ] `/lgtm` with no OWNERS file → succeeds for any org member
- [ ] `+1` reaction added to triggering comment on success
- [ ] `synchronize` removes `lgtm` when `invalidate_on_push: true`
- [ ] `synchronize` does NOT remove `lgtm` when `invalidate_on_push: false`
- [ ] `CheckEligibility` called after label change; `EnableAutoMerge` called when ready
- [ ] OWNERS parser: resolves `approvers:` and `reviewers:` lists
- [ ] OWNERS parser: walks directory hierarchy to root
- [ ] OWNERS parser: resolves `OWNERS_ALIASES` from root alias file
- [ ] All tests use mock client

**Stern repo test:**

Prerequisites: add an `OWNERS` file to `elevran/stern` root with `reviewers: [elevran]`.
Enable `lgtm` plugin in `stern.yaml`. Run `stern config sync-labels`.

- Post `/lgtm` as `elevran` on a test PR → `lgtm` label added, `+1` reaction
- Post `/lgtm` as a non-reviewer GitHub account → `-1` reaction + error comment
- Post `/lgtm` as PR author → `-1` reaction + error comment
- Push a commit to the PR → `lgtm` label removed (timeline records it)
- Post `/lgtm cancel` → `lgtm` label removed, `+1` reaction

---

### Task 1.2 — `/approve`

**Slash commands:** `/approve`, `/approve cancel`
**PR event:** `synchronize` (if `invalidate_on_push: true`)

**What:**
- `/approve`: fetches changed files via `GetPullRequestChanges()`; checks
  commenter appears in `LeafApprovers` for at least one OWNERS file covering
  the changed paths; blocks self-approval if `allow_self_approval: false`;
  adds `approved` label; calls `CheckEligibility()` — if ready, calls
  `EnableAutoMerge()`; reacts `+1`
- `/approve cancel`: removes `approved` label; calls `DisableAutoMerge()`; reacts `+1`
- On `synchronize`: if `approve.invalidate_on_push: true`, removes `approved`,
  calls `DisableAutoMerge()` (silent)

**Files:**
- `internal/commands/approve.go` + `approve_test.go`

**Passing criteria:**
- [ ] `/approve` adds `approved` label when commenter is a leaf approver
- [ ] `/approve` denied when commenter not in any OWNERS approvers set → `-1` + comment
- [ ] Self-approval blocked if `allow_self_approval: false` → `-1` + comment
- [ ] `/approve cancel` removes `approved` label, calls `DisableAutoMerge`
- [ ] `CheckEligibility` called after label add; `EnableAutoMerge` called when
  both `lgtm` and `approved` are present
- [ ] `synchronize` removes `approved` when `invalidate_on_push: true`
- [ ] All tests use mock client

**Stern repo test:**

Prerequisites: same OWNERS setup as 1.1, `approve` in plugins list.

- Post `/approve` as `elevran` (who is in OWNERS approvers) on a test PR
  that also has `lgtm` → `approved` label added, auto-merge enabled
  (merge button shows "This pull request will be automatically merged")
- Post `/approve` as a non-approver → `-1` reaction + error comment
- Push a commit after approval → `approved` removed (visible in timeline)
- Post `/approve` as PR author → `-1` if `allow_self_approval: false`

---

### Task 1.3 — `/hold`

**Slash commands:** `/hold`, `/hold cancel`

**What:**
- `/hold`: any contributor may apply; adds `do-not-merge/hold` label; calls
  `DisableAutoMerge()`; reacts `+1`
- `/hold cancel`: requires original commenter or write access; removes label;
  calls `CheckEligibility()` — re-enables auto-merge if eligible; reacts `+1`

**Files:**
- `internal/commands/hold.go` + `hold_test.go`

**Passing criteria:**
- [ ] `/hold` adds `do-not-merge/hold` label and calls `DisableAutoMerge`
- [ ] `/hold` succeeds for any org member (not write-access-only)
- [ ] `/hold cancel` removes label and calls `CheckEligibility`
- [ ] `/hold cancel` by a third party without write access → `-1` + error comment
- [ ] After `/hold cancel` when PR is otherwise eligible: `EnableAutoMerge` called
- [ ] `+1` reaction on all successful operations

**Stern repo test:**
- Post `/hold` → label added, merge button shows auto-merge off
- Post `/hold cancel` → label removed; if `lgtm` + `approved` present,
  auto-merge re-enabled (merge button reflects it)

---

### Task 1.4 — `/wip` and Title-Based WIP Detection

**Slash commands:** `/wip`
**PR events:** `opened`, `edited` (title changed), `synchronize`, `reopened`

**What:**
- `/wip`: toggles `do-not-merge/work-in-progress`; calls
  `DisableAutoMerge()` when adding, `CheckEligibility()` when removing; reacts `+1`
- `opened` / `synchronize` / `reopened`: if title matches
  `WIP`, `[WIP]`, `[Draft]`, or `Draft:` prefix → add WIP label (silent)
- `edited`: only re-evaluates WIP if `event.Changes.Title != nil`
  (title actually changed, not body or base)
- Draft PR: `opened` with `draft: true` → add WIP label; when un-drafted
  (`pull_request.draft` becomes false on next event) → remove WIP label,
  call `CheckEligibility()`

**Files:**
- `internal/commands/wip.go` + `wip_test.go`
- `internal/pr/events.go` — WIP detection on PR events

**Passing criteria:**
- [ ] `/wip` adds WIP label when absent; removes when present (toggle)
- [ ] `DisableAutoMerge` called when adding WIP
- [ ] `CheckEligibility` called when removing WIP
- [ ] PR opened with `[WIP]` in title → WIP label added (silent)
- [ ] PR opened with clean title → no WIP label
- [ ] PR `edited` with title change from `[WIP] foo` to `foo` → WIP removed
- [ ] PR `edited` with body-only change (no `changes.title`) → WIP unchanged
- [ ] Draft PR open → WIP label added
- [ ] PR un-drafted → WIP label removed, eligibility checked

**Stern repo test:**
- Open a PR titled `[WIP] test pr` → WIP label appears automatically
- Edit title to remove `[WIP]` → WIP label removed (timeline shows it)
- Post `/wip` → WIP label added, `+1` reaction
- Post `/wip` again → WIP label removed, `+1` reaction
- Open a draft PR → WIP label appears; convert to ready → label removed

---

## Enabling Self-Hosting

After Task 1.4, update `stern.yaml`:

```yaml
plugins:
  - lgtm
  - approve
  - hold
  - wip

merge:
  strategy: native
  method: squash
  blocking_labels:
    - "do-not-merge/hold"
    - "do-not-merge/work-in-progress"
    - "needs-rebase"
```

**Prerequisites before enabling `merge.strategy: native`:**
- Branch protection on `main` must have at least one required status check.
  Without this, GitHub auto-merge triggers immediately on enablement —
  before CI runs.
- Run `stern config sync-labels` to ensure all labels exist in the repo.
- Test with `STERN_DRY_RUN=true` on a draft PR first.

From this point, all stern PRs are managed by stern itself.

---

## Rollout Checklist (per phase)

Before enabling any new plugin on the live repo:

- [ ] Unit tests pass with mock client: happy path + permission-denied path
- [ ] `stern config check` passes after `stern.yaml` changes
- [ ] `STERN_DRY_RUN=true` tested on a draft PR — output matches expectations
- [ ] New plugin added to `plugins:` list in `stern.yaml`
- [ ] `stern config sync-labels --dry-run` shows no unexpected label changes
- [ ] Branch protection configured with at least one required status check
      (required before enabling `merge.strategy: native`)
- [ ] At least one reviewer has read the handler code
