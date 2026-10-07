---
owner: jiparis
ticket: https://github.com/chainloop-dev/chainloop/issues/3540
prd:
---

# Spec 006: Skills used in a Chainloop Trace session

## Summary

Chainloop Trace will record the skills that an agent used in a session, with the text of each skill. Each used skill becomes a spec entry of the new kind `skill`. The skill text that the agent loaded goes into the attestation as an EVIDENCE material, the same as other spec sources. Spec entries get a new optional `metadata` object. Its fields depend on the kind. For a skill, it holds the skill source and the use counts.

This spec changes the spec capture of [Spec 002](002-session-spec-capture.md) and [Spec 004](004-spec-source-role.md). It does not replace them. It departs from D-001 of Spec 002 for skills only. The CLI makes the skill entries from the transcript. The agent does not write them. The first version supports Claude Code and OpenCode.

## Problem

- The evidence shows which skills are available to an agent, but not which skills the agent used. The `ai-agent-config` material lists the skill files in the repository. That list does not show use.
- The tool usage summary counts calls per tool name. All Claude Code skill calls go into one `Skill` entry, so the summary does not show which skills ran.
- The skill names and texts are in the raw transcript only. To answer "did this session use skill X?", a policy or the UI must scan every transcript entry. It must also know the transcript format of each agent.
- A skill sets how the agent works, as a spec sets what the agent builds. A reviewer cannot see the skill text next to the other sources of the session.
- A user can start a skill with a slash command (for example `/graphify`). That skill use is not a tool call, so a scan of tool calls alone does not find it.

## Goals and Non-Goals

- Goal: A policy or the UI can find the skills that a session used in the spec entries of the session material.
- Goal: The attestation holds the skill text that the agent loaded, as a material that a reviewer can download and verify.
- Goal: The record separates skills that the model started from skills that the user started, and includes skills that subagents used.
- Goal: The `metadata` object can hold kind-specific data for other kinds later, with no new top-level field.
- Non-goal: Store the `SKILL.md` file byte for byte. The material holds the text that the agent loaded.
- Non-goal: Read skill files from disk. All skill data comes from the transcript.
- Non-goal: Support Cursor and Pi in the first version. Their transcript formats for skill use are not known yet. A later change can add them without a schema change.
- Non-goal: Record the local path of a skill file.
- Non-goal: Change how the agent captures other spec sources, the tool usage summary, the raw transcript, or the `ai-agent-config` material.

## Requirements

### R-001: One spec entry for each used skill

The session material MUST hold one spec entry of kind `skill` for each skill that the session used. The title MUST be the skill name. The CLI MUST NOT set a role, a description, or a source address for a skill entry. The capture time MUST be the time of the first use in the transcript. A session that used no skill MUST have no skill entries.

- Done when: sessions with no skill, one skill, and several skills each produce the expected spec entries. Two runs over the same transcript produce the same output.

### R-002: Skill text material

Each skill entry MUST point by digest to an EVIDENCE material that holds the skill text that the agent loaded. The CLI MUST take the text from the transcript. It MUST remove the arguments of one call, so that two calls with different arguments give the same text. It MUST use the text of the first use. When a later use gives a different text, the CLI MUST record a warning. The text MUST go through the secret redaction of Spec 002 (R-007) before upload. The material MUST carry the same annotations as other spec materials, with `chainloop.spec.kind` set to `skill` and `chainloop.spec.title` set to the skill name.

- Done when: the attestation holds one EVIDENCE material for each used skill, and its digest downloads the redacted skill text.

### R-003: Metadata object

A spec entry MAY hold a `metadata` object. The fields of the object depend on the kind of the entry. This spec defines the fields for the kind `skill` only. Entries of other kinds MUST leave the object out.

### R-004: Use counts and who started the skill

The metadata of a skill entry MUST hold the total number of uses. It MUST also hold the number of uses that the model started and the number that the user started. The two start counts MUST add up to the total.

