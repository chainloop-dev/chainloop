---
owner: jiparis
ticket: https://github.com/chainloop-dev/chainloop/issues/3540
prd:
---

# Spec 006: Skills used in a Chainloop Trace session

## Summary

Chainloop Trace will record the skills that an agent used in a session. The AI coding session evidence gets a new optional list, `skills_used`. Each entry names one skill and gives the number of uses. It also tells who started the skill (the model or the user) and where the skill came from (project, user, or plugin). It also gives the number of uses that came from subagents. The first version supports Claude Code and OpenCode. The change adds a field to the existing evidence schema and changes no existing field.

## Problem

- The evidence shows which skills are available to an agent, but not which skills the agent used. The `ai-agent-config` material lists the skill files in the repository. That list does not show use.
- The tool usage summary counts calls per tool name. All Claude Code skill calls go into one `Skill` entry, so the summary does not show which skills ran.
- The skill names are in the raw transcript only. To answer "did this session use skill X?", a policy or the UI must scan every transcript entry. It must also know the transcript format of each agent.
- A user can start a skill with a slash command (for example `/graphify`). That skill use is not a tool call, so a scan of tool calls alone does not find it.

## Goals and Non-Goals

- Goal: A policy or the UI can find the skills that a session used with one lookup in the session evidence.
- Goal: The record separates skills that the model started from skills that the user started.
- Goal: The record includes skills that subagents used.
- Non-goal: Record which version of a skill ran (for example a digest of its `SKILL.md`). The transcript does not hold the skill file content. The `ai-agent-config` material already records digests of repository skills.
- Non-goal: Support Cursor and Pi in the first version. Their transcript formats for skill use are not confirmed yet. A later change can add them without a schema change.
- Non-goal: Record the local path of a skill file in the new field.
- Non-goal: Change the existing tool usage summary, the raw transcript, or the `ai-agent-config` material.

## Requirements

### R-001: Skills used list

The AI coding session evidence MUST include a `skills_used` list when the session used one or more skills. Each skill MUST appear one time only in the list. The list MUST be sorted by name. A session that used no skill MUST leave the field out.

- Done when: sessions with no skill, one skill, and several skills each produce the expected evidence. Two runs over the same transcript produce the same output.

### R-002: Counts and who started the skill

Each entry MUST hold the skill name and the total number of uses. It MUST also hold the number of uses that the model started and the number that the user started. The two start counts MUST add up to the total. A model start is a skill tool call that loaded a skill. A failed call MUST NOT count. A user start is a slash command that loads a skill. One skill use MUST count one time only, also when the transcript records it in more than one entry.

- Done when: fixtures with a model-started skill, a user-started skill, and the same skill started both ways produce correct counts.

### R-003: User-started skills only

A slash command that does not load a skill MUST NOT go into the list. Examples are built-in agent commands such as `/clear` or `/model`, and custom commands that are not skills.

- Done when: a transcript with built-in commands, custom commands, and skill commands lists only the skill commands.

### R-004: Subagent skills

Skills that subagents used MUST count in the session total of the same entry. Each entry MUST also hold the number of uses that came from subagents.

- Done when: the main agent and a subagent use the same skill in one session. The evidence holds one entry with the correct total and subagent count.

### R-005: Skill source

