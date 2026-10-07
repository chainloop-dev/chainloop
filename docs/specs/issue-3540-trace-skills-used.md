---
owner: jiparis
ticket: https://github.com/chainloop-dev/chainloop/issues/3540
prd:
---

# Spec issue-3540: Skills used in a Chainloop Trace session

## Summary

Chainloop Trace will record the skills that an agent used in a session, and it will put each full skill into the attestation. Each used skill becomes a spec entry of the new kind `skill`. The CLI copies the skill folder when the session uses the skill. That copy includes `SKILL.md` and any scripts or other files. At push time, the CLI uploads the copy as one archive, as an EVIDENCE material, the same as other spec sources. Spec entries get a new optional `metadata` object. Its fields depend on the kind. For a skill, it holds the skill source and the use counts.

This spec changes the spec capture of [Spec issue-3495](issue-3495-session-spec-capture.md) and [Spec issue-3531](issue-3531-spec-source-role.md). It does not replace them. It departs from D-001 of Spec issue-3495 for skills only. The CLI makes the skill entries. The agent does not write them. The first version supports Claude Code and OpenCode.

## Problem

- The evidence shows which skills are available to an agent, but not which skills the agent used. The `ai-agent-config` material lists the skill files in the repository. That list does not show use.
- The `ai-agent-config` material holds only `SKILL.md` files in the repository. It has no user or organization skills, and no scripts that a skill runs.
- The tool usage summary counts calls per tool name. All Claude Code skill calls go into one `Skill` entry, so the summary does not show which skills ran.
- A skill sets how the agent works, as a spec sets what the agent builds. A reviewer cannot see the skill, or the code that it runs, next to the other sources of the session.
- A user can start a skill with a slash command (for example `/graphify`). That skill use is not a tool call, so a scan of tool calls alone does not find it.

## Goals and Non-Goals

- Goal: A policy or the UI can find the skills that a session used in the spec entries of the session material.
- Goal: The attestation holds each used skill in full: its definition and its files, at all skill levels.
- Goal: The attestation does not need the `ai-agent-config` material to show a used skill.
- Goal: The record separates skills that the model started from skills that the user started, and includes skills that subagents used.
- Goal: The `metadata` object can hold kind-specific data for other kinds later, with no new top-level field.
- Non-goal: Record the arguments of each skill call. The raw transcript already holds them.
- Non-goal: Support Cursor and Pi in the first version. Their transcript formats for skill use are not known yet. A later change can add them without a schema change.
- Non-goal: Record the local path of a skill folder.
- Non-goal: Change how the agent captures other spec sources, the tool usage summary, the raw transcript, or the `ai-agent-config` material.

## Requirements

### R-001: One spec entry for each used skill

The session material MUST hold one spec entry of kind `skill` for each skill that the session used. The title MUST be the skill name. The CLI MUST NOT set a role, a description, or a source address for a skill entry. The capture time MUST be the time of the first use in the transcript. A session that used no skill MUST have no skill entries.

- Done when: sessions with no skill, one skill, and several skills each produce the expected spec entries. Two runs over the same transcript and the same copies produce the same output.

### R-002: Copy at use time

The CLI MUST copy the skill folder at the first hook event after the first use of the skill in the session. When no hook event comes before the push, the CLI MUST copy the folder at push time and record a warning. The copy MUST stay in the Trace state until the session ends. When a later use finds different folder content, the CLI MUST keep the first copy and record a warning.

- Done when: a skill file that changes after the use and before the push does not change the uploaded package.

### R-003: Skill package material

Each skill entry MUST point by digest to one EVIDENCE material that holds the skill package. The package MUST be a reproducible archive of the skill folder: sorted entries, fixed timestamps, and no owner data. The same folder content MUST give the same digest in every session. Before it makes the archive, the CLI MUST redact each text file. It MUST use the secret redaction of Spec issue-3495 (R-007). The material MUST carry the same annotations as other spec materials, with `chainloop.spec.kind` set to `skill` and `chainloop.spec.title` set to the skill name.

- Done when: the attestation holds one archive for each used skill, and the archive holds `SKILL.md` and the other files of the folder, redacted.

### R-004: Package limits

The CLI MUST read only a folder that holds a `SKILL.md` file. It MUST NOT follow a link that points out of the skill folder. It MUST leave out version-control folders. When a package is larger than 5 MB, the CLI MUST NOT upload it. It MUST keep the spec entry with no digest and record a warning.

- Done when: fixtures cover a link out of the folder, a `.git` folder, and a package over 5 MB. Each gives the expected package or warning.

