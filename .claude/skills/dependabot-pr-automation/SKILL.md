---
name: dependabot-pr-automation
description: Reviews open Dependabot pull requests, assesses their risk level based on version bump type and CI status, approves low-risk PRs, and merges them. Use when asked to process, review, merge, or triage Dependabot PRs.
allowed-tools:
  - Bash
  - Read
  - WebFetch
  - mcp__github__list_pull_requests
  - mcp__github__get_pull_request
  - mcp__github__get_pull_request_files
  - mcp__github__get_pull_request_status
  - mcp__github__create_pull_request_review
  - mcp__github__merge_pull_request
---

# Dependabot PR Automation for chainloop

This skill reviews open Dependabot pull requests, assesses their risk, approves safe ones, and merges them.

## Repository Info

| Item | Value |
|------|-------|
| Owner | `chainloop-dev` |
| Repo | `chainloop` |

## Step 1: List Open Dependabot PRs

Use `mcp__github__list_pull_requests` to fetch open PRs:

- `owner`: `chainloop-dev`
- `repo`: `chainloop`
- `state`: `open`

Filter the results to only include PRs authored by `dependabot[bot]`. Collect each PR's number, title, head branch, and labels.

If there are no open Dependabot PRs, report that and stop.

## Step 2: Assess Risk for Each PR

For each Dependabot PR, determine the risk level using these criteria:

### 2a. Parse Version Bump from PR Title

Dependabot PR titles follow the pattern: `Bump <package> from <old-version> to <new-version>`. Extract the old and new versions and classify the bump:

| Bump Type | Risk Level | Criteria |
|-----------|------------|----------|
| **Patch** (x.x.OLD → x.x.NEW) | Low | Only the patch segment changed |
| **Minor** (x.OLD.x → x.NEW.x) | Medium | The minor segment changed |
| **Major** (OLD.x.x → NEW.x.x) | High | The major segment changed |

### 2b. Check CI / Check Status

Use `mcp__github__get_pull_request_status` to retrieve the CI check status for each PR. A PR is considered CI-passing only if all checks have concluded with a success state.

### 2c. Inspect the Diff

Use `mcp__github__get_pull_request_files` to review the files changed. Flag any PR that modifies unexpected files beyond dependency manifests (`go.mod`, `go.sum`, `package.json`, `yarn.lock`, `Dockerfile*`, `.github/workflows/*`).

### 2d. Identify Dependency Scope

- **Development-only** (test frameworks, linters, dev tools) → Lower risk
- **Production** (runtime dependencies) → Higher risk
- **GitHub Actions** (workflow dependencies) → Typically low risk for minor/patch bumps

### 2e. Final Risk Matrix

| Version Bump | CI Passing | Only Manifest Files | Final Risk | Action |
|-------------|------------|---------------------|------------|--------|
| Patch | Yes | Yes | **Low** | Auto-approve and merge |
| Patch | No | Yes | **Medium** | Approve but do not merge |
| Minor | Yes | Yes | **Medium** | Auto-approve and merge |
| Minor | Yes | No | **High** | Do not approve |
| Minor | No | * | **High** | Do not approve |
| Major | * | * | **High** | Do not approve |

GitHub Actions patch and minor bumps with passing CI → **Low** risk.

## Step 3: Approve Eligible PRs

Use `mcp__github__create_pull_request_review` with `event: APPROVE` for eligible PRs.

## Step 4: Merge Approved PRs

Use `mcp__github__merge_pull_request` with `merge_method: squash`. If the merge fails, note the failure and continue.

## Step 5: Report Results

After processing all PRs, produce a summary table showing merged, approved-pending, flagged, and errored PRs.

## Important Notes

- Never force-merge.
- Respect branch protection rules.
- Go module PRs may need `go mod tidy` after merge.
- Process oldest-first to avoid dependency tree conflicts.
- Security-labeled PRs should be prioritized; treat security patch/minor bumps as Low risk if CI passes.
