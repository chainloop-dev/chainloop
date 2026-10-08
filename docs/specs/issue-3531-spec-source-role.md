---
owner: jiparis
ticket: https://github.com/chainloop-dev/chainloop/issues/3531
prd:
---

<!--
AGENT INSTRUCTIONS. Keep this comment. Read it before you change this file.

This spec records a design that humans reviewed and approved. Do not change it as part of other work. Find the status of the parent ticket (the `ticket:` field) before you change anything. If you cannot find the status, or you are not sure which case below applies, do not change the spec. Ask the human first.

- Ticket in design, and the spec PR is open: change the spec only through the br:eng-spec skill (`--revise`). Never renumber or reuse an R-xxx or D-xxx ID.
- Ticket in progress: do not rewrite the requirements, the Proposal, or existing Decision Record rows. Record a changed decision as a new Decision Record row. The owner records the differences with the shipped work through br:eng-spec (`--done`).
- Ticket done or canceled: the spec is frozen. Do not change, delete, or strike through any text. The only allowed edits are typo fixes, broken links, and the "Superseded by" line under the title. For a design change, write a new spec with br:eng-spec that supersedes this one.

When your code goes against this spec, say so in your PR. Do not edit the spec to match the code.
-->

# Spec issue-3531: Role, title, and description of each spec source

## Summary
This spec changes the spec capture of [Spec issue-3495](issue-3495-session-spec-capture.md) and [Spec issue-3515](issue-3515-spec-capture-during-session.md). It does not replace them. Today each spec source has a kind: ticket, document, image, or text. The kind tells the format of a source, but not its purpose. A source also has no name that a person can read. With this change, the agent can also record a role, a title, and a description for each source. The role is `task`, `spec`, `plan`, or `reference`. The title is a short name, for example the title of a ticket. The description says in one or two sentences what the source holds. The CLI stores these values in the session material and in the annotations of the spec material. A tool that shows the sources can then group them by purpose, label them by title, and explain each one.

## Problem
- A reader of the evidence cannot tell a spec from a background document. Both have the kind `document`.
- A reader cannot tell an approved plan from other text. Both have the kind `text`.
- A ticket can hold the task, and it can also be only background for the work. The kind `ticket` does not tell which.
- A tool that shows the spec sources must guess the purpose from the kind. The guess is wrong for some sources.
- A source has no name that a person can read. A tool must use the material name, which the CLI makes from the file name. The CLI shortens it and changes it to lower case, for example `ticket-eng-1234-comment-rca`.
- An image or another binary file has no header, so the agent cannot record anything about it.

## Goals and Non-Goals
- Goal: each spec source can carry its purpose, a short title, and a short description, as the agent decides them.
- Goal: the evidence holds these values next to the kind, in the spec entry and in the material annotations.
- Goal: the agent can give these values to a binary file too.
- Goal: sessions without these values stay valid, and earlier consumers keep working.
- Non-goal: a change to the kinds, or to how the CLI finds the kind.
- Non-goal: rules in the CLI that guess a role, a title, or a description. The agent decides them, as it decides what a spec is (D-001 of Spec issue-3495).
- Non-goal: a source address or other metadata for binary files. Only the role, the title, and the description are new.
- Non-goal: a change to the reminder at each user turn of Spec issue-3515.

## Requirements

### R-001: Role vocabulary
A spec source MAY have one role. The roles are:
- `task`: the item that states the work to do, for example the ticket that the session works on.
- `spec`: a document that defines what to build, for example requirements or a design.
- `plan`: a plan for the work that the user approved in the session.
- `reference`: supporting material, for example a screenshot, a mockup, an example, or a background document.

### R-002: Role in the header
The header of a text spec file MAY hold a `role` field. The CLI MUST read it with no regard to letter case.

### R-003: Role of a binary file
For an image or another binary file, the agent MAY write a companion file. Its name is the name of the binary file plus `.meta.yaml`. It holds the fields `role`, `title`, and `description`. The CLI MUST apply those values to the binary file. The CLI MUST NOT store the companion file as a spec material of its own. The CLI MUST ignore a companion file with no binary file next to it.
- Done when: the agent copies a screenshot and writes its companion file with `role: reference`. The push records one image material with the role `reference`, and no material for the companion file.