### R-005: Metadata object

A spec entry MAY hold a `metadata` object. The fields of the object depend on the kind of the entry. This spec defines the fields for the kind `skill` only. Entries of other kinds MUST leave the object out.

### R-006: Use counts and who started the skill

The metadata of a skill entry MUST hold the total number of uses. It MUST also hold the number of uses that the model started and the number that the user started. The two start counts MUST add up to the total.

- A model start is a skill tool call that loaded a skill. A failed call MUST NOT count.
- A user start is a slash command that loads a skill. A slash command that does not load a skill MUST NOT count. Examples are `/clear`, `/model`, and custom commands that are not skills.
- One skill use MUST count one time only, also when the transcript records it in more than one entry.
- Done when: fixtures cover a model start, a user start, both starts for one skill, a failed call, and built-in commands. Each gives the expected counts.

### R-007: Subagent skills

Skills that subagents used MUST count in the totals of the same entry. The metadata MUST also hold the number of uses that came from subagents.

### R-008: Skill source

The metadata of a skill entry MUST hold a source, with one of these values:

- `project`: a skill from the repository.
- `user`: a skill from the user's agent configuration.
- `organization`: a skill that an administrator installs for all users.
- `plugin`: a skill from an installed plugin.
- `unknown`: all other cases.
 The evidence MUST NOT hold the skill folder path. When the data is not sufficient, the source MUST be `unknown`.

- Done when: fixtures for each source value produce the expected source.

### R-009: Limit on skill entries

Skill entries MUST have their own limit of 25 for each session. They MUST NOT count against the limit of the spec sources that the agent writes. When a session used more skills than the limit, the CLI MUST keep the skills that the session used first and record a warning.

### R-010: Providers, compatibility, and failure safety

Claude Code and OpenCode MUST produce skill entries. Cursor and Pi MUST produce none until a later change adds support. For those providers, no skill entry MUST mean "not recorded". The new kind and the `metadata` object MUST be additive, and the schema version MUST stay the same. A skill that the CLI cannot read or copy MUST give a warning, and it MUST NOT stop evidence creation or block the Git push.

- Done when: old evidence and new evidence both pass schema validation. A missing skill folder produces a warning and the rest of the evidence.

## Constraints

- The repository is public. The evidence format is visible to all users.
- User and organization skills can hold private text. The secret redaction applies to all text files of a package before upload.
- A hook must stay fast. The copy at use time must not delay the agent for a normal skill folder.
- Output ordering and archives must be deterministic, so the same input gives the same digests.

## Proposal

The trace hooks already run on agent events. They will also find new skill uses and copy each skill folder into the Trace state. At push time, the provider parser counts the skill uses in the transcript. The CLI then adds the skill entries to the spec list, after the sources that the agent wrote. It uploads each copy as an archive. When the copy did not change since an earlier push, it reuses the redacted archive.

```json
"spec": [
  { "kind": "ticket", "role": "task", "title": "ENG-1234: Add an export button",
    "uri": "https://tracker.example.com/issue/ENG-1234",
    "digest": "sha256:e4c2...", "captured_at": "2026-10-07T10:12:03Z" },
  { "kind": "skill", "title": "asd-ste100",
    "digest": "sha256:ab12...", "captured_at": "2026-10-07T10:14:51Z",
    "metadata": { "source": "user", "invocation_count": 3,
                  "by_model": 2, "by_user": 1, "in_subagents": 1 } }
]
```

```text
asd-ste100.tar.gz
├── SKILL.md
├── references/writing-rules.md
├── examples/before-after.md
└── scripts/ste-lint.py
```

**Claude Code.** A model start is a `Skill` tool call. Its input holds the skill name. A user start is a user message that holds a `<command-name>` marker, followed by a meta message that starts with `Base directory for this skill:`. That meta message tells a skill command apart from a built-in or custom command. It gives the skill folder. Claude Code also writes the meta message after a `Skill` tool call. The parser counts one use for each start, not for each meta message. The tool-use hooks will also match the `Skill` tool. Each hook reads the new transcript entries, finds new skill folders, and copies them. Subagent transcripts go through the same steps, and their counts merge into the session entry.

**OpenCode.** OpenCode loads skills through its native `skill` tool, with one input, `name`. A model start is a completed call to this tool. The tool result holds the skill folder, so the plugin copies the folder after the call. The OpenCode documentation does not describe a slash command that starts a skill. For OpenCode, the user count stays at zero. OpenCode finds project skills in `.opencode/skills`, `.claude/skills`, and `.agents/skills` in the repository. It finds user skills in the same folders in the user's home configuration.

