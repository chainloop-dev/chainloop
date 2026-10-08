---
owner: migmartri
ticket: https://github.com/chainloop-dev/chainloop/issues/3561
prd:
---

# Spec issue-3561: Up-to-date spec evidence for local sources

## Summary
This spec makes sure that the spec evidence that a push records is up to date. A spec source can change during the session, for example when the agent edits it. The push must then record the current version, not the version that the agent copied earlier.

This spec changes the spec capture of [Spec issue-3495](issue-3495-session-spec-capture.md) and [Spec issue-3515](issue-3515-spec-capture-during-session.md). It does not replace them. Today the agent copies the text of each source into the session spec folder, and the push records what the agent wrote. The agent can forget to copy a source again after a change, or write a summary. Then the evidence holds an old or false record.

In this spec, the scope is sources that are local files. The push reads each local source itself, and records its content at push time. The agent writes only a header with the path. The copies of the file in the transcript become pointers to the spec material, as [Spec issue-3556](issue-3556-session-image-placeholders.md) defines. The capture instruction also tells the agent that the captures are attested evidence. Remote sources, for example a Notion page or a Linear ticket, can also change during a session. A later spec looks into them.

## Problem
- In a real session, the agent captured a repository spec with its full text. Later it edited that spec, and did not copy it again. The push recorded the old version.
- In the same session, the agent wrote a long design note outside the repository. It captured the note as a one-paragraph description that pointed to the file, and not as the text.
- The instruction already forbids a summary and requires a new copy after a change. The agent judged the copy redundant, and skipped it. The rule depends on a choice that the agent makes at each edit.
- The instruction says that the push records the files "as part of this session's evidence". It also says "There is no need to mention any of this to the user". The agent read this as low-value bookkeeping.
- Nothing checks the captures at push time. An edit can be many turns before the push.
- A full copy of a local file is a duplicate. The source file is on disk, and the transcript already holds the agent's reads and writes of it.

## Goals and Non-Goals
- Goal: when a local source changes during the session, the next push records its current version. This does not depend on the agent.
- Goal: the agent does not copy the text of a local file. It writes a header with the path only.
- Goal: the transcript holds no exact copy of the recorded content of a local source. Each exact copy becomes a pointer to the spec material.
- Goal: the instruction and the reminder tell the agent that the captures are attested evidence.
- Non-goal, for now: up-to-date evidence for remote sources, for example a Notion page, a Linear ticket, or a URL. The agent still copies their text (D-001 of Spec issue-3495). A later spec looks into them (D-008).
- Non-goal: a block or a failure of the push because of a capture. Capture problems never block the push (Spec issue-3495).
- Non-goal: a change to images. The agent copies an image file, as today.
- Non-goal: a record of each version of a local file. The push records the content at push time only, as Spec issue-3515 defines.

## Requirements

### R-001: Local source read at push
A capture file has a header and a body. The header holds the kind, the source, the role, the title, and the description. The body is the text below the header. The header of a text capture can name a local file as its source. Then the push MUST use the content of that file at push time as the body. It MUST keep the header that the agent wrote. If the agent wrote a body, the push MUST replace it. A local file is one of these:
- an absolute path,
- a path that starts with `~/`,
- a `file://` URI,
- a path relative to the repository root.

The file MUST be a regular text file.
- Done when: the agent captures a repository spec, edits the spec three times, and does not touch the capture again. The push records the spec with its last edit.

### R-002: Header only for a local file
The capture instruction MUST tell the agent to write only the header for a local file, with the path as its source. The push MUST accept a capture with a header and no body when its source is a local file.
- Done when: the agent writes a design note outside the repository and captures it as a header with its path. The push records the full note.

### R-003: Fallback when the file cannot be read
The push can fail to use the local file. For example, it cannot read the file, or the file is not text or is too large. Then the push MUST record the body that the agent wrote, as today. When there is no body, the push MUST record a warning that names the capture. It MUST NOT record the capture. The push MUST NOT fail or stop.
- Done when: the agent captures a local file and then deletes the file. The push succeeds, and the evidence holds a warning for that capture.

### R-004: Capture time
For a capture that the push reads from a local file, the capture time MUST be the modification time of that file.

### R-005: No copy of a local source in the transcript
The push MUST register the digest of the content that it read from a local file as a source of the spec material. A full read or a full write of that content in the transcript then becomes a pointer to the material. Spec issue-3556 defines this replacement (R-003 and R-011 of that spec). A partial edit, or a write of an earlier version, stays inline.
- Done when: the agent writes a design note in one full write, reads it in full, and captures it by its path. Both copies in the transcript are pointers to the spec material.

### R-006: Instruction states the stakes
The capture instruction and the reminder MUST tell the agent that the captures are attested evidence. A partial or wrong capture is a false record. They MUST tell the agent to copy the full text of a remote source. They MUST tell the agent to update a capture of a remote source in the same turn as the change. They MUST tell the agent to check these captures before a push. The instruction MUST NOT tell the agent that there is no need to mention the capture to the user.

## Constraints
- The repository is public. The instruction text is visible to all users.
- The reminder goes into the agent context at each turn. It must stay short (Spec issue-3515).
- The push runs in a git hook. It reads each local source one time, with a size limit.
- The redaction of Spec issue-3495 applies to the recorded content, as to any other capture.

## Proposal
The agent writes a capture file for each source, as today. A local source is, for example, a repository spec or a design note in a notes folder. For a local source, the agent writes only the header. The header has the kind, the role, the title, the description, and the path of the file as its source. For a ticket, a web page, pasted text, or an approved plan, the agent still copies the full text. For an image, it copies the file.

The agent writes this capture for a repository spec:

