---
owner: migmartri
ticket: https://github.com/chainloop-dev/chainloop/issues/3829
spec: docs/specs/issue-3829-session-human-touch-points.md
---

<!--
AGENT INSTRUCTIONS. Keep this comment. Read it before you change this file.

This plan says how to build the spec in the `spec:` field, or the ticket in the `ticket:` field when there is no spec. The spec holds the design and its reasons. This plan holds only the steps.

When you build a milestone, build only its steps. Each milestone merges by itself, as one PR. Cite the milestone (M1) and its requirements (R-001) in the PR. Before you add a step, a file, a dependency, an option, or a mechanism that this plan does not name, stop and ask the human. A vague word such as "secure" means the baseline the product already has for the same need, not a new mechanism.

When the code shows that a step does not work, stop and tell the human. Change this plan only after the human agrees. A change to a requirement or a decision goes in the spec, through br:eng-spec.

After each update to this plan, automatically apply br:eng-ste100 to the prose you changed, and run its linter until it reports no hard violations. Then run br:eng-plan (`--validate`) on the latest text. Do both before you commit or push. Do not wait for the human to request validation. This applies to every update, including removals and clarifications. An earlier validation does not cover later edits.

Do not answer an open question of this plan by yourself. Propose an answer to the human.
-->

# Plan issue-3829: Record the human touch points of an AI coding session

## Overview

This plan covers milestone 1 of the spec: the evidence section and Claude Code. The Claude Code parser already counts the messages of a session at push time. It will also count the human touch points and the permission modes, and write them into a new human interaction section of the session evidence. A new hook records each permission request in the Trace state, as the skill hooks record skill loads. The push reads these records and gives them to the parser.

## Architecture and impact

```mermaid
flowchart LR
    A[Agent] -->|transcript| C[Transcript copy]
    A -->|permission request hook, Claude Code only| S[Trace state]
    C --> P[Provider parser at push]
    S -->|permission requests of the session| P
    P -->|human interaction section| E[Session evidence]
    E --> V[Schema validation]
```

The components stay the same. The change adds one hook path for Claude Code and one new output of the Claude Code parser.

- Session evidence format: the new section, its schema, and its redaction rule.
- Claude Code hooks: the new permission request hook and its state record.
- Push assembly: reads the permission requests from the state and passes them to the parser.
- Claude Code parser: counts the touch points and the permission modes.
- Trace README: the new rows of the compatibility matrix.

| Impact on | What changes |
|-----------|--------------|
| Users and other parts of the product | The session evidence gets a new section. The existing conversation counts do not change. The platform can ignore the section until a later change reads it. |
| Data and migrations | None. The evidence is a material, not a table. |
| Configuration and deployment | `trace init` installs one more Claude Code hook. A repository that ran `trace init` before gets it on the next `trace init`. Until then, its evidence has no permission counts. |
| Compatibility | The section is additive and the schema version stays the same. An old control plane rejects evidence with the new section, because the root of the schema allows no unknown fields. The control plane validates the evidence, so it must ship before or with the CLI. |
| Rollback | Revert the CLI. Evidence that is already stored keeps the section, and the schema still accepts it. |

## Milestones

| # | Milestone | Requirements | Depends on |
|---|-----------|--------------|------------|
| M1 | Evidence section and Claude Code | R-001 to R-009 | none |

### M1: Evidence section and Claude Code

The session evidence gets the human interaction section, and its schema accepts it. The Claude Code parser fills all the counts and the permission modes, from the transcript of the session and its subagents. A new permission request hook records each request in the Trace state, with the pattern of the skill hooks (rung 2). The README matrix gets the new rows, with the cells for Claude Code set and the other cells at **No** until their milestone.

**Done when:** a pushed Claude Code session holds the section with all counts and its permission modes. An old session without the section still passes schema validation. The parser tests with new transcript fixtures and the schema validator tests prove both.

**Steps**, in build order. Open a step to see what changes and the files it touches.

<details>
<summary><strong>01 / 05 · The session evidence can hold the human interaction section (R-007, R-009)</strong></summary>