- A model start is a skill tool call that loaded a skill. A failed call MUST NOT count.
- A user start is a slash command that loads a skill. A slash command that does not load a skill MUST NOT count. Examples are `/clear`, `/model`, and custom commands that are not skills.
- One skill use MUST count one time only, also when the transcript records it in more than one entry.
- Done when: fixtures with a model start, a user start, both starts for one skill, a failed call, and built-in commands produce correct counts.

### R-005: Subagent skills

Skills that subagents used MUST count in the totals of the same entry. The metadata MUST also hold the number of uses that came from subagents.

- Done when: the main agent and a subagent use the same skill in one session. The evidence holds one entry with the correct total and subagent count.

### R-006: Skill source

The metadata of a skill entry MUST hold a source. The values are `project` (a skill from the repository), `user` (a skill from the user's agent configuration), `plugin` (a skill from an installed plugin), and `unknown`. The source MUST come from data in the transcript. Examples are a plugin prefix in the skill name and the location of the skill file. The evidence MUST NOT hold the skill file path. When the data is not sufficient, the source MUST be `unknown`.

- Done when: fixtures for each source value, plus a transcript with no location data, produce the expected source.

### R-007: Limit on skill entries

Skill entries MUST have their own limit of 25 for each session. They MUST NOT count against the limit of the spec sources that the agent writes. When a session used more skills than the limit, the CLI MUST keep the skills that the session used first and record a warning.

### R-008: Provider support

Claude Code and OpenCode MUST produce skill entries. Cursor and Pi MUST produce none until a later change adds support. For those providers, no skill entry MUST mean "not recorded" and MUST NOT mean "no skills used".

### R-009: Compatibility and failure safety

The new kind and the `metadata` object MUST be additive. The schema version MUST stay the same, and existing evidence MUST stay valid. A consumer that does not know the kind `skill` MUST still read the other spec entries. When the parser cannot read a skill use, it MUST skip it and add a warning to the evidence. A bad skill use MUST NOT stop evidence creation or block the Git push.

- Done when: old evidence and new evidence both pass schema validation. A malformed skill use produces a warning and the rest of the evidence.

## Constraints

- All skill data comes from the session transcript that Trace already collects. The change adds no new hook, network call, or file read.
- The repository is public. The evidence format is visible to all users.
- User and plugin skills can hold private text. The secret redaction applies to all skill text before upload.
- Output ordering must be deterministic, so the same transcript gives the same evidence digest.

## Proposal

Each provider parser already reads the full transcript to count tools, tokens, and messages. The parser will also collect skill uses in the same pass. At push time, the CLI adds the skill entries to the spec list, after the sources that the agent wrote. It uploads each skill text the same way as a spec file. When the text did not change since an earlier push, it reuses the redacted copy.

```json
"spec": [
  { "kind": "ticket", "role": "task", "title": "ENG-1234: Add an export button",
    "uri": "https://tracker.example.com/issue/ENG-1234",
    "digest": "sha256:e4c2...", "captured_at": "2026-10-07T10:12:03Z" },
  { "kind": "skill", "title": "superpowers:brainstorming",
    "digest": "sha256:ab12...", "captured_at": "2026-10-07T10:14:51Z",
    "metadata": { "source": "plugin", "invocation_count": 3,
                  "by_model": 2, "by_user": 1, "in_subagents": 1 } }
]
```

```json
{
  "name": "superpowers-brainstorming.md",
  "annotations": {
    "chainloop.material.name": "spec-fd4e67-skill-superpowers-brainstorming",
    "chainloop.material.type": "EVIDENCE",
    "chainloop.spec.session_id": "fd4e6754-3b26-4f54-9807-13c58465bb35",
    "chainloop.spec.kind": "skill",
    "chainloop.spec.title": "superpowers:brainstorming"
  }
}
```

**Claude Code.** A model start is a `Skill` tool call. Its input holds the skill name. A user start is a user message that holds a `<command-name>` marker, followed by a meta message that starts with `Base directory for this skill:`. The second message tells a skill command apart from a built-in or custom command. Claude Code also writes that meta message after a `Skill` tool call. The parser counts one use for each start, not for each meta message. The meta message holds the skill text and the skill location. Claude Code removes the frontmatter of the skill file and adds the call arguments at the end. The parser removes the arguments. Subagent transcripts go through the same steps, and their counts merge into the session entry.

**OpenCode.** OpenCode loads skills through its native `skill` tool, with one input, `name`. A model start is a completed call to this tool. The OpenCode documentation does not describe a slash command that starts a skill. For OpenCode, the user count stays at zero. The tool result holds the skill text and the skill directory. OpenCode finds project skills in `.opencode/skills`, `.claude/skills`, and `.agents/skills` in the repository. It finds user skills in the same folders in the user's home configuration.

**Source.** A name with a plugin prefix (`plugin:skill`) or a location in the agent's plugin directory gives `plugin`. A location in the repository gives `project`. A location in the user's agent configuration directory gives `user`. All other cases give `unknown`.

## Decision Record

| ID | Decision | Choice | Why (and what we rejected) | Source |
|----|----------|--------|----------------------------|--------|
| D-001 | Where to record skill use | A spec entry of kind `skill` for each used skill | A skill is a source of the session, like a ticket or a plan. Its text gets the same storage, redaction, and annotations. Rejected: a separate `skills_used` list in the session material, which keeps the counts but not the text. | owner |
| D-002 | Who makes skill entries | The CLI, from the transcript | Skill use is a fact in the transcript, and no rule must pick it. This departs from D-001 of Spec 002 for skills only. That decision rejected transcript extraction because the CLI cannot tell which input sets the task. That reason does not apply to skills. | drafting |
| D-003 | Shape of the counts | A generic `metadata` object on the spec entry, with fields that depend on the kind | Other kinds can add data later with no new top-level field. Rejected: a field named `skill` (one field for each kind). Rejected: a separate `skills_used` list linked by digest (two places for one skill). | owner |
| D-004 | Skill text | The text that the agent loaded, taken from the transcript, without the call arguments | It is what the model received, and it needs no file read. It works for user and plugin skills. Rejected: the `SKILL.md` file on disk (it can change after the session, and the CLI would read files outside the repository). Rejected: the disk file with a fallback to the transcript (two origins for one field). | drafting |
| D-005 | Which skills get a material | All used skills, after secret redaction | A reviewer needs the text of every skill that shaped the work. Rejected: project skills only, and project and plugin skills only (both leave out skills that the session used). | drafting |
| D-006 | Limit on skill entries | A separate limit of 25 | Many skills must not push out the task and spec sources. Rejected: one shared limit. | drafting |
| D-007 | Role of a skill entry | No role | The CLI does not guess a role (D-004 of Spec 004). The kind `skill` already tells the purpose. | drafting |
| D-008 | First-version providers | Claude Code and OpenCode | Both transcripts hold the skill name and text. Rejected: all four providers, because the Cursor and Pi formats are not known yet. | drafting |
| D-009 | User-started skills | Count them, and record who started each use | A user who types a slash command uses the skill as much as a model that calls a tool. Rejected: tool calls only. Rejected: one count with no split. | drafting |
| D-010 | Subagent skills | Merge into the session entry and add a subagent count | This matches how the evidence merges subagent token usage. Rejected: a list on each subagent. Rejected: main agent only. | drafting |
| D-011 | Skill source | `project`, `user`, `plugin`, or `unknown`, with no path | The source tells a reviewer if the skill came from the repository. A path exposes local user data. | drafting |
| D-012 | Failed skill calls | Do not count them | A failed call loads no skill. This matches the OpenCode parser, which counts only completed tool calls. | drafting |

## Open Questions

None.

## Risks

| Risk | Mitigation |
|------|------------|
| An agent changes how it records skill use or skill text. | Golden fixtures for each format. An unknown format gives no entry and a warning, never a failed push. |
| A user-started skill and its meta message count two times. | Count starts, not meta messages. A fixture covers both start types for the same skill. |
| A skill text holds a secret or private data. | The secret redaction of Spec 002 runs on each skill text before upload. |
| A consumer reads no skill entry as "no skills used" for Cursor or Pi. | Document that no entry means "not recorded" for providers without support. |
| A consumer compares the material digest with the digest of the `SKILL.md` file. | Document that the material holds the loaded text, not the file. |