```markdown
---
kind: document
uri: docs/specs/foo.md
role: spec
title: "Spec foo"
description: The spec that this session implements.
---
```

At push time, the CLI reads the spec folder as today. For each text capture whose source is a local file, it reads that file. When the read succeeds, and the file is text and under the limit, the CLI puts the file content below the header. It keeps the header that the agent wrote, and sets the capture time to the modification time of the file. The push stores this result:

```markdown
---
kind: document
uri: docs/specs/foo.md
role: spec
title: "Spec foo"
description: The spec that this session implements.
---
# Spec foo

## Summary
(the current content of docs/specs/foo.md)
```

The stored result has the same form as a capture today. The CLI redacts it and stores it as a spec material, as today. The material and the entry in the spec list do not change. When the CLI cannot use the file, it keeps the agent's body, or records a warning when there is no body.

The push compares by digest, so an unchanged source costs little. The CLI already keeps the redacted copy of each capture under its digest for the session. When a local source did not change since the last push, the result has the same digest. The CLI then uses the stored copy and does not scan it again. The only new work at each push is one read and one digest of each local source.

The CLI then knows two digests for the material of a local source. One is the digest of the capture as the CLI recorded it. The other is the digest of the file content. It gives both to the transcript replacement of Spec issue-3556. A full read of the file, and a full write of its final content, become pointers to the material. The spec content is then in the evidence one time, as the material.

The instruction leads with the stakes. The captures are attested evidence, like a commit. A partial or wrong capture is a false record. The instruction no longer says that there is no need to mention the capture to the user. It says that a local file needs a header with its path only. For a remote source, it asks for the full text, an update in the same turn as a change, and a check before a push. The reminder repeats these rules in a few lines.

```mermaid
sequenceDiagram
    participant Agent as Coding agent
    participant Note as Local file
    participant Folder as Session spec folder
    participant CLI as Trace push

    Agent->>Note: Write or edit the spec
    Agent->>Folder: Header only, source = path of the file
    Agent->>Note: More edits (capture not touched)
    CLI->>Folder: Read captures
    CLI->>Note: Read current content
    alt Readable text file under the limit
        CLI->>CLI: Body = file content, time = file modification time
    else Not readable
        CLI->>CLI: Keep agent body, or warn when there is none
    end
    CLI->>CLI: Redact and store spec material
    CLI->>CLI: Replace copies of the file content in the transcript with pointers
```

## Decision Record

| ID | Decision | Choice | Why (and what we rejected) | Source |
|----|----------|--------|----------------------------|--------|
| D-001 | How the push treats a local source | Read the file at push time and record its content | The record no longer depends on the agent. It never blocks the push. Rejected: compare the capture with the file and warn (the evidence still holds the wrong text). Rejected: refuse the push on a difference (Spec issue-3495 never blocks a push for a capture). | drafting |
| D-002 | Body of a local-file capture | Header with the path only | The push reads the file, so a copy from the agent is a duplicate that the push discards. The duplicate was the reason the agent skipped the copy. Rejected: always copy the full text (a copy the push throws away, and one more copy in the transcript). | drafting |
| D-003 | Forms of a local path | Absolute path, `~/` path, `file://` URI, and path relative to the repository root | A design note in a notes folder is outside the repository. Rejected: files inside the repository only (misses the case of the incident). | drafting |
| D-004 | Evidence signal when the push replaces the agent's body | No warning. The capture time is the modification time of the file | The record is correct, so a warning adds noise. The file time tells when the content last changed. Rejected: a warning for each replaced body. | drafting |
| D-005 | Copies of a local source in the transcript | Register the digest of the file content as a source for the pointers of Spec issue-3556 | The content is in the spec material, so a full copy in the transcript is a duplicate. The match stays exact, so no other content leaves the transcript. Rejected: keep the copies inline (the evidence holds the spec two or three times). | owner request |
| D-006 | Size limit for a local file | 1 MiB. Over the limit, the agent's body stays | A spec is text and small. A large file in the evidence is more likely an error. | drafting |
| D-007 | Instruction wording | State the stakes, drop "no need to mention", same-turn update and check before push for remote sources | The agent read the captures as bookkeeping. These rules still apply to remote sources, which the push cannot read. | drafting |
| D-008 | Scope | Local files only. Remote sources, for example Notion pages and Linear tickets, in a later spec | The push can read a local file with no credentials and no connector. A remote source needs a fetch, and Spec issue-3495 keeps the CLI free of connectors (D-001 of that spec). The problem is the same for remote sources, so a later spec looks into them. | owner request |
| D-009 | When the push does the work again | Compare by digest at push time, and reuse the stored redacted copy when the digest is the same | The push already keeps a redacted copy for each digest. Rejected: a hook that copies each changed source into the spec folder at each turn. It computes a digest at each turn, and the push must read the files anyway. | owner request |

## Open Questions
None.

## Milestones
1. **Local files.** The push records the current content of each local source, and the instruction states the stakes.
2. **Remote sources.** A later spec looks into up-to-date evidence for sources such as Notion pages and Linear tickets.

## Risks

| Risk | Mitigation |
|------|------------|
| The agent sets a local path to a file that is not a spec, for example a file with credentials. | The redaction of Spec issue-3495 runs on the recorded content. The agent could already copy any file into the folder, so the risk is not new. Only text files under the limit are read. |
| The file changes after the last AI-assisted commit and before the push. The evidence then holds content that the session did not produce. | The push records the content at push time, as Spec issue-3515 defines for a changed spec. The capture time shows the file time. |
| The file is deleted before the push, and the capture has only a header. | The push records a warning (R-003). The transcript still holds the agent's writes of the file. |
| The agent still writes a summary for a remote source. | The instruction states the stakes and asks for a check before a push. A remote source is the case that a hook cannot check. |