**Source.** A name with a plugin prefix (`plugin:skill`) or a folder in the agent's plugin directory gives `plugin`. A folder in the repository gives `project`. A folder in the agent's managed configuration gives `organization`. A folder in the user's agent configuration gives `user`. All other cases give `unknown`.

## Decision Record

| ID | Decision | Choice | Why (and what we rejected) | Source |
|----|----------|--------|----------------------------|--------|
| D-001 | Where to record skill use | A spec entry of kind `skill` for each used skill | A skill is a source of the session, like a ticket or a plan. It gets the same storage, redaction, and annotations. Rejected: a separate `skills_used` list in the session material. | owner |
| D-002 | Who makes skill entries | The CLI, from the transcript and hook events | Skill use is a fact in the transcript, and no rule must pick it. This departs from D-001 of Spec issue-3495 for skills only. That decision rejected transcript extraction because the CLI cannot tell which input sets the task. That reason does not apply to skills. | drafting |
| D-003 | Shape of the counts | A generic `metadata` object on the spec entry, with fields that depend on the kind | Other kinds can add data later with no new top-level field. Rejected: a field named `skill`. Rejected: a separate list linked by digest. | owner |
| D-004 | Skill content | The full skill folder, not only the text that the agent loaded | A skill can run its own scripts, and a reviewer needs that code. The attestation must not depend on the `ai-agent-config` material. Rejected: the `SKILL.md` text from the transcript (no scripts, and Claude Code removes the frontmatter). | [PR comment](https://github.com/chainloop-dev/chainloop/pull/3541#discussion_r4205209188) |
| D-005 | When to copy the folder | At the first hook event after the use, with a fallback at push time | The copy matches what ran, also when the files change before the push. Rejected: read the folder at push time only. | drafting |
| D-006 | Package format | One reproducible archive for each skill | One material and one digest for each skill. The same skill version gives the same digest in all sessions. Rejected: one material for each file, which fills the attestation for a skill with many files. | drafting |
| D-007 | Package limits | Skip a package over 5 MB with a warning, no links out of the folder, no version-control folders | A large or unsafe folder must not block the push or read files outside the skill. Rejected: no size limit. | drafting |
| D-008 | Arguments of each call | Not recorded | Arguments are user text that can hold private data. The raw transcript already holds them. | [PR comment](https://github.com/chainloop-dev/chainloop/pull/3541#discussion_r4205209188) |
| D-009 | Which skills get a package | All used skills, at all levels, after secret redaction | A reviewer needs every skill that shaped the work. Rejected: project skills only. | [PR comment](https://github.com/chainloop-dev/chainloop/pull/3541#discussion_r4205209188) |
| D-010 | Limit on skill entries | A separate limit of 25 | Many skills must not push out the task and spec sources. Rejected: one shared limit. | drafting |
| D-011 | Role of a skill entry | No role | The CLI does not guess a role (D-004 of Spec issue-3531). The kind `skill` already tells the purpose. | drafting |
| D-012 | First-version providers | Claude Code and OpenCode | Both agents give the skill name and folder. Rejected: all four providers, because the Cursor and Pi formats are not known yet. | drafting |
| D-013 | Who started the skill, and subagents | Count model and user starts apart, and merge subagent uses into the session entry with their own count | A user who types a slash command uses the skill as much as a model that calls a tool. Subagent token usage merges the same way. | drafting |
| D-014 | Skill source | `project`, `user`, `organization`, `plugin`, or `unknown`, with no path | The source tells a reviewer where the skill came from. A path exposes local user data. | drafting |
| D-015 | Failed skill calls | Do not count them | A failed call loads no skill. This matches the OpenCode parser, which counts only completed tool calls. | drafting |

## Open Questions

None.

## Risks

| Risk | Mitigation |
|------|------------|
| An agent changes how it records skill use. | Golden fixtures for each format. An unknown format gives no entry and a warning, never a failed push. |
| The skill folder changes between the use and the first hook event. | The window is short. A later difference gives a warning. |
| A skill script or file holds a secret. | The secret redaction runs on each text file. Binary files go into the archive as they are, as for other spec materials. |
| A skill folder is very large. | The 5 MB limit skips the package with a warning. The entry and counts stay. |
| A consumer reads no skill entry as "no skills used" for Cursor or Pi. | Document that no entry means "not recorded" for providers without support. |
