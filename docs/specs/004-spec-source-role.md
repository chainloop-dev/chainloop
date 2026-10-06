---
owner: jiparis
ticket: https://github.com/chainloop-dev/chainloop/issues/3531
prd:
---

# Spec 004: Role of each spec source

## Summary
This spec changes the spec capture of [Spec 002](002-session-spec-capture.md) and [Spec 003](003-spec-capture-during-session.md). It does not replace them. Today each spec source has a kind: ticket, document, image, or text. The kind tells the format of a source, but not its purpose. With this change, the agent can also record a role for each source: `task`, `spec`, `plan`, or `reference`. The CLI stores the role in the session material and in the annotations of the spec material. A tool that shows the sources can then group them by purpose.

## Problem
- A reader of the evidence cannot tell a spec from a background document. Both have the kind `document`.
- A reader cannot tell an approved plan from other text. Both have the kind `text`.
- A ticket can hold the task, and it can also be only background for the work. The kind `ticket` does not tell which.
- A tool that shows the spec sources must guess the purpose from the kind. The guess is wrong for some sources.
- An image or another binary file has no header, so the agent cannot record anything about it.

## Goals and Non-Goals
- Goal: each spec source can carry its purpose, as the agent decides it.
- Goal: the evidence holds the role next to the kind, in the spec entry and in the material annotations.
- Goal: the agent can give a role to a binary file too.
- Goal: sessions without roles stay valid, and earlier consumers keep working.
- Non-goal: a change to the kinds, or to how the CLI finds the kind.
- Non-goal: rules in the CLI that guess a role. The agent decides the role, as it decides what a spec is (D-001 of Spec 002).
- Non-goal: a source address or other metadata for binary files. Only the role is new.
- Non-goal: a change to the reminder at each user turn of Spec 003.

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
For an image or another binary file, the agent MAY write a companion file. Its name is the name of the binary file plus `.meta.yaml`. It holds a `role` field. The CLI MUST apply that role to the binary file. The CLI MUST NOT store the companion file as a spec material of its own. The CLI MUST ignore a companion file with no binary file next to it.
- Done when: the agent copies a screenshot and writes its companion file with `role: reference`. The push records one image material with the role `reference`, and no material for the companion file.

### R-004: Role in the evidence
When a source has a role, the system MUST record it in the spec entry of the session material. It MUST also record it in a `chainloop.spec.role` annotation on the spec material. When a source has no role, the system MUST leave out both.

### R-005: Missing or unknown role
A role that is missing, empty, or not in the vocabulary MUST give no role. The system MUST NOT guess a role from the kind. A bad role MUST NOT stop the push.
- Done when: a header with `role: design` pushes a spec material with no role annotation, and the push succeeds.

### R-006: Capture instruction
The capture instruction at session start SHOULD ask the agent to set the role of each source, with one short line for each role. It SHOULD tell the agent how to give a role to a binary file.

## Constraints
- The repository is public. The instruction text and the format are visible to all users.
- The change to the evidence is additive. A consumer that does not know the role MUST still read the spec entries and materials.
- Only known header fields reach the evidence. A new field does not open the header to other keys.
- The instruction at session start goes into the agent context. It must stay short.

## Proposal
The user does nothing new. At session start, the capture instruction lists the four roles. The agent adds a role to the header of each text file that it writes:

```markdown
---
kind: ticket
uri: https://tracker.example.com/issue/ENG-1234
role: task
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
```

At push time, the CLI reads the role as it reads the kind and the source address. It normalizes the value, and it drops a value that is not in the vocabulary. It applies the role of a companion file to its binary file, and it does not upload the companion file. The spec entry and the material annotations then hold the role:

```json
"spec": [
  { "kind": "ticket", "role": "task", "uri": "https://tracker.example.com/issue/ENG-1234",
    "digest": "sha256:e4c2...", "captured_at": "2026-10-06T10:12:03Z" },
  { "kind": "image", "role": "reference",
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
    "chainloop.spec.role": "reference"
  }
}
```

A tool that shows the sources groups them by role when a source has one. For a source with no role, the tool uses its own rule from the kind. The evidence holds only what the agent stated.

## Decision Record

| ID | Decision | Choice | Why (and what we rejected) | Source |
|----|----------|--------|----------------------------|--------|
| D-001 | How to record the purpose | A role field next to the kind | The kind is the format and the role is the purpose. A ticket can be the task or only background. Rejected: more kinds, for example a `plan` kind (it mixes format and purpose, and a document can be a spec or a reference). | drafting |
| D-002 | Role values | `task`, `spec`, `plan`, `reference` | Four values cover the purposes that a reviewer looks for: what to do, what to build, how, and supporting material. Rejected: only `spec`, with no role for all other sources (a reader cannot tell a task from a reference). | drafting |
| D-003 | Role of a binary file | A companion header file that the agent writes | The agent decides the role for every source, also for an image. Rejected: the CLI sets `reference` for each image (a guess, and a mockup can be the spec). Rejected: no role for binary files. | drafting |
| D-004 | Missing or unknown role | No role in the evidence | The evidence holds only what the agent stated. Consumers can apply their own rule. Rejected: a default role from the kind (a guess looks like a fact in the evidence). | drafting |
| D-005 | Instruction strength | SHOULD, in the session-start instruction only | A role is useful but not necessary, and the reminder at each turn must stay short (Spec 003). Rejected: MUST, and the roles in the reminder at each turn (more tokens on each turn). | drafting |
| D-006 | Name of the companion file | The binary file name plus `.meta.yaml`, for example `export-mockup.png.meta.yaml` | The suffix tells that the file holds metadata, not a spec. Rejected: the binary file name plus `.md`, with a header and an empty body (a reader can take it for an empty spec). | drafting |
| D-007 | Companion file with no binary file | The CLI ignores it, with no warning | The file holds no spec content, so the push loses nothing. Rejected: a warning in the session material (noise for a case with no loss). | drafting |

## Open Questions
None.

## Risks

| Risk | Mitigation |
|------|------------|
| The agent gives a wrong role, for example `spec` for a background document. | The role is the agent's statement, as the kind is. The kind and the content stay in the evidence, so a reviewer can check. |
| The agent does not set a role. | The role is optional (R-005). Consumers use their own rule from the kind. The share of sources with a role is measurable from the evidence. |
| The agent forgets the companion file for an image. | The image has no role, and consumers use their own rule. |
| A source has more than one purpose. For example, a ticket also holds the full spec. | The agent records the main purpose. The instruction says to pick the role that tells why the source is in the session. |