### R-004: Role in the evidence
When a source has a role, the system MUST record it in the spec entry of the session material. It MUST also record it in a `chainloop.spec.role` annotation on the spec material. When a source has no role, the system MUST leave out both.

### R-005: Missing or unknown role
A role that is missing, empty, or not in the vocabulary MUST give no role. The system MUST NOT guess a role from the kind. A bad role MUST NOT stop the push.
- Done when: a header with `role: design` pushes a spec material with no role annotation, and the push succeeds.

### R-006: Capture instruction
The capture instruction at session start SHOULD ask the agent to set the role, the title, and the description of each source. It SHOULD give one short line for each role. It SHOULD tell the agent how to give these values to a binary file.

### R-007: Title
The header of a text spec file, and the companion file of a binary file, MAY hold a `title` field. When a source has a title, the system MUST record it in the spec entry of the session material. It MUST also record it in a `chainloop.spec.title` annotation on the spec material. When the title is missing or empty, the system MUST leave out both. The system MUST NOT make a title from the content or from the file name.
- Done when: a ticket file with `title: "ENG-1234: Add an export button"` pushes a spec entry and a material annotation with that title.

### R-008: Title limits
The system MUST cut a title that is longer than 120 characters, and keep the first 120. Each title MUST pass the secret redaction of Spec issue-3495 before it goes into any evidence field. For a text file, the system MUST read the title from the redacted file, as it reads the source address. For a binary file, the system MUST redact the title of the companion file. A bad title MUST NOT stop the push.

### R-009: Description
The header of a text spec file, and the companion file of a binary file, MAY hold a `description` field. It says in one or two sentences what the source holds, or why it is in the session. When a source has a description, the system MUST record it in the spec entry of the session material. It MUST also record it in a `chainloop.spec.description` annotation on the spec material. When the description is missing or empty, the system MUST leave out both. The system MUST NOT make a description from the content. The rules of R-008 apply, with a limit of 300 characters.

## Constraints
- The repository is public. The instruction text and the format are visible to all users.
- The change to the evidence is additive. A consumer that does not know the role or the title MUST still read the spec entries and materials.
- The title is text that an agent wrote. It can hold a secret, as the spec text can.
- Only known header fields reach the evidence. A new field does not open the header to other keys.
- The instruction at session start goes into the agent context. It must stay short.

## Proposal
The user does nothing new. At session start, the capture instruction lists the four roles and asks for a short title. The agent adds a role and a title to the header of each text file that it writes:

```markdown
---
kind: ticket
uri: https://tracker.example.com/issue/ENG-1234
role: task
title: "ENG-1234: Add an export button"
description: The ticket that the session implements. It asks for a CSV export on the report page.
---

# ENG-1234: Add an export button
...
```

For a binary file, the agent writes a companion file next to it, with the suffix `.meta.yaml`:

```text
spec folder
├── ticket-eng-1234.md            kind: ticket, role: task
├── design-proposal.md            kind: document, role: spec
├── approved-plan.md              kind: text, role: plan
├── export-mockup.png             image, stored as it is
└── export-mockup.png.meta.yaml   role: reference
                                  title: Export button mockup
                                  description: Where the button goes.
```

At push time, the CLI reads the role and the title as it reads the kind and the source address. It normalizes the role, and it drops a role that is not in the vocabulary. It cuts a long title. It applies the values of a companion file to its binary file, after it redacts the title, and it does not upload the companion file. The spec entry and the material annotations then hold the role and the title:

```json
"spec": [
  { "kind": "ticket", "role": "task", "title": "ENG-1234: Add an export button",
    "description": "The ticket that the session implements. It asks for a CSV export on the report page.",
    "uri": "https://tracker.example.com/issue/ENG-1234",
    "digest": "sha256:e4c2...", "captured_at": "2026-10-06T10:12:03Z" },
  { "kind": "image", "role": "reference", "title": "Export button mockup",
    "description": "Where the button goes.",
    "digest": "sha256:c2a1...", "captured_at": "2026-10-06T10:31:40Z" }
]
```

