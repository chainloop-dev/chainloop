# Chainloop Trace: agent support

Chainloop Trace records the sessions of AI coding agents as `CHAINLOOP_AI_CODING_SESSION` evidence. Each agent is a provider in this package: `claude/`, `cursor/` and `opencode/`. The interfaces are in `provider.go`, and the registry is in `providers/`. One provider covers OpenCode 1.x and OpenCode 2: it detects the version at runtime.

This page lists the features of Trace, by area, and tells which agents support each one. **Keep it current.** When you add or change a provider, or add or change a feature of Trace, update this page in the same pull request.

In the tables, **Yes** means supported, **Partial** means supported with limits, **No** means not supported, and **Unknown** means not verified. Below each table, "Why not Yes" tells the reason for each cell that is not **Yes**.

Claude Code has the most complete support.

## Session collection

What the session evidence holds about the conversation.

| Feature | What it means | Claude Code | Cursor | OpenCode 1.x | OpenCode 2 |
|---------|---------------|-------------|--------|--------------|------------|
| Transcript | The evidence holds the conversation (`raw_session`): the messages and the tool calls of the agent. | Yes | Yes | Partial | Partial |
| Token usage and cost | The evidence holds the tokens that the session used and their cost in USD. | Yes | No | Yes | Yes |
| Model detection | The evidence names the primary model and the other models that the session used. | Yes | Partial | Yes | Yes |
| Subagents | The evidence holds the conversations of the subagents, and their token usage, as part of the session. | Yes | No | No | No |

Why not Yes:

- **Transcript, OpenCode:** OpenCode has no transcript file. The provider runs `opencode export` (1.x) or `opencode session export` (2) and rebuilds the transcript from the export. It keeps the text and the inputs of the tool calls. It drops the tool outputs, the reasoning and the file parts. OpenCode 2 exports also have no agent version and no session slug.
- **Token usage and cost, Cursor:** the Cursor transcript does not hold token usage. Usage and cost are zero, and the evidence has a warning that tells so. (For OpenCode, the cost is the value that OpenCode reports. For Claude Code, Chainloop computes it from its price table.)
- **Model detection, Cursor:** the primary model comes from the session-start hook. The transcript does not tell the models, so the evidence does not list the other models.
- **Subagents, Cursor:** the provider does not read subagent conversations.
- **Subagents, OpenCode:** a subagent runs in a child session with its own export. It is not part of the evidence of its parent session.

## Code attribution

Which lines of a commit the AI wrote.

| Feature | What it means | Claude Code | Cursor | OpenCode 1.x | OpenCode 2 |
|---------|---------------|-------------|--------|--------------|------------|
| Line attribution | Each changed line of a commit is attributed to the AI or to a human, from a snapshot of each file before and after the agent edits it. | Yes | Partial | Yes | Yes |
| Shell changes attribution | Changes that the agent makes with shell commands (for example `sed`) are attributed to the AI too, not only changes made with the file tools. | Yes | No | Yes | Yes |

Why not Yes:

- **Line attribution, Cursor:** Cursor has only the `afterFileEdit` hook, which gives the edits but no snapshot before the edit. The provider rebuilds the content before the edit from the edits.
- **Shell changes attribution, Cursor:** Cursor has no hook before and after a shell command.

## User experience

What the user sees in the agent.

| Feature | What it means | Claude Code | Cursor | OpenCode 1.x | OpenCode 2 |
|---------|---------------|-------------|--------|--------------|------------|
| Welcome message | At session start, the user sees a message that tells that Chainloop records the session, and in which project. | Yes | No | No | No |
| Session link after push | After a `git push` that the agent runs, the user sees a link to the session in Chainloop. | Yes | No | No | No |

Why not Yes:

- **Welcome message, Cursor and OpenCode:** the provider gives the session-start context to the model only. It has no channel that shows a message to the user at session start, so the welcome message is dropped.
- **Session link after push, Cursor:** Cursor has no hook after a shell command, so there is no point at which to show the link.
- **Session link after push, OpenCode:** the plugin hook fires after a shell command, but the response that shows a message to the user is not verified yet. The link stays pending until it expires.

## Specs and skills

What the session was asked to build, and which skills it used.