Each entry MUST hold a source. The values are `project` (a skill from the repository), `user` (a skill from the user's agent configuration), `plugin` (a skill from an installed plugin), and `unknown`. The source MUST come from data in the transcript. Examples are a plugin prefix in the skill name and the location of the skill file. The evidence MUST NOT hold the skill file path in this field. When the data is not sufficient, the source MUST be `unknown`.

- Done when: fixtures for each source value, plus a transcript with no location data, produce the expected source.

### R-006: Provider support

Claude Code and OpenCode MUST fill the list. Cursor and Pi MUST leave the field out until a later change adds support. A missing field MUST mean "not recorded" and MUST NOT mean "no skills used" for those providers.

- Done when: Claude Code and OpenCode fixtures produce the list, and Cursor evidence does not change.

### R-007: Compatibility and failure safety

The new field MUST be optional and additive. The schema version MUST stay the same, and existing evidence MUST stay valid. The tool usage summary MUST continue to count skill tool calls as it does today. When the parser cannot read an entry, it MUST skip the entry and add a warning to the evidence. A bad entry MUST NOT stop evidence creation or block the Git push.

- Done when: old evidence and new evidence both pass schema validation. A malformed skill entry produces a warning and the rest of the evidence.

## Constraints

- The skill data comes only from the session transcript that Trace already collects. The change adds no new hook, network call, or file read outside the transcript.
- The evidence must not hold local file paths in the new field. The raw transcript keeps its current content.
- Output ordering must be deterministic, so the same transcript gives the same evidence digest.

## Proposal

Each provider parser already reads the full transcript to count tools, tokens, and messages. The parser will also collect skill uses in the same pass and add them to the evidence as `skills_used`.

```yaml
skills_used:
  - name: superpowers:brainstorming
    source: plugin
    invocation_count: 3
    by_model: 2
    by_user: 1
    in_subagents: 1
```

**Claude Code.** A model start is a `Skill` tool call. Its input holds the skill name. A user start is a user message that holds a `<command-name>` marker, followed by a meta message that starts with `Base directory for this skill:`. The second message tells a skill command apart from a built-in or custom command. Claude Code also writes that meta message after a `Skill` tool call. The parser counts one use for each start, not for each meta message. The meta message gives the skill location, which sets the source. Subagent transcripts go through the same steps, and their counts merge into the session entry.

**OpenCode.** OpenCode loads skills through its native `skill` tool, with one input, `name`. A model start is a completed call to this tool. The OpenCode documentation does not describe a slash command that starts a skill. For OpenCode, the user count stays at zero. The tool result holds the skill directory, which sets the source. OpenCode finds project skills in `.opencode/skills`, `.claude/skills`, and `.agents/skills` in the repository. It finds user skills in the same folders in the user's home configuration.

**Source.** A name with a plugin prefix (`plugin:skill`) or a location in the agent's plugin directory gives `plugin`. A location in the repository gives `project`. A location in the user's agent configuration directory gives `user`. All other cases give `unknown`.

## Decision Record

| ID | Decision | Choice | Why (and what we rejected) | Source |
|----|----------|--------|----------------------------|--------|
| D-001 | Where to record skill use | A new optional `skills_used` list in the AI coding session evidence | One lookup answers "was skill X used". Rejected: a per-skill breakdown inside the tool usage summary, which mixes two concepts. Rejected: no change, which leaves every consumer to scan the raw transcript. | drafting |
| D-002 | First-version providers | Claude Code and OpenCode | Both transcripts hold the skill name in a tool input. Rejected: all four providers, because the Cursor and Pi formats are not confirmed. Rejected: Claude Code only, because OpenCode support costs little. | drafting |
| D-003 | User-started skills | Count them, and record who started each use | A user who types a slash command uses the skill as much as a model that calls a tool. Rejected: tool calls only, which misses user starts. Rejected: one count with no split, which hides who chose the skill. | drafting |
| D-004 | Entry shape | One entry per skill, with total, model, user, and subagent counts | A policy finds a skill with one lookup. Rejected: one entry per combination of skill, starter, and agent, which lists one skill several times. | drafting |
| D-005 | Subagent skills | Merge into the session entry and add a subagent count | This matches how the evidence merges subagent token usage. Rejected: a skills list on each subagent, which adds detail no consumer asked for. Rejected: main agent only, which misses skills. | drafting |
| D-006 | Skill version | Out of scope | The transcript does not hold the skill file content. The `ai-agent-config` material records digests of repository skills. | drafting |
| D-007 | Skill source | `project`, `user`, `plugin`, or `unknown`, with no path | The source tells a reviewer if the skill came from the repository. A path exposes local user data. | drafting |
| D-008 | Failed skill calls | Do not count them | A failed call (for example, an unknown skill name) loads no skill. This matches the OpenCode parser, which counts only completed tool calls. Rejected: count every call. | drafting |

## Open Questions

None.

## Risks

| Risk | Mitigation |
|------|------------|
| An agent changes how it records skill use. | Golden fixtures for each format. An unknown format gives no entry and a warning, never a failed push. |
| A user-started skill and its meta message count two times. | Count starts, not meta messages. A fixture covers both start types for the same skill. |
| A consumer reads a missing field as "no skills used" for Cursor or Pi. | Document that a missing field means "not recorded" for providers without support. |