Add the section to the evidence data with the fields of the spec Proposal table. Each count is a pointer, and the list of permission modes is a nil slice when not recorded. So a zero stays in the JSON and a missing count stays out (R-007). The JSON schema gets the new optional property, with the same open string rule as `session.mode` for the mode names. The secret redaction keeps the mode names, as it keeps `session.mode`.

| File | Change |
|------|--------|
| `pkg/attestation/crafter/materials/aicodingsession/aicodingsession.go` | Add `HumanInteraction` with `PermissionModes []string` and a `TouchPoints` struct of `*int` fields. Add `HumanInteraction *HumanInteraction` to `Data`. |
| `pkg/attestation/crafter/materials/aicodingsession/aicodingsession_test.go` | Add a case: a zero count stays in the JSON output, and a nil count stays out. |
| `internal/schemavalidators/internal_schemas/aicodingsession/ai-coding-session-0.1.schema.json` | Add the optional `human_interaction` property. State in its description that the mode names are native to each agent. |
| `internal/schemavalidators/schemavalidators_test.go` | Add cases: evidence with the section validates, and evidence without it validates. |
| `pkg/attestation/crafter/materials/aicodingsession/redact.go` | Add `/data/human_interaction/permission_modes/*` to the protected paths. |
| `pkg/attestation/crafter/materials/aicodingsession/redact_test.go` | Add a case: a mode name that looks like a secret stays. |

</details>

<details>
<summary><strong>02 / 05 · Claude Code sessions record prompts, questions, plans, interrupts, and permission modes (R-001, R-002, R-004, R-005, R-006, R-008)</strong></summary>

The parser reads each record of the main transcript and of each subagent transcript. Prompts come from the main transcript only, because the first message of a subagent comes from the parent agent. A prompt is a record that the existing `isUserPrompt` function accepts, with no interrupt marker and no injected marker. A question is an `AskUserQuestion` tool result with an answer. A plan is an `ExitPlanMode` tool result: approved when it is not a rejection. An interrupt is the interrupt marker text. The interrupt marker that follows a rejection at a plan or permission dialog is not an interrupt (R-008). The permission modes come from the `permissionMode` field of the user records, in the order of first use. A record that the parser cannot read gives a warning, and the parser leaves out the counts that it affects (R-009).

| Rule input | Count |
|------------|-------|
| User record of the main transcript that `isUserPrompt` accepts | prompts |
| User record with an injected marker (command output, task notification, compact summary, command caveat) | none |
| `AskUserQuestion` result with an answer | questions_answered |
| `ExitPlanMode` result, not rejected | plans_approved |
| `ExitPlanMode` result, rejected | plans_rejected |
| Interrupt marker | interrupts |

| File | Change |
|------|--------|
| `app/cli/internal/trace/claude/interaction.go` | New: count the touch points and the permission modes of one transcript stream. Reuse the raw-entry helpers of `pastedimages.go`. |
| `app/cli/internal/trace/claude/parse.go` | Run the counter on the main stream and on each subagent stream, add the results, and set `HumanInteraction`. |
| `app/cli/internal/trace/claude/testdata/interaction-session.jsonl` | New fixture, cut from a real interactive session, with each touch point type and a mode change. |
| `app/cli/internal/trace/claude/interaction_test.go` | Table tests for each row of the rule table and for the mode order. Also: the subagent sum has no subagent prompts, an Esc at a plan dialog, and a record that cannot be read. |

</details>

<details>
<summary><strong>03 / 05 · Claude Code records each permission request that it shows (R-003)</strong></summary>

`trace init` installs a `PermissionRequest` hook. The handler appends the tool name and the tool call ID of the request to a state file of the session. It writes nothing to stdout, so the hook makes no permission decision. The handler never fails the agent. It skips the question and plan tools (R-008).