| Feature | What it means | Claude Code | Cursor | OpenCode 1.x | OpenCode 2 |
|---------|---------------|-------------|--------|--------------|------------|
| Spec capture instruction | At session start, the agent is told to capture the specs of the session (tickets, documents, plans, images) in the spec folder. A plan is captured only when it comes from an external source or from the plan mode of the agent: an agreement in the chat is not a plan. A pasted image is not captured, also when the agent has a file path for it. The agent tells the user in one line when it captures a file. | Yes | Yes | Yes | Yes |
| Spec capture reminder | At each user prompt, the agent is reminded to capture new or changed specs. | Yes | No | Yes | Yes |
| Local spec sources read at push | A capture whose source is a local file holds only a header and a placeholder. Each push reads the file and records its current content, so the evidence follows the edits of the file. Only a regular text file of up to 1 MiB is read. Otherwise the push keeps the body that the agent wrote, or drops the capture with a warning when there is no body (spec issue-3561). | Yes | Yes | Yes | Yes |
| Skills tracking | The evidence lists the skills that the session used, with a copy of each skill as it ran. | Yes | No | Partial | Partial |
| Spec source pointers | Exact copies of spec sources in the transcript (full file reads and file writes) are replaced with a pointer to the spec material, so the evidence does not hold the same content two or three times (spec issue-3556). | Yes | No | No | No |
| Pasted image pointers | Each distinct image that the user pasted into the session, including images in prompts sent while the agent was busy, is stored once as an image spec entry, from the data in the transcript, with the title "Pasted image N" and no role. A repeated paste of the same image adds no entry. The inline image in the transcript is replaced with a pointer to that material. An image whose data cannot be decoded, or that the push could not store, stays inline, and the push continues. When the agent captured the same image, its entry is kept and no second entry is added (spec issue-3569). | Yes | No | No | No |

Why not Yes:

- **Spec capture reminder, Cursor:** Cursor has no hook at each user prompt. The agent gets the session-start instruction only.
- **Skills tracking, Cursor:** the provider does not track skills. The evidence has no skill entries, which means "not recorded", not "no skill used".
- **Skills tracking, OpenCode:** only the skills that the model starts with the `skill` tool are counted. Skills that the user starts, and skills used in subagents, are not.
- **Spec source pointers, Cursor and OpenCode:** the provider has no finder for the copies in its transcript yet. For OpenCode, the rebuilt transcript holds no tool outputs, so only the write copies would apply.
- **Pasted image pointers, Cursor and OpenCode:** the provider has no finder for pasted images yet. Each agent puts pasted images in a different block shape. The pasted images stay inline in the transcript. The agent is told not to capture them in the spec folder.

## Security

How the evidence is protected before it leaves the machine.

| Feature | What it means | Claude Code | Cursor | OpenCode 1.x | OpenCode 2 |
|---------|---------------|-------------|--------|--------------|------------|
| Secret redaction | Secrets in the session evidence and in the spec materials are replaced with a placeholder before upload. | Yes | Yes | Yes | Yes |
| Media skipped by redaction | The secret redaction does not scan base64 media (images, PDFs) in the transcript, so it is faster and does not break the media. | Yes | Unknown | Not applicable | Not applicable |

Why not Yes:

- **Media skipped by redaction, Cursor:** the redaction skips base64 media blocks of the shape `{"type":"base64","data":...}`. The Cursor transcript is stored as it is, and it is not verified whether it holds media in that shape.
- **Media skipped by redaction, OpenCode:** the rebuilt transcript holds no media.

## Git integration

How the sessions are linked to the commits and attested.

| Feature | What it means | Claude Code | Cursor | OpenCode 1.x | OpenCode 2 |
|---------|---------------|-------------|--------|--------------|------------|
| Commit trailer | Each commit gets a `Chainloop-Trace-Sessions` trailer with the sessions that contributed to it. | Yes | Yes | Yes | Yes |
| Attestation at push | The `pre-push` hook attests the sessions of the pushed commits. | Yes | Yes | Yes | Yes |
| Evidence export to disk | `chainloop trace run --export <dir>` runs the same assembly a push does — the same redaction, pointers, and spec, skill, and image materials — and writes the result to a local directory, each material under its content digest, with no control plane and no attestation. The exported evidence equals what a push would upload, apart from the signature and the attestation wrapper. `--no-redact` keeps secrets, for a trusted local run. | Yes | Yes | Yes | Yes |

## How each agent is hooked in

| Agent | Hook installation | Hooks |
|-------|-------------------|-------|
| Claude Code | `.claude/settings.json` | Session start and end, user prompt, and before and after each file and shell tool and the Skill tool. |
| Cursor | `.cursor/hooks.json` | Session start and end, and `afterFileEdit`. There is no shell hook and no prompt hook. |
| OpenCode | `.opencode/plugins/chainloop-trace.ts` | Session start and end, user prompt, and before and after each tool. One plugin covers both versions: `server()` for 1.x (1.3.4 or later) and `setup()` for 2. The provider picks the export command from `opencode --version`. |

All agents share the git hooks `commit-msg`, `post-commit` and `pre-push`.