```json
{
  "name": "export-mockup.png",
  "annotations": {
    "chainloop.material.name": "spec-fd4e67-export-mockup",
    "chainloop.material.type": "EVIDENCE",
    "chainloop.spec.session_id": "fd4e6754-3b26-4f54-9807-13c58465bb35",
    "chainloop.spec.kind": "image",
    "chainloop.spec.role": "reference",
    "chainloop.spec.title": "Export button mockup",
    "chainloop.spec.description": "Where the button goes."
  }
}
```

A tool that shows the sources groups them by role when a source has one, and labels them by title. For a source with no role, the tool uses its own rule from the kind. For a source with no title, it uses the material name. The evidence holds only what the agent stated.

## Decision Record

| ID | Decision | Choice | Why (and what we rejected) | Source |
|----|----------|--------|----------------------------|--------|
| D-001 | How to record the purpose | A role field next to the kind | The kind is the format and the role is the purpose. A ticket can be the task or only background. Rejected: more kinds, for example a `plan` kind (it mixes format and purpose, and a document can be a spec or a reference). | drafting |
| D-002 | Role values | `task`, `spec`, `plan`, `reference` | Four values cover the purposes that a reviewer looks for: what to do, what to build, how, and supporting material. Rejected: only `spec`, with no role for all other sources (a reader cannot tell a task from a reference). | drafting |
| D-003 | Role of a binary file | A companion header file that the agent writes | The agent decides the role for every source, also for an image. Rejected: the CLI sets `reference` for each image (a guess, and a mockup can be the spec). Rejected: no role for binary files. | drafting |
| D-004 | Missing or unknown role | No role in the evidence | The evidence holds only what the agent stated. Consumers can apply their own rule. Rejected: a default role from the kind (a guess looks like a fact in the evidence). | drafting |
| D-005 | Instruction strength | SHOULD, in the session-start instruction only | A role is useful but not necessary, and the reminder at each turn must stay short (Spec issue-3515). Rejected: MUST, and the roles in the reminder at each turn (more tokens on each turn). | drafting |
| D-006 | Name of the companion file | The binary file name plus `.meta.yaml`, for example `export-mockup.png.meta.yaml` | The suffix tells that the file holds metadata, not a spec. Rejected: the binary file name plus `.md`, with a header and an empty body (a reader can take it for an empty spec). | drafting |
| D-007 | Companion file with no binary file | The CLI ignores it, with no warning | The file holds no spec content, so the push loses nothing. Rejected: a warning in the session material (noise for a case with no loss). | drafting |
| D-008 | Name of a source | An optional title that the agent writes, next to the role | The agent knows the title of the source when it writes the file. The format already changes for the role, so a second field costs little. A tool can label a source from the session material alone. Rejected: the material name only (a short lower-case file stem). Rejected: a title that the CLI takes from the first heading (a guess that looks like a fact in the evidence). This changes the outcome of [#3528](https://github.com/chainloop-dev/chainloop/issues/3528), which closed a title field before the format had other changes. | drafting |
| D-009 | Where the title goes | The spec entry and a `chainloop.spec.title` annotation, as for the role | A tool that reads only one material can label it. Rejected: the spec entry only. | drafting |
| D-010 | Long title | Cut at 120 characters, and keep the first part | A model sometimes writes a full paragraph. A cut keeps a usable label. Rejected: drop a long title (the source loses its label). Rejected: no limit (long annotations and labels). | drafting |
| D-011 | Description of a source | An optional description of one or two sentences, with the rules of the title and a limit of 300 characters | A reviewer sees why a source is in the session before opening it. Rejected: no description, because the content is the description (a reviewer must open each source to know what it is). | [PR comment](https://github.com/chainloop-dev/chainloop/pull/3532#discussion_r4195022169) |

## Open Questions
None.

## Risks

| Risk | Mitigation |
|------|------------|
| The agent gives a wrong role, for example `spec` for a background document. | The role is the agent's statement, as the kind is. The kind and the content stay in the evidence, so a reviewer can check. |
| The agent does not set a role. | The role is optional (R-005). Consumers use their own rule from the kind. The share of sources with a role is measurable from the evidence. |
| The agent forgets the companion file for an image. | The image has no role, and consumers use their own rule. |
| A title holds a secret, for example a token in a pasted ticket title. | Each title passes the redaction before it goes into the evidence (R-008). |
| A source has more than one purpose. For example, a ticket also holds the full spec. | The agent records the main purpose. The instruction says to pick the role that tells why the source is in the session. |