| File | Change |
|------|--------|
| `app/cli/internal/trace/claude/hooks.go` | Add the `PermissionRequest` event to `hookEvents`, with the `permission-request` command. |
| `app/cli/cmd/trace_hook.go` | Add the `claude permission-request` subcommand. |
| `app/cli/pkg/action/trace_agent_hook.go` | Add `HandleAgentPermissionRequest`: read the input, skip `AskUserQuestion` and `ExitPlanMode`, append the record. |
| `app/cli/internal/trace/state/permissions.go` | New: append and read the permission request records of a session. |
| `app/cli/internal/trace/claude/hooks_test.go`, `app/cli/cmd/trace_hook_test.go`, `app/cli/internal/trace/state/permissions_test.go` | Add tests: the hook installs and uninstalls, the command exists, records round-trip. |
| `app/cli/pkg/action/trace_agent_hook_test.go` | Add tests: the handler writes nothing to stdout, and it exits with success on input that it cannot read. |

</details>

<details>
<summary><strong>04 / 05 · Claude Code sessions record permission approvals and rejections (R-003, R-007, R-009)</strong></summary>

At push, the assembly reads the permission requests of the session and passes them to the parser in `ParseOpts`. For each request, the parser finds the tool result of the same call. A rejection marker gives a rejection. A completed result gives an approval. No result gives neither. A session with no requests is normal, for example in an autonomous mode. So the assembly also tells the parser whether the repository has the permission request hook installed. With the hook and no requests, both counts are zero. Without the hook, the parser leaves out both counts and adds a warning (R-007).

| File | Change |
|------|--------|
| `app/cli/internal/trace/provider.go` | Add `PermissionRequests` and `PermissionHookInstalled` to `ParseOpts`. |
| `app/cli/internal/trace/claude/hooks.go` | Add a function that tells if the permission request hook is installed, next to `HooksInstalled`. |
| `app/cli/pkg/action/trace_hook_handler.go` | In `buildSessionEvidence`, read the requests from the state, call that function, and set both results in `ParseOpts`. |
| `app/cli/internal/trace/claude/interaction.go` | Match each request to its tool result and count approvals and rejections. |
| `app/cli/internal/trace/claude/interaction_test.go` | Add cases: approved, rejected with feedback text, Esc at a permission dialog, unanswered, no requests with the hook, no hook. |

</details>

<details>
<summary><strong>05 / 05 · The README matrix shows which providers record the touch points (R-009)</strong></summary>

Add one row for each touch point type and one for the permission modes to the Session collection table. Set the Claude Code cells to **Yes**. Set the other cells to **No**, with the "Why not Yes" line "Not implemented yet (spec issue-3829, milestone 2)". The e2e suite reports these rows as "no check" (TRC-026). That result is correct, because the rows have no check.

| File | Change |
|------|--------|
| `app/cli/internal/trace/README.md` | Add the rows and the "Why not Yes" lines. The existing `TestTraceREADMEParses` test checks that the table still parses. |

</details>

Verify: `go test ./pkg/attestation/crafter/materials/aicodingsession/... ./internal/schemavalidators/... ./app/cli/internal/trace/... ./app/cli/pkg/action/... ./app/cli/cmd/...`

## Open Questions

- [ ] **Does the Claude Code permission request hook carry the tool call ID?** Proposed: find out in step 03. Without an ID, match each request to the next result of the same tool.

## Skipped and risks

| Skipped step | Reason | Follow-up |
|--------------|--------|-----------|
| End-to-end checks for the new README rows | The owner cut them from the spec. The rows report "no check". | none |
| A change to the existing user message count | The spec keeps its meaning (D-001). | none |
| A view in the platform UI | A Non-Goal of the spec. | platform ticket |
| Milestone 2 of the spec: Codex, Copilot, Cursor, OpenCode, and Pi | The owner limited this plan to Claude Code. | One plan for each provider, written when the team starts it |

Not checked: some Claude Code field and marker names. No fixture in the repository holds `permissionMode`, the question and plan tool results, the rejection and interrupt markers, or the permission request hook payload. Steps 02 and 03 start with a real interactive session for that reason.

| Risk | Mitigation |
|------|------------|
| An old control plane rejects evidence with the new section. | Ship the control plane schema change before or with the CLI release (see Compatibility). |
| A repository with old hooks has no permission request records. | The parser leaves out the approval and rejection counts and adds a warning (M1 step 04). |
| The permission request hook writes output that Claude Code reads as a decision. | The handler writes nothing to stdout. A test in step 03 checks it. |
